package mq

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	readerManager "github.com/superj80820/system-design/kit/mq/reader_manager"
	writerManager "github.com/superj80820/system-design/kit/mq/writer_manager"
)

type Message interface {
	GetKey() string
	Marshal() ([]byte, error)
}

type MQTopicOption func(*MQTopicConfig)

type MQTopicConfig struct {
	url     string
	topic   string
	brokers []string

	writerBalancer writerManager.WriterBalancer

	readerWay         readerManager.ReaderWay
	readerGroupID     string
	readerPartition   int
	readerStartOffset int64
}

func ProduceWay(balancer writerManager.WriterBalancer) MQTopicOption {
	return func(m *MQTopicConfig) {
		m.writerBalancer = balancer
	}
}

func ConsumeByGroupID(groupID string, startOffset int64) MQTopicOption {
	return func(m *MQTopicConfig) {
		m.readerWay = readerManager.GroupIDReader
		m.readerGroupID = groupID
		m.readerStartOffset = startOffset
	}
}

func ConsumeBySpecPartition(partition int, startOffset int64) MQTopicOption {
	return func(m *MQTopicConfig) {
		m.readerWay = readerManager.SpecPartitionReader
		m.readerPartition = partition
		m.readerStartOffset = startOffset
	}
}

func ConsumeByPartitionsBindObserver(startOffset int64) MQTopicOption {
	return func(m *MQTopicConfig) {
		m.readerWay = readerManager.PartitionsBindObserverReader
		m.readerStartOffset = startOffset
	}
}

type MQTopic interface {
	Subscribe(key string, notify readerManager.Notify, options ...readerManager.ObserverOption) *readerManager.Observer
	UnSubscribe(observer *readerManager.Observer)
	Produce(ctx context.Context, message Message) error
	Done() <-chan struct{}
	Err() error
	Shutdown() bool
}

type mqTopic struct {
	readerManager readerManager.ReaderManager
	writerManager writerManager.WriterManager

	topicPartitionInfo []kafka.Partition

	lock   sync.RWMutex
	cancel context.CancelFunc
	doneCh chan struct{}
	errCh  chan error
	err    error
}

func CreateMQTopic(ctx context.Context, url, topic string, consumeWay MQTopicOption, options ...MQTopicOption) (MQTopic, error) {
	ctx, cancel := context.WithCancel(ctx)

	mqConfig := &MQTopicConfig{
		topic:   topic,
		url:     url,
		brokers: strings.Split(url, ","),

		writerBalancer: &kafka.RoundRobin{},
	}

	consumeWay(mqConfig)

	for _, option := range options {
		option(mqConfig)
	}

	writer := writerManager.CreateWriterManager(
		mqConfig.brokers,
		mqConfig.topic,
		mqConfig.writerBalancer,
	)

	var (
		reader readerManager.ReaderManager
		err    error
	)
	errCh := make(chan error)
	doneCh := make(chan struct{})
	readerErrorHandlerFn := func(err error) {
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}
	if err := func() error {
		defer cancel()

		switch mqConfig.readerWay {
		case readerManager.GroupIDReader:
			reader, err = readerManager.CreateGroupIDReaderManager(
				ctx,
				mqConfig.brokers,
				mqConfig.topic,
				mqConfig.readerGroupID,
				mqConfig.readerStartOffset,
				readerManager.AddErrorHandleFn(readerErrorHandlerFn),
			)
			if err != nil {
				return errors.Wrap(err, "create reader manager failed")
			}
		case readerManager.SpecPartitionReader:
			reader, err = readerManager.CreateSpecPartitionReaderManager(
				ctx,
				mqConfig.topic,
				mqConfig.readerStartOffset,
				mqConfig.readerPartition,
				mqConfig.brokers,
				readerManager.AddErrorHandleFn(readerErrorHandlerFn),
			)
			if err != nil {
				return errors.Wrap(err, "create reader manager failed")
			}
		case readerManager.PartitionsBindObserverReader:
			reader, err = readerManager.CreatePartitionBindObserverReaderManager(
				ctx,
				mqConfig.url,
				mqConfig.readerStartOffset,
				mqConfig.brokers,
				mqConfig.topic,
				readerManager.AddErrorHandleFn(readerErrorHandlerFn),
			)
			if err != nil {
				return errors.Wrap(err, "create reader manager failed")
			}
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	mq := &mqTopic{
		writerManager: writer,
		readerManager: reader,
		cancel:        cancel,
		doneCh:        doneCh,
		errCh:         errCh,
	}

	go func() {
		err := <-errCh
		mq.err = err
		cancel()
		close(doneCh)
	}()

	return mq, nil
}

func (m *mqTopic) Subscribe(key string, notify readerManager.Notify, options ...readerManager.ObserverOption) *readerManager.Observer {
	observer := readerManager.CreateObserver(key, notify, options...)

	m.readerManager.AddObserver(observer)
	m.readerManager.StartConsume(context.Background())

	return observer
}

func (m *mqTopic) UnSubscribe(observer *readerManager.Observer) {
	m.readerManager.RemoveObserverWithHook(observer)
	m.readerManager.IfNoObserversThenStopConsume()
}

func (m *mqTopic) Produce(ctx context.Context, message Message) error {
	marshalMessage, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal message failed")
	}

	kafkaMsg := kafka.Message{
		Key:   []byte(message.GetKey()),
		Value: marshalMessage,
	}

	if err := m.writerManager.WriteMessages(ctx, kafkaMsg); err != nil {
		if ctx.Err() != nil { // expected. context done
			return nil
		}
		return errors.Wrap(err, "write messages to kafka failed")
	}

	return nil
}

func (m *mqTopic) Shutdown() bool {
	m.cancel()

	done := make(chan bool)
	go func() {
		m.readerManager.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(10 * time.Second):
		return false
	}
}

func (m *mqTopic) Done() <-chan struct{} {
	return m.doneCh
}

func (m *mqTopic) Err() error {
	return m.err
}
