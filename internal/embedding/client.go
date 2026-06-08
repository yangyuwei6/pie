package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"pie/internal/config"
)

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	httpClient *http.Client
}

func NewClient(cfg config.AIConfig) *Client {
	return &Client{
		baseURL:    strings.TrimRight(cfg.EmbeddingBaseURL, "/"),
		apiKey:     cfg.EmbeddingAPIKey,
		model:      cfg.EmbeddingModel,
		dimensions: cfg.EmbeddingDimensions,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Model() string {
	return c.model
}

func (c *Client) CreateEmbedding(ctx context.Context, text string) ([]float64, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("embedding base url is empty")
	}

	reqBody := embeddingRequest{
		Model:      c.model,
		Input:      []string{text},
		Dimensions: c.dimensions,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embedding api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding api returned status %s", resp.Status)
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding api returned empty vector")
	}

	return result.Data[0].Embedding, nil
}

type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
