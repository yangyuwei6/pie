package repo

import (
	"context"

	"pie/internal/data/model"
)

type DocumentVectorRepo interface {
	DeleteByFileMD5(ctx context.Context, fileMD5 string) error
	BatchCreate(ctx context.Context, vectors []*model.DocumentVector) error
	FindByFileMD5(ctx context.Context, fileMD5 string) ([]*model.DocumentVector, error)
}
