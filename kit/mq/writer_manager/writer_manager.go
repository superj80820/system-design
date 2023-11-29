package writermanager

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type (
	WriterBalancer = kafka.Balancer

	Hash       = kafka.Hash
	RoundRobin = kafka.RoundRobin
)

type WriterManager interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type writer struct {
	*kafka.Writer
}

func CreateWriterManager(brokers []string, topic string, writerBalancer WriterBalancer) WriterManager {
	return &writer{
		kafka.NewWriter(kafka.WriterConfig{
			Brokers:  brokers,
			Topic:    topic,
			Balancer: writerBalancer,
			Dialer: &kafka.Dialer{
				Timeout:   10 * time.Second,
				DualStack: true,
			},
		}),
	}
}
