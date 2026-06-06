package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"pie/internal/data/model"
	"pie/internal/repo"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultChunkSize = 5 * 1024 * 1024

type UploadService struct {
	uploadRepo repo.UploadRepo
	userRepo   repo.UserRepo
	logger     *zap.Logger
}

type CheckFileResult struct {
	Completed      bool  `json:"completed"`
	UploadedChunks []int `json:"uploadedChunks"`
}

type UploadChunkResult struct {
	Uploaded []int   `json:"uploaded"`
	Progress float64 `json:"progress"`
}

func NewUploadService(uploadRepo repo.UploadRepo, userRepo repo.UserRepo, logger *zap.Logger) *UploadService {
	return &UploadService{
		uploadRepo: uploadRepo,
		userRepo:   userRepo,
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

func (s *UploadService) UploadChunk(ctx context.Context, fileMD5, fileName string, totalSize int64, chunkIndex int, file io.Reader, fileSize int64, userID int64, orgTag string, isPublic bool) (*UploadChunkResult, error) {
	userIDString := strconv.FormatInt(userID, 10)

	if chunkIndex < 0 {
		s.logger.Warn("upload chunk failed: invalid chunk index",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("chunk_index", chunkIndex),
		)
		return nil, ErrInvalidUploadChunk
	}

	if chunkIndex == 0 && !isSupportedFileType(fileName) {
		s.logger.Warn("upload chunk failed: unsupported file type",
			zap.String("file_name", fileName),
			zap.Int64("user_id", userID),
		)
		return nil, ErrUnsupportedFileType
	}

	record, err := s.uploadRepo.GetFileUploadRecord(ctx, fileMD5, userIDString)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record, err = s.createFileUploadRecord(ctx, fileMD5, fileName, totalSize, userID, userIDString, orgTag, isPublic)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		s.logger.Error("upload chunk failed: get file upload record failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	uploaded, err := s.uploadRepo.IsChunkUploaded(ctx, fileMD5, userIDString, chunkIndex)
	if err != nil {
		s.logger.Error("upload chunk failed: check chunk status failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("chunk_index", chunkIndex),
			zap.Error(err),
		)
		return nil, err
	}

	totalChunks := calculateTotalChunks(record.TotalSize)
	if uploaded {
		uploadedChunks, err := s.uploadRepo.GetUploadedChunks(ctx, fileMD5, userIDString, totalChunks)
		if err != nil {
			s.logger.Error("upload chunk failed: get uploaded chunks failed",
				zap.String("file_md5", fileMD5),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
		return &UploadChunkResult{
			Uploaded: uploadedChunks,
			Progress: calculateProgress(uploadedChunks, totalChunks),
		}, nil
	}

	objectName := fmt.Sprintf("chunks/%s/%d", fileMD5, chunkIndex)
	if err := s.uploadRepo.SaveChunk(ctx, objectName, file, fileSize); err != nil {
		s.logger.Error("upload chunk failed: save chunk failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("chunk_index", chunkIndex),
			zap.String("object_name", objectName),
			zap.Error(err),
		)
		return nil, err
	}

	chunkRecord := &model.ChunkInfo{
		FileMd5:     fileMD5,
		ChunkIndex:  int32(chunkIndex),
		ChunkMd5:    "",
		StoragePath: objectName,
	}
	if err := s.uploadRepo.CreateChunkInfoRecord(ctx, chunkRecord); err != nil {
		s.logger.Error("upload chunk failed: create chunk info failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("chunk_index", chunkIndex),
			zap.Error(err),
		)
		return nil, err
	}

	if err := s.uploadRepo.MarkChunkUploaded(ctx, fileMD5, userIDString, chunkIndex); err != nil {
		s.logger.Error("upload chunk failed: mark chunk uploaded failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("chunk_index", chunkIndex),
			zap.Error(err),
		)
		return nil, err
	}

	uploadedChunks, err := s.uploadRepo.GetUploadedChunks(ctx, fileMD5, userIDString, totalChunks)
	if err != nil {
		s.logger.Error("upload chunk failed: get uploaded chunks failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	s.logger.Info("upload chunk success",
		zap.String("file_md5", fileMD5),
		zap.Int64("user_id", userID),
		zap.Int("chunk_index", chunkIndex),
	)

	return &UploadChunkResult{
		Uploaded: uploadedChunks,
		Progress: calculateProgress(uploadedChunks, totalChunks),
	}, nil
}

func (s *UploadService) createFileUploadRecord(ctx context.Context, fileMD5, fileName string, totalSize int64, userID int64, userIDString string, orgTag string, isPublic bool) (*model.FileUpload, error) {
	if orgTag == "" {
		user, err := s.userRepo.FindByID(ctx, userID)
		if err != nil {
			s.logger.Error("upload chunk failed: get user primary org failed",
				zap.String("file_md5", fileMD5),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
		orgTag = stringValue(user.PrimaryOrg)
	}

	record := &model.FileUpload{
		FileMd5:   fileMD5,
		FileName:  fileName,
		TotalSize: totalSize,
		Status:    0,
		UserID:    userIDString,
		OrgTag:    stringPtrOrNil(orgTag),
		IsPublic:  isPublic,
	}

	if err := s.uploadRepo.CreateFileUploadRecord(ctx, record); err != nil {
		s.logger.Error("upload chunk failed: create file upload record failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return record, nil
}

func calculateTotalChunks(totalSize int64) int {
	if totalSize <= 0 {
		return 0
	}
	return int(math.Ceil(float64(totalSize) / float64(defaultChunkSize)))
}

func calculateProgress(uploadedChunks []int, totalChunks int) float64 {
	if totalChunks == 0 {
		return 0
	}
	return float64(len(uploadedChunks)) / float64(totalChunks) * 100
}

func isSupportedFileType(fileName string) bool {
	ext := strings.ToLower(fileName)
	for _, suffix := range []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".md"} {
		if strings.HasSuffix(ext, suffix) {
			return true
		}
	}
	return false
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
