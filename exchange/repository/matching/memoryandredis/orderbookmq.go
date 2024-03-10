package memoryandredis

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
)

type mqOrderBookMessage struct {
	*domain.OrderBookL3Entity
}

var _ mq.Message = (*mqOrderBookMessage)(nil)

func (m *mqOrderBookMessage) GetKey() string {
	return strconv.Itoa(m.OrderBookL3Entity.SequenceID)
}

func (m *mqOrderBookMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (ob *orderBookRepo) ProduceOrderBook(ctx context.Context, orderBook *domain.OrderBookL3Entity) error {
	if err := ob.orderBookMQTopic.Produce(ctx, &mqOrderBookMessage{
		OrderBookL3Entity: orderBook,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (ob *orderBookRepo) ConsumeOrderBook(ctx context.Context, key string, notify func(*domain.OrderBookL3Entity) error) {
	ob.orderBookMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqOrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookL3Entity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

type mqL1OrderBookMessage struct {
	*domain.OrderBookL1Entity
}

var _ mq.Message = (*mqL1OrderBookMessage)(nil)

func (m *mqL1OrderBookMessage) GetKey() string {
	return strconv.Itoa(m.OrderBookL1Entity.SequenceID)
}

func (m *mqL1OrderBookMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (ob *orderBookRepo) ProduceL1OrderBook(ctx context.Context, l1OrderBook *domain.OrderBookL1Entity) error {
	if err := ob.l1OrderBookMQTopic.Produce(ctx, &mqL1OrderBookMessage{
		OrderBookL1Entity: l1OrderBook,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (ob *orderBookRepo) ConsumeL1OrderBook(ctx context.Context, key string, notify func(*domain.OrderBookL1Entity) error) {
	ob.l1OrderBookMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqL1OrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookL1Entity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

type mqL2OrderBookMessage struct {
	*domain.OrderBookL2Entity
}

var _ mq.Message = (*mqL2OrderBookMessage)(nil)

func (m *mqL2OrderBookMessage) GetKey() string {
	return strconv.Itoa(m.OrderBookL2Entity.SequenceID)
}

func (m *mqL2OrderBookMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (ob *orderBookRepo) ProduceL2OrderBook(ctx context.Context, l2OrderBook *domain.OrderBookL2Entity) error {
	if err := ob.l2OrderBookMQTopic.Produce(ctx, &mqL2OrderBookMessage{
		OrderBookL2Entity: l2OrderBook,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (ob *orderBookRepo) ConsumeL2OrderBook(ctx context.Context, key string, notify func(*domain.OrderBookL2Entity) error) {
	ob.l2OrderBookMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqL2OrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookL2Entity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

type mqL3OrderBookMessage struct {
	*domain.OrderBookL3Entity
}

var _ mq.Message = (*mqL3OrderBookMessage)(nil)

func (m *mqL3OrderBookMessage) GetKey() string {
	return strconv.Itoa(m.OrderBookL3Entity.SequenceID)
}

func (m *mqL3OrderBookMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (ob *orderBookRepo) ProduceL3OrderBook(ctx context.Context, l3OrderBook *domain.OrderBookL3Entity) error {
	if err := ob.l3OrderBookMQTopic.Produce(ctx, &mqL3OrderBookMessage{
		OrderBookL3Entity: l3OrderBook,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (ob *orderBookRepo) ConsumeL3OrderBook(ctx context.Context, key string, notify func(*domain.OrderBookL3Entity) error) {
	ob.l3OrderBookMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqL3OrderBookMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.OrderBookL3Entity); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}
