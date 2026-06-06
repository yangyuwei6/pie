package service

import (
	"context"
	"errors"
	"math"
	"strconv"

	"pie/internal/repo"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultChunkSize = 5 * 1024 * 1024

type UploadService struct {
	uploadRepo repo.UploadRepo
	logger     *zap.Logger
}

type CheckFileResult struct {
	Completed      bool  `json:"completed"`
	UploadedChunks []int `json:"uploadedChunks"`
}

func NewUploadService(uploadRepo repo.UploadRepo, logger *zap.Logger) *UploadService {
	return &UploadService{
		uploadRepo: uploadRepo,
		logger:     logger,
	}
}

func (s *UploadService) CheckFile(ctx context.Context, fileMD5 string, userID int64) (*CheckFileResult, error) {
	userIDString := strconv.FormatInt(userID, 10)
	record, err := s.uploadRepo.GetFileUploadRecord(ctx, fileMD5, userIDString)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &CheckFileResult{
				Completed:      false,
				UploadedChunks: []int{},
			}, nil
		}
		return nil, err
	}

	if record.Status == 1 {
		return &CheckFileResult{
			Completed:      true,
			UploadedChunks: []int{},
		}, nil
	}

	totalChunks := calculateTotalChunks(record.TotalSize)
	uploadedChunks, err := s.uploadRepo.GetUploadedChunks(ctx, fileMD5, userIDString, totalChunks)
	if err != nil {
		return nil, err
	}

	return &CheckFileResult{
		Completed:      false,
		UploadedChunks: uploadedChunks,
	}, nil
}

func calculateTotalChunks(totalSize int64) int {
	if totalSize <= 0 {
		return 0
	}
	return int(math.Ceil(float64(totalSize) / float64(defaultChunkSize)))
}
