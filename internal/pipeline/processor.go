package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"pie/internal/data/model"
	"pie/internal/messaging/message"
	"pie/internal/repo"

	"go.uber.org/zap"
)

const (
	defaultTextChunkSize    = 1000
	defaultTextChunkOverlap = 100
	defaultModelVersion     = "text-embedding-v4"
)

type TextExtractor interface {
	ExtractText(reader io.Reader, fileName string) (string, error)
}

type EmbeddingClient interface {
	CreateEmbedding(ctx context.Context, text string) ([]float64, error)
	Model() string
}

type SearchIndexer interface {
	IndexDocument(ctx context.Context, doc SearchDocument) error
}

type SearchDocument struct {
	VectorID     string    `json:"vector_id"`
	FileMD5      string    `json:"file_md5"`
	ChunkID      int32     `json:"chunk_id"`
	TextContent  string    `json:"text_content"`
	Vector       []float64 `json:"vector"`
	ModelVersion string    `json:"model_version"`
	UserID       int64     `json:"user_id"`
	OrgTag       string    `json:"org_tag"`
	IsPublic     bool      `json:"is_public"`
}

type Processor struct {
	uploadRepo    repo.UploadRepo
	vectorRepo    repo.DocumentVectorRepo
	extractor     TextExtractor
	embedding     EmbeddingClient
	searchIndexer SearchIndexer
	logger        *zap.Logger
}

func NewProcessor(
	uploadRepo repo.UploadRepo,
	vectorRepo repo.DocumentVectorRepo,
	extractor TextExtractor,
	embedding EmbeddingClient,
	searchIndexer SearchIndexer,
	logger *zap.Logger,
) *Processor {
	return &Processor{
		uploadRepo:    uploadRepo,
		vectorRepo:    vectorRepo,
		extractor:     extractor,
		embedding:     embedding,
		searchIndexer: searchIndexer,
		logger:        logger,
	}
}

func (p *Processor) Process(ctx context.Context, task message.FileProcessing) error {
	p.logger.Info("process file started",
		zap.String("file_md5", task.FileMD5),
		zap.String("file_name", task.FileName),
		zap.Int64("user_id", task.UserID),
	)

	if task.FileMD5 == "" || task.FileName == "" {
		return errors.New("file processing task missing file metadata")
	}

	objectName := fmt.Sprintf("merged/%s", task.FileName)
	object, err := p.uploadRepo.GetObject(ctx, objectName)
	if err != nil {
		p.logger.Error("process file failed: get merged object failed",
			zap.String("object_name", objectName),
			zap.Error(err),
		)
		return fmt.Errorf("get merged object: %w", err)
	}
	defer object.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(object); err != nil {
		p.logger.Error("process file failed: read object failed",
			zap.String("object_name", objectName),
			zap.Error(err),
		)
		return fmt.Errorf("read merged object: %w", err)
	}
	if buf.Len() == 0 {
		return errors.New("merged file is empty")
	}

	textContent, err := p.extractor.ExtractText(bytes.NewReader(buf.Bytes()), task.FileName)
	if err != nil {
		p.logger.Error("process file failed: extract text failed",
			zap.String("file_name", task.FileName),
			zap.Error(err),
		)
		return fmt.Errorf("extract text: %w", err)
	}
	if textContent == "" {
		return errors.New("extracted text is empty")
	}

	chunks := splitText(textContent, defaultTextChunkSize, defaultTextChunkOverlap)
	if len(chunks) == 0 {
		return errors.New("no text chunks generated")
	}

	if err := p.vectorRepo.DeleteByFileMD5(ctx, task.FileMD5); err != nil {
		p.logger.Error("process file failed: delete old vectors failed",
			zap.String("file_md5", task.FileMD5),
			zap.Error(err),
		)
		return fmt.Errorf("delete old document vectors: %w", err)
	}

	modelVersion := p.embeddingModel()
	vectors := make([]*model.DocumentVector, 0, len(chunks))
	userID := strconv.FormatInt(task.UserID, 10)
	for i, chunk := range chunks {
		text := chunk
		vectors = append(vectors, &model.DocumentVector{
			FileMd5:      task.FileMD5,
			ChunkID:      int32(i),
			TextContent:  &text,
			ModelVersion: &modelVersion,
			UserID:       userID,
			OrgTag:       stringPtrOrNil(task.OrgTag),
			IsPublic:     task.IsPublic,
		})
	}

	if err := p.vectorRepo.BatchCreate(ctx, vectors); err != nil {
		p.logger.Error("process file failed: batch create vectors failed",
			zap.String("file_md5", task.FileMD5),
			zap.Error(err),
		)
		return fmt.Errorf("batch create document vectors: %w", err)
	}

	savedVectors, err := p.vectorRepo.FindByFileMD5(ctx, task.FileMD5)
	if err != nil {
		p.logger.Error("process file failed: find saved vectors failed",
			zap.String("file_md5", task.FileMD5),
			zap.Error(err),
		)
		return fmt.Errorf("find document vectors: %w", err)
	}

	for _, docVector := range savedVectors {
		text := stringValue(docVector.TextContent)
		vector, err := p.embedding.CreateEmbedding(ctx, text)
		if err != nil {
			p.logger.Error("process file failed: create embedding failed",
				zap.String("file_md5", task.FileMD5),
				zap.Int32("chunk_id", docVector.ChunkID),
				zap.Error(err),
			)
			return fmt.Errorf("create embedding for chunk %d: %w", docVector.ChunkID, err)
		}

		searchDoc := SearchDocument{
			VectorID:     fmt.Sprintf("%s_%d", docVector.FileMd5, docVector.ChunkID),
			FileMD5:      docVector.FileMd5,
			ChunkID:      docVector.ChunkID,
			TextContent:  text,
			Vector:       vector,
			ModelVersion: stringValue(docVector.ModelVersion),
			UserID:       task.UserID,
			OrgTag:       stringValue(docVector.OrgTag),
			IsPublic:     docVector.IsPublic,
		}

		if err := p.searchIndexer.IndexDocument(ctx, searchDoc); err != nil {
			p.logger.Error("process file failed: index search document failed",
				zap.String("file_md5", task.FileMD5),
				zap.Int32("chunk_id", docVector.ChunkID),
				zap.Error(err),
			)
			return fmt.Errorf("index search document for chunk %d: %w", docVector.ChunkID, err)
		}
	}

	p.logger.Info("process file finished",
		zap.String("file_md5", task.FileMD5),
		zap.String("file_name", task.FileName),
		zap.Int("chunks", len(savedVectors)),
	)
	return nil
}

func (p *Processor) embeddingModel() string {
	if p.embedding == nil {
		return defaultModelVersion
	}
	model := p.embedding.Model()
	if model == "" {
		return defaultModelVersion
	}
	return model
}

func splitText(text string, chunkSize int, chunkOverlap int) []string {
	if chunkSize <= 0 {
		return nil
	}
	if chunkSize <= chunkOverlap {
		return simpleSplit(text, chunkSize)
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	step := chunkSize - chunkOverlap
	chunks := make([]string, 0)
	for i := 0; i < len(runes); i += step {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func simpleSplit(text string, chunkSize int) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	chunks := make([]string, 0)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
