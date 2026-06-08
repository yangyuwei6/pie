package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"pie/internal/data/model"
	"pie/internal/messaging/message"
	"pie/internal/repo"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultChunkSize = 5 * 1024 * 1024

type UploadService struct {
	uploadRepo repo.UploadRepo
	userRepo   repo.UserRepo
	producer   FileTaskProducer
	logger     *zap.Logger
}

type FileTaskProducer interface {
	ProduceFileTask(ctx context.Context, msg message.FileProcessing) error
}

type CheckFileResult struct {
	Completed      bool  `json:"completed"`
	UploadedChunks []int `json:"uploadedChunks"`
}

type UploadChunkResult struct {
	Uploaded []int   `json:"uploaded"`
	Progress float64 `json:"progress"`
}

type MergeChunksResult struct {
	ObjectURL string `json:"objectUrl"`
}

type UploadStatusResult struct {
	FileName    string  `json:"fileName"`
	FileType    string  `json:"fileType"`
	Uploaded    []int   `json:"uploaded"`
	Progress    float64 `json:"progress"`
	TotalChunks int     `json:"totalChunks"`
}

type SupportedFileTypesResult struct {
	SupportedExtensions []string `json:"supportedExtensions"`
	SupportedTypes      []string `json:"supportedTypes"`
	Description         string   `json:"description"`
}

func NewUploadService(uploadRepo repo.UploadRepo, userRepo repo.UserRepo, producer FileTaskProducer, logger *zap.Logger) *UploadService {
	return &UploadService{
		uploadRepo: uploadRepo,
		userRepo:   userRepo,
		producer:   producer,
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

func (s *UploadService) MergeChunks(ctx context.Context, fileMD5, fileName string, userID int64) (*MergeChunksResult, error) {
	userIDString := strconv.FormatInt(userID, 10)

	record, err := s.uploadRepo.GetFileUploadRecord(ctx, fileMD5, userIDString)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("merge chunks failed: upload record not found",
				zap.String("file_md5", fileMD5),
				zap.Int64("user_id", userID),
			)
			return nil, ErrUploadNotFound
		}
		s.logger.Error("merge chunks failed: get file upload record failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	totalChunks := calculateTotalChunks(record.TotalSize)
	uploadedChunks, err := s.uploadRepo.GetUploadedChunks(ctx, fileMD5, userIDString, totalChunks)
	if err != nil {
		s.logger.Error("merge chunks failed: get uploaded chunks failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	if len(uploadedChunks) < totalChunks {
		s.logger.Warn("merge chunks failed: upload chunks not completed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int("uploaded_chunks", len(uploadedChunks)),
			zap.Int("total_chunks", totalChunks),
		)
		return nil, ErrUploadNotCompleted
	}

	sourceObjects := make([]string, 0, totalChunks)
	for i := 0; i < totalChunks; i++ {
		sourceObjects = append(sourceObjects, fmt.Sprintf("chunks/%s/%d", fileMD5, i))
	}
	destObject := fmt.Sprintf("merged/%s", fileName)
	if err := s.uploadRepo.MergeChunks(ctx, sourceObjects, destObject); err != nil {
		s.logger.Error("merge chunks failed: merge objects failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.String("dest_object", destObject),
			zap.Error(err),
		)
		return nil, err
	}

	if err := s.uploadRepo.UpdateFileUploadStatus(ctx, record.ID, 1); err != nil {
		s.logger.Error("merge chunks failed: update upload status failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Int64("record_id", record.ID),
			zap.Error(err),
		)
		return nil, err
	}

	objectURL, err := s.uploadRepo.GetPresignedURL(ctx, destObject, time.Hour)
	if err != nil {
		s.logger.Error("merge chunks failed: get presigned url failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.String("dest_object", destObject),
			zap.Error(err),
		)
		return nil, err
	}

	if s.producer != nil {
		if err := s.producer.ProduceFileTask(ctx, message.FileProcessing{
			FileMD5:   fileMD5,
			ObjectURL: objectURL,
			FileName:  fileName,
			UserID:    userID,
			OrgTag:    stringValue(record.OrgTag),
			IsPublic:  record.IsPublic,
		}); err != nil {
			s.logger.Error("merge chunks failed: produce file processing task failed",
				zap.String("file_md5", fileMD5),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			return nil, err
		}
	}

	if err := s.uploadRepo.DeleteUploadMark(ctx, fileMD5, userIDString); err != nil {
		s.logger.Error("merge chunks failed: delete upload mark failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	s.logger.Info("merge chunks success",
		zap.String("file_md5", fileMD5),
		zap.Int64("user_id", userID),
		zap.String("dest_object", destObject),
	)

	return &MergeChunksResult{
		ObjectURL: objectURL,
	}, nil
}

func (s *UploadService) GetUploadStatus(ctx context.Context, fileMD5 string, userID int64) (*UploadStatusResult, error) {
	userIDString := strconv.FormatInt(userID, 10)

	record, err := s.uploadRepo.GetFileUploadRecord(ctx, fileMD5, userIDString)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("get upload status failed: upload record not found",
				zap.String("file_md5", fileMD5),
				zap.Int64("user_id", userID),
			)
			return nil, ErrUploadNotFound
		}
		s.logger.Error("get upload status failed: get file upload record failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	totalChunks := calculateTotalChunks(record.TotalSize)
	uploadedChunks, err := s.uploadRepo.GetUploadedChunks(ctx, fileMD5, userIDString, totalChunks)
	if err != nil {
		s.logger.Error("get upload status failed: get uploaded chunks failed",
			zap.String("file_md5", fileMD5),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return &UploadStatusResult{
		FileName:    record.FileName,
		FileType:    getFileType(record.FileName),
		Uploaded:    uploadedChunks,
		Progress:    calculateProgress(uploadedChunks, totalChunks),
		TotalChunks: totalChunks,
	}, nil
}

func (s *UploadService) GetSupportedFileTypes() *SupportedFileTypesResult {
	typeMapping := map[string]string{
		".doc":  "Word document",
		".docx": "Word document",
		".md":   "Markdown document",
		".pdf":  "PDF document",
		".ppt":  "PowerPoint presentation",
		".pptx": "PowerPoint presentation",
		".txt":  "Text file",
		".xls":  "Excel spreadsheet",
		".xlsx": "Excel spreadsheet",
	}

	extensions := make([]string, 0, len(typeMapping))
	typeSet := make(map[string]struct{})
	for ext, fileType := range typeMapping {
		extensions = append(extensions, ext)
		typeSet[fileType] = struct{}{}
	}
	sort.Strings(extensions)

	fileTypes := make([]string, 0, len(typeSet))
	for fileType := range typeSet {
		fileTypes = append(fileTypes, fileType)
	}
	sort.Strings(fileTypes)

	return &SupportedFileTypesResult{
		SupportedExtensions: extensions,
		SupportedTypes:      fileTypes,
		Description:         "Supported document types can be parsed and indexed for retrieval.",
	}
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

func getFileType(fileName string) string {
	ext := fileExtension(fileName)
	switch ext {
	case ".pdf":
		return "PDF document"
	case ".doc", ".docx":
		return "Word document"
	case ".xls", ".xlsx":
		return "Excel spreadsheet"
	case ".ppt", ".pptx":
		return "PowerPoint presentation"
	case ".txt":
		return "Text file"
	case ".md":
		return "Markdown document"
	case "":
		return "Unknown file"
	default:
		return strings.ToUpper(strings.TrimPrefix(ext, ".")) + " file"
	}
}

func fileExtension(fileName string) string {
	index := strings.LastIndex(fileName, ".")
	if index < 0 || index == len(fileName)-1 {
		return ""
	}
	return strings.ToLower(fileName[index:])
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
