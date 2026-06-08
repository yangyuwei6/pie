package data

import (
	"context"
	"fmt"
	"io"
	"time"

	"pie/internal/data/model"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type UploadRepo struct {
	data   *Data
	bucket string
}

func NewUploadRepo(data *Data, bucket string) *UploadRepo {
	return &UploadRepo{data: data, bucket: bucket}
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
	_, err := r.data.minioClient.PutObject(ctx, r.bucket, objectName, reader, size, minio.PutObjectOptions{})
	return err
}

func (r *UploadRepo) GetObject(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return r.data.minioClient.GetObject(ctx, r.bucket, objectName, minio.GetObjectOptions{})
}

func (r *UploadRepo) MergeChunks(ctx context.Context, sourceObjects []string, destObject string) error {
	if len(sourceObjects) == 0 {
		return fmt.Errorf("source objects is empty")
	}

	dst := minio.CopyDestOptions{
		Bucket: r.bucket,
		Object: destObject,
	}

	if len(sourceObjects) == 1 {
		src := minio.CopySrcOptions{
			Bucket: r.bucket,
			Object: sourceObjects[0],
		}
		_, err := r.data.minioClient.CopyObject(ctx, dst, src)
		return err
	}

	srcs := make([]minio.CopySrcOptions, 0, len(sourceObjects))
	for _, objectName := range sourceObjects {
		srcs = append(srcs, minio.CopySrcOptions{
			Bucket: r.bucket,
			Object: objectName,
		})
	}

	_, err := r.data.minioClient.ComposeObject(ctx, dst, srcs...)
	return err
}

func (r *UploadRepo) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	u, err := r.data.minioClient.PresignedGetObject(ctx, r.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (r *UploadRepo) UpdateFileUploadStatus(ctx context.Context, recordID int64, status int32) error {
	f := r.data.q.FileUpload
	_, err := f.WithContext(ctx).Where(f.ID.Eq(recordID)).Update(f.Status, status)
	return err
}

func (r *UploadRepo) DeleteUploadMark(ctx context.Context, fileMD5 string, userID string) error {
	key := uploadKey(userID, fileMD5)
	return r.data.rdb.Del(ctx, key).Err()
}

func (r *UploadRepo) FindBatchByMD5s(ctx context.Context, fileMD5s []string) ([]*model.FileUpload, error) {
	if len(fileMD5s) == 0 {
		return []*model.FileUpload{}, nil
	}

	f := r.data.q.FileUpload
	return f.WithContext(ctx).Where(f.FileMd5.In(fileMD5s...)).Find()
}

func uploadKey(userID string, fileMD5 string) string {
	return fmt.Sprintf("upload:%s:%s", userID, fileMD5)
}
