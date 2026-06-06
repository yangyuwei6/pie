package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"pie/internal/config"
	"pie/internal/messaging/message"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(cfg config.MessagingConfig) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.KafkaBrokers...),
			Topic:    cfg.Topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) ProduceFileTask(ctx context.Context, msg message.FileProcessing) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal file processing message: %w", err)
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Value: data,
	})
}
