package readermanager

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type specPartitionReaderManager struct {
	*readerManager

	reader *Reader
}

type specPartitionReaderManagerOption func(*specPartitionReaderManager)

func CreateSpecPartitionReaderManager(topic string, startOffset int64, partition int, brokers []string, options ...readerManagerConfigOption) (ReaderManager, error) {
	config := new(readerManagerConfig)
	for _, option := range options {
		option(config)
	}

	rm := &specPartitionReaderManager{
		readerManager: createReaderManager(config.readerManagerOptions...),
		reader: createReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			MinBytes:    10e3, // 10KB
			MaxBytes:    10e6, // 10MB
			Partition:   partition,
			StartOffset: startOffset,
		}, config.readerOptions...),
	}

	for _, option := range config.specPartitionReaderManagers {
		option(rm)
	}

	rm.reader.Run()

	return rm, nil
}

func (s *specPartitionReaderManager) AddObserver(observer *Observer) bool {
	return s.reader.AddObserver(observer)
}

func (s *specPartitionReaderManager) GetObserversLen() int {
	return s.reader.GetObserversLen()
}

func (s *specPartitionReaderManager) IfNoObserversThenStopConsume() {
	if s.reader.GetObserversLen() == 0 {
		s.reader.StopConsume()
	}
}

func (s *specPartitionReaderManager) RemoveObserverWithHook(observer *Observer) bool {
	if ok := s.reader.RemoveObserver(observer); !ok {
		return false
	}
	go observer.unSubscribeHook()
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
