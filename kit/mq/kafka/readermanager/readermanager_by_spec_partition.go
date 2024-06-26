package readermanager

import (
	"context"

	"github.com/segmentio/kafka-go"
	"github.com/superj80820/system-design/kit/mq"
)

type specPartitionReaderManager struct {
	*readerManager

	reader *Reader
}

type specPartitionReaderManagerOption func(*specPartitionReaderManager)

func CreateSpecPartitionReaderManager(ctx context.Context, topic string, startOffset int64, partition int, brokers []string, options ...ReaderManagerConfigOption) (mq.ReaderManager, error) {
	config := new(readerManagerConfig)
	for _, option := range options {
		option(config)
	}

	AddKafkaReaderHookFn(func(kafkaReader KafkaReader) {
		kafkaReader.SetOffset(startOffset)
	})(config)

	kafkaReader := createReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		MaxBytes:  10e6, // 10MB
		Partition: partition,
	}, config.readerOptions...)
	rm := &specPartitionReaderManager{
		readerManager: createReaderManager(config.readerManagerOptions...),
		reader:        kafkaReader,
	}

	for _, option := range config.specPartitionReaderManagers {
		option(rm)
	}

	rm.reader.Run()

	return rm, nil
}

func (s *specPartitionReaderManager) AddObserver(observer mq.Observer) bool {
	return s.reader.AddObserver(observer)
}

func (s *specPartitionReaderManager) AddObserverBatch(observer mq.Observer) bool {
	return s.reader.AddObserverBatch(observer)
}

func (s *specPartitionReaderManager) GetObserversLen() int {
	return s.reader.GetObserversLen()
}

func (s *specPartitionReaderManager) IfNoObserversThenStopConsume() {
	if s.reader.GetObserversLen() == 0 {
		s.reader.StopConsume()
	}
}

func (s *specPartitionReaderManager) RemoveObserverWithHook(observer mq.Observer) bool {
	if ok := s.reader.RemoveObserver(observer); !ok {
		return false
	}
	go observer.UnSubscribeHook()
	return true
}

func (s *specPartitionReaderManager) StartConsume(ctx context.Context) bool {
	return s.reader.StartConsume(ctx)
}

func (s *specPartitionReaderManager) StopConsume() bool {
	return s.reader.StopConsume()
}

func (s *specPartitionReaderManager) SyncStartConsume(ctx context.Context) bool {
	return s.reader.SyncStartConsume(ctx)
}

func (s *specPartitionReaderManager) Wait() {
	// TODO
}
