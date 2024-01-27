package writermanager

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/superj80820/system-design/kit/mq"
)

type (
	WriterBalancer = kafka.Balancer

	Hash       = kafka.Hash
	RoundRobin = kafka.RoundRobin
)

type writer struct {
	kafkaWriter *kafka.Writer
}

func (w *writer) WriteMessages(ctx context.Context, msgs ...mq.Message) error {
	kafkaMessages := make([]kafka.Message, len(msgs))
	for _, msg := range msgs {
		marshalMessage, err := msg.Marshal()
		if err != nil {
			return errors.Wrap(err, "marshal message failed")
		}
		kafkaMessages = append(kafkaMessages, kafka.Message{
			Key:   []byte(msg.GetKey()),
			Value: marshalMessage,
		})
	}
	w.kafkaWriter.WriteMessages(ctx, kafkaMessages...)
	return nil
}

func CreateWriterManager(brokers []string, topic string, writerBalancer WriterBalancer) mq.WriterManager {
	return &writer{
		kafkaWriter: kafka.NewWriter(kafka.WriterConfig{
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
