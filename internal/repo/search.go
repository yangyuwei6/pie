package repo

import (
	"context"

	"pie/internal/data/model"
)

type SearchRepo interface {
	EnsureIndex(ctx context.Context) error
	IndexDocument(ctx context.Context, doc *model.DocumentVector, vector []float64) error
}
