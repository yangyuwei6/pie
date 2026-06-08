package data

import (
	"context"
	"fmt"
	"strconv"

	"pie/internal/data/model"

	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
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

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
