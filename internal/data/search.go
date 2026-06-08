package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"pie/internal/data/model"
	"pie/internal/repo"

	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
)

type SearchRepo struct {
	data      *Data
	indexName string
}

type searchDocument struct {
	VectorID     string    `json:"vector_id"`
	FileMD5      string    `json:"file_md5"`
	ChunkID      int32     `json:"chunk_id"`
	TextContent  string    `json:"text_content"`
	Vector       []float64 `json:"vector"`
	ModelVersion string    `json:"model_version"`
	UserID       int64     `json:"user_id"`
	OrgTag       string    `json:"org_tag"`
	IsPublic     bool      `json:"is_public"`
}

func NewSearchRepo(data *Data, indexName string) *SearchRepo {
	return &SearchRepo{data: data, indexName: indexName}
}

// EnsureIndex creates the knowledge index if it does not already exist.
// The mapping mirrors the old project: text for BM25, dense_vector for KNN,
// and user/org/public fields for permission filtering.
func (r *SearchRepo) EnsureIndex(ctx context.Context) error {
	exists, err := r.data.esClient.Indices.Exists(r.indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	if exists {
		return nil
	}

	req, err := create.NewRequest().FromJSON(indexMapping)
	if err != nil {
		return fmt.Errorf("build index mapping: %w", err)
	}

	_, err = r.data.esClient.Indices.Create(r.indexName).Request(req).Do(ctx)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	return nil
}

// IndexDocument stores one processed text chunk in Elasticsearch.
// VectorID is used as the document id so re-processing the same file overwrites
// the same chunk documents instead of creating duplicates.
func (r *SearchRepo) IndexDocument(ctx context.Context, docVector *model.DocumentVector, vector []float64) error {
	doc := buildSearchDocument(docVector, vector)
	_, err := r.data.esClient.Index(r.indexName).
		Id(doc.VectorID).
		Document(doc).
		Refresh(refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("index document: %w", err)
	}
	return nil
}

func (r *SearchRepo) HybridSearch(ctx context.Context, query string, queryVector []float64, topK int, userID int64, orgTags []string) ([]repo.SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}

	normalized, phrase := normalizeQuery(query)
	searchBody := buildHybridSearchBody(normalized, phrase, queryVector, topK, userID, orgTags)

	res, err := r.data.esClient.Search().
		Index(r.indexName).
		Raw(searchBody).
		TrackTotalHits(true).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("search elasticsearch: %w", err)
	}

	results, err := decodeSearchHits(res.Hits.Hits)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 || phrase == "" || phrase == query {
		return results, nil
	}

	retryBody := buildHybridSearchBody(phrase, phrase, queryVector, topK, userID, orgTags)
	retryRes, err := r.data.esClient.Search().
		Index(r.indexName).
		Raw(retryBody).
		TrackTotalHits(true).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("retry search elasticsearch: %w", err)
	}

	return decodeSearchHits(retryRes.Hits.Hits)
}

func buildSearchDocument(docVector *model.DocumentVector, vector []float64) searchDocument {
	userID, _ := strconv.ParseInt(docVector.UserID, 10, 64)
	return searchDocument{
		VectorID:     fmt.Sprintf("%s_%d", docVector.FileMd5, docVector.ChunkID),
		FileMD5:      docVector.FileMd5,
		ChunkID:      docVector.ChunkID,
		TextContent:  stringValue(docVector.TextContent),
		Vector:       vector,
		ModelVersion: stringValue(docVector.ModelVersion),
		UserID:       userID,
		OrgTag:       stringValue(docVector.OrgTag),
		IsPublic:     docVector.IsPublic,
	}
}

func buildHybridSearchBody(query string, phrase string, queryVector []float64, topK int, userID int64, orgTags []string) *strings.Reader {
	recallK := topK * 30
	if recallK < topK {
		recallK = topK
	}

	body := map[string]any{
		"knn": map[string]any{
			"field":          "vector",
			"query_vector":   queryVector,
			"k":              recallK,
			"num_candidates": recallK,
		},
		"query": map[string]any{
			"bool": map[string]any{
				"must": map[string]any{
					"match": map[string]any{
						"text_content": query,
					},
				},
				"filter": map[string]any{
					"bool": map[string]any{
						"should": []map[string]any{
							{"term": map[string]any{"user_id": userID}},
							{"term": map[string]any{"is_public": true}},
							{"terms": map[string]any{"org_tag": orgTags}},
						},
						"minimum_should_match": 1,
					},
				},
				"should": buildPhraseShould(phrase),
			},
		},
		"rescore": map[string]any{
			"window_size": recallK,
			"query": map[string]any{
				"rescore_query": map[string]any{
					"match": map[string]any{
						"text_content": map[string]any{
							"query":    query,
							"operator": "and",
						},
					},
				},
				"query_weight":         0.2,
				"rescore_query_weight": 1.0,
			},
		},
		"size": topK,
	}

	b, _ := json.Marshal(body)
	return strings.NewReader(string(b))
}

func decodeSearchHits(hits []types.Hit) ([]repo.SearchResult, error) {
	results := make([]repo.SearchResult, 0, len(hits))
	for _, hit := range hits {
		var source searchDocument
		if err := json.Unmarshal(hit.Source_, &source); err != nil {
			return nil, fmt.Errorf("decode search hit source: %w", err)
		}

		score := 0.0
		if hit.Score_ != nil {
			score = float64(*hit.Score_)
		}

		results = append(results, repo.SearchResult{
			FileMD5:     source.FileMD5,
			ChunkID:     source.ChunkID,
			TextContent: source.TextContent,
			Score:       score,
			UserID:      source.UserID,
			OrgTag:      source.OrgTag,
			IsPublic:    source.IsPublic,
		})
	}
	return results, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeQuery(q string) (string, string) {
	kept := strings.TrimSpace(strings.ToLower(q))
	if kept == "" {
		return q, ""
	}
	for _, sp := range []string{"是谁", "是什么", "是啥", "请问", "怎么", "如何", "告诉我", "严格", "按照", "不要补充", "的区别", "区别", "吗", "呢", "？", "?"} {
		kept = strings.ReplaceAll(kept, sp, " ")
	}

	kept = regexp.MustCompile(`[^\p{Han}a-z0-9\s]+`).ReplaceAllString(kept, " ")
	kept = regexp.MustCompile(`\s+`).ReplaceAllString(kept, " ")
	kept = strings.TrimSpace(kept)
	if kept == "" {
		return q, ""
	}
	return kept, kept
}

func buildPhraseShould(phrase string) any {
	if phrase == "" {
		return nil
	}
	return []map[string]any{
		{
			"match_phrase": map[string]any{
				"text_content": map[string]any{
					"query": phrase,
					"boost": 3.0,
				},
			},
		},
	}
}

const indexMapping = `{
  "mappings": {
    "properties": {
      "vector_id": { "type": "keyword" },
      "file_md5": { "type": "keyword" },
      "chunk_id": { "type": "integer" },
      "text_content": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "vector": {
        "type": "dense_vector",
        "dims": 2048,
        "index": true,
        "similarity": "cosine"
      },
      "model_version": { "type": "keyword" },
      "user_id": { "type": "long" },
      "org_tag": { "type": "keyword" },
      "is_public": { "type": "boolean" }
    }
  }
}`
