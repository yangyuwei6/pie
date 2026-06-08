package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"pie/internal/config"
	"pie/internal/messaging/message"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type FileProcessor interface {
	Process(ctx context.Context, task message.FileProcessing) error
}

type Consumer struct {
	reader    *kafka.Reader
	processor FileProcessor
	logger    *zap.Logger
}

func NewConsumer(cfg config.MessagingConfig, processor FileProcessor, logger *zap.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  cfg.KafkaBrokers,
			Topic:    cfg.Topic,
			GroupID:  "pie-file-processing-consumer",
			MinBytes: 10e3,
			MaxBytes: 10e6,
		}),
		processor: processor,
		logger:    logger,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("kafka consumer started")
	defer c.reader.Close()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch kafka message: %w", err)
		}

		var task message.FileProcessing
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			c.logger.Error("kafka consumer received invalid message",
				zap.ByteString("value", msg.Value),
				zap.Error(err),
			)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				return fmt.Errorf("commit invalid kafka message: %w", err)
			}
			continue
		}

		if err := c.processor.Process(ctx, task); err != nil {
			c.logger.Error("kafka file processing failed",
				zap.String("file_md5", task.FileMD5),
				zap.String("file_name", task.FileName),
				zap.Error(err),
			)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("commit kafka message: %w", err)
		}
	}
}
