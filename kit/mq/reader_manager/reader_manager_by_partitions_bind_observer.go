package readermanager

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/segmentio/kafka-go"
	"github.com/superj80820/system-design/kit/util"
)

type KafkaConn interface {
	Broker() kafka.Broker
	Controller() (broker kafka.Broker, err error)
	Brokers() ([]kafka.Broker, error)
	DeleteTopics(topics ...string) error
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Offset() (offset int64, whence int)
	Seek(offset int64, whence int) (int64, error)
	Read(b []byte) (int, error)
	ReadMessage(maxBytes int) (kafka.Message, error)
	ReadBatch(minBytes, maxBytes int) *kafka.Batch
	ReadBatchWith(cfg kafka.ReadBatchConfig) *kafka.Batch
	ReadOffset(t time.Time) (int64, error)
	ReadFirstOffset() (int64, error)
	ReadLastOffset() (int64, error)
	ReadOffsets() (first, last int64, err error)
	ReadPartitions(topics ...string) (partitions []kafka.Partition, err error)
	Write(b []byte) (int, error)
	WriteMessages(msgs ...kafka.Message) (int, error)
	WriteCompressedMessages(codec kafka.CompressionCodec, msgs ...kafka.Message) (nbytes int, err error)
	WriteCompressedMessagesAt(codec kafka.CompressionCodec, msgs ...kafka.Message) (nbytes int, partition int32, offset int64, appendTime time.Time, err error)
	SetRequiredAcks(n int) error
	ApiVersions() ([]kafka.ApiVersion, error)
}

type partitionBindObserverReaderManager struct {
	*readerManager

	kafkaControllerConnProvider func() (KafkaConn, error)
	kafkaControllerConn         KafkaConn

	readerProvider func(partitionID int) *Reader
	readers        map[int]*Reader

	topic              string
	topicPartitionInfo []kafka.Partition

	watchBalanceDuration time.Duration

	consistentHash util.ConsistentHash

	lock *sync.RWMutex
}

type partitionBindObserverReaderManagerOption func(*partitionBindObserverReaderManager)

func useMockKafkaControllerConnProvider(fn func() (KafkaConn, error)) readerManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.partitionBindObserverReaderManagerOptions = append(
			rmc.partitionBindObserverReaderManagerOptions,
			func(pborm *partitionBindObserverReaderManager) {
				pborm.kafkaControllerConnProvider = fn
			})
	}
}

func setWatchBalanceDuration(duration time.Duration) readerManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.partitionBindObserverReaderManagerOptions = append(
			rmc.partitionBindObserverReaderManagerOptions,
			func(pborm *partitionBindObserverReaderManager) {
				pborm.watchBalanceDuration = duration
			},
		)
	}
}

func defaultKafkaControllerConnProvider(url string) func() (KafkaConn, error) {
	return func() (KafkaConn, error) {
		conn, err := kafka.Dial("tcp", url)
		if err != nil {
			return nil, errors.Wrap(err, "connect kafka failed")
		}
		defer conn.Close()

		controller, err := conn.Controller()
		if err != nil {
			return nil, errors.Wrap(err, "get kafka current controller failed")
		}

		var controllerConn *kafka.Conn
		controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
		if err != nil {
			return nil, errors.Wrap(err, "connect kafka failed")
		}

		return controllerConn, nil
	}
}

func CreatePartitionBindObserverReaderManager(url string, startOffset int64, brokers []string, topic string, options ...readerManagerConfigOption) (ReaderManager, error) {
	config := new(readerManagerConfig)
	for _, option := range options {
		option(config)
	}

	rm := &partitionBindObserverReaderManager{
		readerManager:               createReaderManager(config.readerManagerOptions...),
		kafkaControllerConnProvider: defaultKafkaControllerConnProvider(url),
		readerProvider: func(partitionID int) *Reader {
			return createReader(kafka.ReaderConfig{
				Brokers:     brokers,
				Topic:       topic,
				MinBytes:    10e3, // 10KB
				MaxBytes:    10e6, // 10MB
				Partition:   partitionID,
				StartOffset: startOffset,
			}, config.readerOptions...)
		},

		topic: topic,

		readers: make(map[int]*Reader),
		lock:    &sync.RWMutex{},
	}

	for _, option := range config.partitionBindObserverReaderManagerOptions {
		option(rm)
	}

	if err := rm.setKafkaControllerConn(); err != nil {
		return nil, errors.Wrap(err, "set kafka controller connect failed")
	}
	if _, err := rm.fetchPartitionsInfoThenSetReaders(); err != nil {
		return nil, errors.Wrap(err, "fetch partitions information then set readers failed")
	}
	go func() {
		ticker := time.NewTicker(rm.watchBalanceDuration)
		for range ticker.C {
			if _, err := rm.fetchPartitionsInfoThenSetReaders(); err != nil {
				rm.errorHandleFn(errors.Wrap(err, "fetch partitions information then set readers failed"))
				continue
			}
		}
	}()

	return rm, nil
}

func (p *partitionBindObserverReaderManager) Run() {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, reader := range p.readers {
		reader.Run()
	}
}

func (p *partitionBindObserverReaderManager) StartConsume(ctx context.Context) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, reader := range p.readers {
		reader.StartConsume(ctx)
	}
	return true
}

func (p *partitionBindObserverReaderManager) SyncStartConsume(ctx context.Context) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, reader := range p.readers {
		reader.SyncStartConsume(ctx)
	}
	return true
}

func (p *partitionBindObserverReaderManager) StopConsume() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, reader := range p.readers {
		reader.StopConsume()
	}
	return true
}

func (p *partitionBindObserverReaderManager) AddObserver(observer *Observer) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	ok := p.readers[p.consistentHash.Get(observer.key, len(p.topicPartitionInfo))].AddObserver(observer)

	return ok
}

func (p *partitionBindObserverReaderManager) RemoveObserverWithHook(observer *Observer) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	if ok := p.readers[p.consistentHash.Get(observer.key, len(p.topicPartitionInfo))].RemoveObserver(observer); !ok {
		return false
	}
	go observer.unSubscribeHook()

	return true
}

func (p *partitionBindObserverReaderManager) IfNoObserversThenStopConsume() {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, reader := range p.readers {
		observerLen := reader.GetObserversLen()
		if observerLen == 0 {
			reader.StopConsume()
		}
	}
}

func (p *partitionBindObserverReaderManager) GetObserversLen() int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	var allObserversLen int
	for _, reader := range p.readers {
		allObserversLen += reader.GetObserversLen()
	}
	return allObserversLen
}

func (p *partitionBindObserverReaderManager) setKafkaControllerConn() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	var err error
	p.kafkaControllerConn, err = p.kafkaControllerConnProvider()
	if err != nil {
		return errors.Wrap(err, "create kafka controller connect failed")
	}

	return nil
}

func (p *partitionBindObserverReaderManager) fetchPartitionsInfoThenSetReaders() (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	partitions, err := p.kafkaControllerConn.ReadPartitions(p.topic)
	if err != nil {
		return false, errors.Wrap(err, "read partitions information failed")
	}
	if len(p.topicPartitionInfo) == len(partitions) {
		return false, nil
	}
	p.topicPartitionInfo = partitions

	for _, partition := range p.topicPartitionInfo {
		r := p.readerProvider(partition.ID)

		if _, ok := p.readers[partition.ID]; !ok {
			p.readers[partition.ID] = r
		}
	}

	var observerInfos []struct {
		originReaderIdx int
		originReader    *Reader
		observer        *Observer
	}

	for idx, reader := range p.readers {
		reader.RangeAllObservers(func(key, value *Observer) bool {
			observerInfos = append(observerInfos, struct {
				originReaderIdx int
				originReader    *Reader
				observer        *Observer
			}{
				originReaderIdx: idx,
				originReader:    reader,
				observer:        value,
			})
			return true
		})
	}

	for _, observerInfo := range observerInfos {
		nextIdx := p.consistentHash.Get(observerInfo.observer.key, len(p.topicPartitionInfo))
		if observerInfo.originReaderIdx != nextIdx {
			observerInfo.originReader.RemoveObserver(observerInfo.observer)
			p.readers[nextIdx].AddObserver(observerInfo.observer)
		}
	}

	return true, nil
}
