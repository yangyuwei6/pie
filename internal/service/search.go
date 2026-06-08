package service

import (
	"context"
	"errors"
	"strconv"

	"pie/internal/repo"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SearchEmbeddingClient interface {
	CreateEmbedding(ctx context.Context, text string) ([]float64, error)
}

type SearchService struct {
	searchRepo repo.SearchRepo
	uploadRepo repo.UploadRepo
	userRepo   repo.UserRepo
	embedding  SearchEmbeddingClient
	logger     *zap.Logger
}

type SearchResult struct {
	FileMD5     string  `json:"fileMd5"`
	FileName    string  `json:"fileName"`
	ChunkID     int32   `json:"chunkId"`
	TextContent string  `json:"textContent"`
	Score       float64 `json:"score"`
	UserID      string  `json:"userId"`
	OrgTag      string  `json:"orgTag"`
	IsPublic    bool    `json:"isPublic"`
}

func NewSearchService(searchRepo repo.SearchRepo, uploadRepo repo.UploadRepo, userRepo repo.UserRepo, embedding SearchEmbeddingClient, logger *zap.Logger) *SearchService {
	return &SearchService{
		searchRepo: searchRepo,
		uploadRepo: uploadRepo,
		userRepo:   userRepo,
		embedding:  embedding,
		logger:     logger,
	}
}

func (s *SearchService) HybridSearch(ctx context.Context, query string, topK int, userID int64) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("hybrid search failed: user not found", zap.Int64("user_id", userID))
			return nil, ErrUserNotFound
		}
		s.logger.Error("hybrid search failed: find user failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, err
	}

	queryVector, err := s.embedding.CreateEmbedding(ctx, query)
	if err != nil {
		s.logger.Error("hybrid search failed: create query embedding failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, err
	}

	orgTags := splitOrgTags(user.OrgTags)
	results, err := s.searchRepo.HybridSearch(ctx, query, queryVector, topK, user.ID, orgTags)
	if err != nil {
		s.logger.Error("hybrid search failed: search repository failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, err
	}
	if len(results) == 0 {
		s.logger.Info("hybrid search success", zap.Int64("user_id", userID), zap.Int("results", 0))
		return []SearchResult{}, nil
	}

	fileNameMap, err := s.fileNameMap(ctx, results)
	if err != nil {
		s.logger.Error("hybrid search failed: find file info failed", zap.Int64("user_id", userID), zap.Error(err))
		return nil, err
	}

	resp := make([]SearchResult, 0, len(results))
	for _, result := range results {
		fileName := fileNameMap[result.FileMD5]
		if fileName == "" {
			fileName = "unknown file"
		}
		resp = append(resp, SearchResult{
			FileMD5:     result.FileMD5,
			FileName:    fileName,
			ChunkID:     result.ChunkID,
			TextContent: result.TextContent,
			Score:       result.Score,
			UserID:      strconv.FormatInt(result.UserID, 10),
			OrgTag:      result.OrgTag,
			IsPublic:    result.IsPublic,
		})
	}

	s.logger.Info("hybrid search success", zap.Int64("user_id", userID), zap.Int("results", len(resp)))
	return resp, nil
}

func (s *SearchService) fileNameMap(ctx context.Context, results []repo.SearchResult) (map[string]string, error) {
	seen := make(map[string]struct{})
	fileMD5s := make([]string, 0, len(results))
	for _, result := range results {
		if result.FileMD5 == "" {
			continue
		}
		if _, ok := seen[result.FileMD5]; ok {
			continue
		}
		seen[result.FileMD5] = struct{}{}
		fileMD5s = append(fileMD5s, result.FileMD5)
	}

	files, err := s.uploadRepo.FindBatchByMD5s(ctx, fileMD5s)
	if err != nil {
		return nil, err
	}

	fileNameMap := make(map[string]string, len(files))
	for _, file := range files {
		fileNameMap[file.FileMd5] = file.FileName
	}
	return fileNameMap, nil
}
