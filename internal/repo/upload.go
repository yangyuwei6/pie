package repo

import (
	"context"
	"io"

	"pie/internal/data/model"
)

type UploadRepo interface {
	GetFileUploadRecord(ctx context.Context, fileMD5 string, userID string) (*model.FileUpload, error)
	CreateFileUploadRecord(ctx context.Context, record *model.FileUpload) error
	CreateChunkInfoRecord(ctx context.Context, record *model.ChunkInfo) error
	IsChunkUploaded(ctx context.Context, fileMD5 string, userID string, chunkIndex int) (bool, error)
	MarkChunkUploaded(ctx context.Context, fileMD5 string, userID string, chunkIndex int) error
	GetUploadedChunks(ctx context.Context, fileMD5 string, userID string, totalChunks int) ([]int, error)
	SaveChunk(ctx context.Context, objectName string, reader io.Reader, size int64) error
}
