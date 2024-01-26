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

func CreateGroupIDReaderManager(ctx context.Context, brokers []string, topic, groupID string, options ...ReaderManagerConfigOption) (ReaderManager, error) {
	config := new(readerManagerConfig)
	for _, option := range options {
		option(config)
	}

	rm := &groupIDReaderManager{
		readerManager: createReaderManager(config.readerManagerOptions...),
		reader: createReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			MaxBytes: 10e6, // 10MB
			GroupID:  groupID,
		}, config.readerOptions...),
	}

	for _, option := range config.groupIDReaderManagerOptions {
		option(rm)
	}

	rm.reader.Run()

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

func (g *groupIDReaderManager) StartConsume(ctx context.Context) bool {
	return g.reader.StartConsume(ctx)
}

func (g *groupIDReaderManager) StopConsume() bool {
	return g.reader.StopConsume()
}

func (g *groupIDReaderManager) SyncStartConsume(ctx context.Context) bool {
	return g.reader.SyncStartConsume(ctx)
}

func (g *groupIDReaderManager) Wait() {
	// TODO
}
