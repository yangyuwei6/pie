package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"pie/internal/config"
	"pie/internal/pipeline"
)

type Indexer struct {
	baseURL    string
	indexName  string
	httpClient *http.Client
}

func NewIndexer(cfg config.SearchConfig) *Indexer {
	return &Indexer{
		baseURL:    strings.TrimRight(cfg.ElasticsearchURL, "/"),
		indexName:  cfg.IndexName,
		httpClient: http.DefaultClient,
	}
}

func (i *Indexer) EnsureIndex(ctx context.Context) error {
	if i.baseURL == "" {
		return fmt.Errorf("elasticsearch url is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, i.baseURL+"/"+i.indexName, nil)
	if err != nil {
		return fmt.Errorf("create exists index request: %w", err)
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("check index exists returned status %s", resp.Status)
	}

	body := strings.NewReader(indexMapping)
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, i.baseURL+"/"+i.indexName, body)
	if err != nil {
		return fmt.Errorf("create index request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index returned status %s: %s", resp.Status, string(data))
	}
	return nil
}

func (i *Indexer) IndexDocument(ctx context.Context, doc pipeline.SearchDocument) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal search document: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_doc/%s?refresh=true", i.baseURL, i.indexName, doc.VectorID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create index document request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("index document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index document returned status %s: %s", resp.Status, string(data))
	}
	return nil
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
