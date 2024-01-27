package kafka

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/superj80820/system-design/kit/mq"
	readerManager "github.com/superj80820/system-design/kit/mq/kafka/reader_manager"
	writerManager "github.com/superj80820/system-design/kit/mq/kafka/writer_manager"
)

type MQTopicOption func(*MQTopicConfig)

type MQTopicConfig struct {
	url     string
	topic   string
	brokers []string

	writerBalancer writerManager.WriterBalancer

	isCreateTopic                bool
	isManualCommit               bool
	createTopicNumPartitions     int
	createTopicReplicationFactor int

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

func ConsumeByGroupID(groupID string, isManualCommit bool) MQTopicOption {
	return func(m *MQTopicConfig) {
		m.readerWay = readerManager.GroupIDReader
		m.readerGroupID = groupID
		m.isManualCommit = isManualCommit
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

func CreateTopic(numPartitions, replicationFactor int) MQTopicOption {
	return func(mc *MQTopicConfig) {
		mc.isCreateTopic = true
		mc.createTopicNumPartitions = numPartitions
		mc.createTopicReplicationFactor = replicationFactor
	}
}

type mqTopic struct {
	readerManager mq.ReaderManager
	writerManager mq.WriterManager

	topicPartitionInfo []kafka.Partition

	lock   sync.RWMutex
	cancel context.CancelFunc
	doneCh chan struct{}
	errCh  chan error
	err    error
}

func CreateMQTopic(ctx context.Context, url, topic string, consumeWay MQTopicOption, options ...MQTopicOption) (mq.MQTopic, error) {
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

	if mqConfig.isCreateTopic {
		if err := createTopic(url, topic, mqConfig.createTopicNumPartitions, mqConfig.createTopicReplicationFactor); err != nil {
			return nil, errors.Wrap(err, "create topic failed")
		}
	}

	writer := writerManager.CreateWriterManager(
		mqConfig.brokers,
		mqConfig.topic,
		mqConfig.writerBalancer,
	)

	var (
		reader mq.ReaderManager
		err    error
	)
	ctx, cancel := context.WithCancel(ctx)
	errCh := make(chan error)
	doneCh := make(chan struct{})
	readerErrorHandlerFn := func(err error) {
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}
	if err := func() error {
		switch mqConfig.readerWay {
		case readerManager.GroupIDReader:
			readerManagerConfigOptions := []readerManager.ReaderManagerConfigOption{
				readerManager.AddErrorHandleFn(readerErrorHandlerFn),
			}
			if mqConfig.isManualCommit {
				readerManagerConfigOptions = append(readerManagerConfigOptions, readerManager.ManualCommit)
			}
			reader, err = readerManager.CreateGroupIDReaderManager(
				ctx,
				mqConfig.brokers,
				mqConfig.topic,
				mqConfig.readerGroupID,
				readerManagerConfigOptions...,
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
		fmt.Println("yorkaaa", err)
		cancel()
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

func (m *mqTopic) Subscribe(key string, notify mq.Notify, options ...mq.ObserverOption) mq.Observer {
	observer := readerManager.CreateObserver(key, notify, options...)

	m.readerManager.AddObserver(observer)
	m.readerManager.StartConsume(context.Background())

	return observer
}

func (m *mqTopic) SubscribeWithManualCommit(key string, notify mq.NotifyWithManualCommit, options ...mq.ObserverOption) mq.Observer {
	observer := readerManager.CreateObserverWithManualCommit(key, notify, options...)

	m.readerManager.AddObserver(observer)
	m.readerManager.StartConsume(context.Background())

	return observer
}

func (m *mqTopic) UnSubscribe(observer mq.Observer) {
	m.readerManager.RemoveObserverWithHook(observer)
	m.readerManager.IfNoObserversThenStopConsume()
}

func (m *mqTopic) Produce(ctx context.Context, message mq.Message) error {
	if err := m.writerManager.WriteMessages(ctx, message); err != nil {
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

func createTopic(url, topic string, numPartitions, replicationFactor int) error {
	conn, err := kafka.Dial("tcp", url)
	if err != nil {
		return errors.Wrap(err, "get error failed")
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return errors.Wrap(err, "read partitions failed")
	}

	for _, p := range partitions {
		if topic == p.Topic {
			return errors.New("already has topic")
		}
	}

	controller, err := conn.Controller()
	if err != nil {
		return errors.Wrap(err, "get controller failed")
	}
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return errors.Wrap(err, "controller connect faile")
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return errors.Wrap(err, "create topics failed")
	}

	return nil
}
