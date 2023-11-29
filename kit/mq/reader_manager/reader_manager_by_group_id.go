package readermanager

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type groupIDReaderManager struct {
	*readerManager

	reader *Reader
}

type groupIDReaderManagerOption func(*groupIDReaderManager)

func CreateGroupIDReaderManager(brokers []string, topic, groupID string, startOffset int64, options ...readerManagerConfigOption) (ReaderManager, error) {
	config := new(readerManagerConfig)
	for _, option := range options {
		option(config)
	}

	rm := &groupIDReaderManager{
		readerManager: createReaderManager(config.readerManagerOptions...),
		reader: createReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			MinBytes:    10e3, // 10KB
			MaxBytes:    10e6, // 10MB
			GroupID:     groupID,
			StartOffset: startOffset,
		}, config.readerOptions...),
	}

	for _, option := range config.groupIDReaderManagerOptions {
		option(rm)
	}

	return rm, nil
}

func (g *groupIDReaderManager) AddObserver(observer *Observer) bool {
	return g.reader.AddObserver(observer)
}

func (g *groupIDReaderManager) GetObserversLen() int {
	return g.reader.GetObserversLen()
}

func (g *groupIDReaderManager) IfNoObserversThenStopConsume() {
	if g.reader.GetObserversLen() == 0 {
		g.reader.StopConsume()
	}
}

func (g *groupIDReaderManager) RemoveObserverWithHook(observer *Observer) bool {
	if ok := g.reader.RemoveObserver(observer); !ok {
		return false
	}
	go observer.unSubscribeHook()
	return true
}

func (g *groupIDReaderManager) Run() {
	g.reader.Run()
}

func (g *groupIDReaderManager) StartConsume(ctx context.Context) bool {
	return g.reader.StartConsume(ctx)
}

func (g *groupIDReaderManager) StopConsume() bool {
	return g.reader.StopConsume()
}

func (g *groupIDReaderManager) SyncStartConsume(ctx context.Context) bool {
	return g.reader.SyncStartConsume(ctx)
}
