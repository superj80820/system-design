package mq

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
)

type Message interface {
	GetKey() string
	Marshal() ([]byte, error)
}

type Observer struct {
	notify          Notify
	unSubscribeHook UnSubscribeHook
}

type Notify func(message []byte) error
type UnSubscribeHook func() error

type ObserverOption func(*Observer)

func AddUnSubscribeHook(unSubscribeHook UnSubscribeHook) ObserverOption {
	return func(o *Observer) { o.unSubscribeHook = unSubscribeHook }
}

func defaultUnSubscribeHook() error { return nil }

type (
	WriterBalancer = kafka.Balancer

	Hash       = kafka.Hash
	RoundRobin = kafka.RoundRobin
)

const (
	FirstOffset = kafka.FirstOffset
	LastOffset  = kafka.LastOffset
)

type mqTopicConfig struct {
	writerBalancer WriterBalancer

	readerConsumeWay  consumeWay
	readerGroupID     string
	readerPartition   int
	readerStartOffset int64
}

type consumeWay int

const (
	consumeByDefault consumeWay = iota // no set
	consumeByGroupID
	consumeBySpecPartition
	consumeByPartitionsBindObserver
)

type MQTopicOption func(*mqTopicConfig)

func ProduceWay(balancer WriterBalancer) MQTopicOption {
	return func(mtc *mqTopicConfig) {
		mtc.writerBalancer = balancer
	}
}

func ConsumeByGroupID(groupID string, startOffset int64) MQTopicOption {
	return func(mtc *mqTopicConfig) {
		mtc.readerConsumeWay = consumeByGroupID
		mtc.readerGroupID = groupID
		mtc.readerStartOffset = startOffset
	}
}

func ConsumeBySpecPartition(partition int, startOffset int64) MQTopicOption {
	return func(mtc *mqTopicConfig) {
		mtc.readerConsumeWay = consumeBySpecPartition
		mtc.readerPartition = partition
		mtc.readerStartOffset = startOffset
	}
}

func ConsumeByPartitionsBindObserver(startOffset int64) MQTopicOption {
	return func(mtc *mqTopicConfig) {
		mtc.readerConsumeWay = consumeByPartitionsBindObserver
		mtc.readerStartOffset = startOffset
	}
}

type reader struct {
	*kafka.Reader
	observers map[*Observer]*Observer

	pauseCh chan context.CancelFunc
	startCh chan context.Context

	lock sync.RWMutex
}

func createReader(kafkaReader *kafka.Reader) *reader {
	r := &reader{
		Reader:    kafkaReader,
		observers: make(map[*Observer]*Observer),
		pauseCh:   make(chan context.CancelFunc, 1), // TODO: need? // TODO: number?
		startCh:   make(chan context.Context),       // TODO: need?
	}
	r.consume()

	return r
}

func (r *reader) consume() {
	go func() {
		for ctx := range r.startCh {
			fmt.Println("start consume", r.Reader.Config().Partition)
			for {
				m, err := r.ReadMessage(ctx)
				if err != nil {
					fmt.Println("pause consume", r.Reader.Config().Partition)
					break
				}

				fmt.Println("consume", m.Partition, string(m.Value))

				r.RangeAllObservers(func(_ *Observer, observer *Observer) bool {
					go observer.notify(m.Value)
					return true
				})
			}
		}
	}()
}

func (r *reader) StartConsume(ctx context.Context) bool {
	ctx, cancel := context.WithCancel(ctx)
	select {
	case r.startCh <- ctx:
		r.pauseCh <- cancel // TODO: async correct?
		return true
	default:
		cancel()
		return false
	}
}

func (r *reader) StopConsume() bool {
	select {
	case cancel := <-r.pauseCh:
		cancel()
		return true
	default:
		return false
	}
}

func (r *reader) GetObserver(observer *Observer) (*Observer, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if observerInstance, ok := r.observers[observer]; !ok {
		return nil, false
	} else {
		return observerInstance, true
	}
}

func (r *reader) AddObserver(observer *Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observers[observer]; ok {
		return false
	}
	r.observers[observer] = observer
	return true
}

func (r *reader) RemoveObserver(observer *Observer) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.observers[observer]; !ok {
		return false
	}
	delete(r.observers, observer)
	return true
}

func (r *reader) GetObserversLen() int {
	r.lock.RLock()
	defer r.lock.RUnlock()

	length := len(r.observers)
	return length
}

func (r *reader) RangeAllObservers(fn func(key *Observer, value *Observer) bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for key, value := range r.observers {
		if !fn(key, value) {
			return
		}
	}
}

type MQTopic struct {
	controllerConn   *kafka.Conn
	readerConsumeWay consumeWay

	writer  *kafka.Writer
	readers map[int]*reader
	topic   string
}

const SingletonReader int = 1

func CreateMQTopic(ctx context.Context, url, topic string, options ...MQTopicOption) (*MQTopic, error) {
	brokers := strings.Split(url, ",")

	conn, err := kafka.Dial("tcp", url) // TODO: check address usage
	if err != nil {
		panic(err.Error())
	}
	// TODO: close
	controller, err := conn.Controller()
	if err != nil {
		panic(err.Error())
	}
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		panic(err.Error())
	}
	// TODO: close

	config := &mqTopicConfig{
		writerBalancer: &kafka.RoundRobin{},

		readerStartOffset: FirstOffset,
	}

	for _, option := range options {
		option(config)
	}

	writerConfig := kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: config.writerBalancer,
		Dialer: &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
	}
	writer := kafka.NewWriter(writerConfig)

	readers := make(map[int]*reader)
	readerConfig := kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	}
	switch config.readerConsumeWay {
	case consumeByGroupID:
		readerConfig.GroupID = config.readerGroupID
		readerConfig.StartOffset = config.readerStartOffset

		kafkaReader := kafka.NewReader(readerConfig)
		reader := createReader(kafkaReader)

		go reader.StartConsume(ctx)

		readers[SingletonReader] = reader
	case consumeBySpecPartition:
		readerConfig.Partition = config.readerPartition

		kafkaReader := kafka.NewReader(readerConfig)
		kafkaReader.SetOffset(config.readerStartOffset)
		reader := createReader(kafkaReader)

		go reader.StartConsume(ctx)

		readers[SingletonReader] = reader
	case consumeByPartitionsBindObserver: // TODO: 一個room一個partition?
		partitions, err := controllerConn.ReadPartitions(topic)
		if err != nil {
			panic(err) // TODO
		}
		for _, partition := range partitions {
			// TODO: async problem?
			readerConfig.Partition = partition.ID

			kafkaReader := kafka.NewReader(readerConfig)
			kafkaReader.SetOffset(config.readerStartOffset)
			reader := createReader(kafkaReader)

			readers[readerConfig.Partition] = reader
		}
	}

	mq := &MQTopic{
		controllerConn:   controllerConn,
		readerConsumeWay: config.readerConsumeWay,
		writer:           writer,
		readers:          readers,
		topic:            topic,
	}

	return mq, nil
}

func (c *MQTopic) Subscribe(key string, notify Notify, options ...ObserverOption) *Observer {
	observerInstance := &Observer{
		notify:          notify,
		unSubscribeHook: defaultUnSubscribeHook,
	}

	for _, option := range options {
		option(observerInstance)
	}

	switch c.readerConsumeWay {
	case consumeByGroupID, consumeBySpecPartition:
		c.readers[SingletonReader].AddObserver(observerInstance)
	case consumeByPartitionsBindObserver: // TODO: 一個room一個partition?
		c.getReaderByKey(key).AddObserver(observerInstance) // TODO: use key
		c.getReaderByKey(key).StartConsume(context.Background())
	}

	return observerInstance
}

func (c *MQTopic) UnSubscribe(key string, observerInstance *Observer) {
	switch c.readerConsumeWay {
	case consumeByGroupID, consumeBySpecPartition:
		if _, ok := c.readers[SingletonReader].GetObserver(observerInstance); ok {
			go observerInstance.unSubscribeHook()
			c.readers[SingletonReader].RemoveObserver(observerInstance)
		}
	case consumeByPartitionsBindObserver: // TODO: 一個room一個partition?
		reader := c.getReaderByKey(key)
		if _, ok := reader.GetObserver(observerInstance); ok {
			go observerInstance.unSubscribeHook()
			reader.RemoveObserver(observerInstance)
			if reader.GetObserversLen() == 0 {
				reader.StopConsume()
			}
		}
	}

}

func (c *MQTopic) Produce(ctx context.Context, message Message) error {
	marshalMessage, err := message.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal message failed")
	}

	kafkaMsg := kafka.Message{
		Key:   []byte(message.GetKey()),
		Value: marshalMessage,
	}

	if err := c.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return errors.Wrap(err, "write messages to kafka failed")
	}

	return nil
}

func (c *MQTopic) getReaderByKey(key string) *reader { // TODO
	partitions, err := c.controllerConn.ReadPartitions(c.topic)
	if err != nil {
		panic(err) // TODO
	}
	partitionsInt := make([]int, len(partitions))
	for idx, partition := range partitions {
		partitionsInt[idx] = partition.ID // TODO: len
	}
	hash := &kafka.Hash{}

	hashKey := hash.Balance(kafka.Message{Key: []byte(key)}, partitionsInt...)

	fmt.Println("hash key", key, hashKey)

	return c.readers[hashKey]
}
