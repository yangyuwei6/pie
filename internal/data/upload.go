package data

import (
	"context"
	"fmt"

	"pie/internal/data/model"

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

func uploadKey(userID string, fileMD5 string) string {
	return fmt.Sprintf("upload:%s:%s", userID, fileMD5)
}
