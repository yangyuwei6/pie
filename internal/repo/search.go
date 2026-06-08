package repo

import (
	"context"

	"pie/internal/data/model"
)

type SearchRepo interface {
	EnsureIndex(ctx context.Context) error
	IndexDocument(ctx context.Context, doc *model.DocumentVector, vector []float64) error
	HybridSearch(ctx context.Context, query string, queryVector []float64, topK int, userID int64, orgTags []string) ([]SearchResult, error)
}

type SearchResult struct {
	FileMD5     string
	ChunkID     int32
	TextContent string
	Score       float64
	UserID      int64
	OrgTag      string
	IsPublic    bool
}
