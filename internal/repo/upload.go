package repo

import (
	"context"

	"pie/internal/data/model"
)

type UploadRepo interface {
	GetFileUploadRecord(ctx context.Context, fileMD5 string, userID string) (*model.FileUpload, error)
	GetUploadedChunks(ctx context.Context, fileMD5 string, userID string, totalChunks int) ([]int, error)
}
