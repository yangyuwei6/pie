package data

import (
	"context"
	"fmt"
	"io"

	"pie/internal/data/model"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type UploadRepo struct {
	data *Data
}

func NewUploadRepo(data *Data) *UploadRepo {
	return &UploadRepo{data: data}
}

func (r *UploadRepo) GetFileUploadRecord(ctx context.Context, fileMD5 string, userID string) (*model.FileUpload, error) {
	f := r.data.q.FileUpload

	return f.WithContext(ctx).Where(f.FileMd5.Eq(fileMD5), f.UserID.Eq(userID)).First()
}

func (r *UploadRepo) CreateFileUploadRecord(ctx context.Context, record *model.FileUpload) error {
	return r.data.q.FileUpload.WithContext(ctx).Create(record)
}

func (r *UploadRepo) CreateChunkInfoRecord(ctx context.Context, record *model.ChunkInfo) error {
	return r.data.q.ChunkInfo.WithContext(ctx).Create(record)
}

func (r *UploadRepo) IsChunkUploaded(ctx context.Context, fileMD5 string, userID string, chunkIndex int) (bool, error) {
	key := uploadKey(userID, fileMD5)
	value, err := r.data.rdb.GetBit(ctx, key, int64(chunkIndex)).Result()
	if err != nil {
		return false, err
	}
	return value == 1, nil
}

func (r *UploadRepo) MarkChunkUploaded(ctx context.Context, fileMD5 string, userID string, chunkIndex int) error {
	key := uploadKey(userID, fileMD5)
	return r.data.rdb.SetBit(ctx, key, int64(chunkIndex), 1).Err()
}

func (r *UploadRepo) GetUploadedChunks(ctx context.Context, fileMD5 string, userID string, totalChunks int) ([]int, error) {
	if totalChunks == 0 {
		return []int{}, nil
	}

	key := uploadKey(userID, fileMD5)
	bitmap, err := r.data.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return []int{}, nil
		}
		return nil, err
	}

	uploaded := make([]int, 0)
	for i := 0; i < totalChunks; i++ {
		byteIndex := i / 8
		bitIndex := i % 8
		if byteIndex < len(bitmap) && (bitmap[byteIndex]>>(7-bitIndex))&1 == 1 {
			uploaded = append(uploaded, i)
		}
	}

	return uploaded, nil
}

func (r *UploadRepo) SaveChunk(ctx context.Context, objectName string, reader io.Reader, size int64) error {
	_, err := r.data.minioClient.PutObject(ctx, r.data.minioBucket, objectName, reader, size, minio.PutObjectOptions{})
	return err
}

func uploadKey(userID string, fileMD5 string) string {
	return fmt.Sprintf("upload:%s:%s", userID, fileMD5)
}
