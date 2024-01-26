package mq

// import (
// 	"context"
// 	"encoding/json"

// 	"github.com/pkg/errors"
// 	readermanager "github.com/superj80820/system-design/kit/mq/reader_manager"
// 	"github.com/superj80820/system-design/kit/util"
// )

// type memoryMQ[T any] struct {
// 	messageCh chan []byte
// 	observers util.GenericSyncMap[any, func(*T)] // TODO: test key safe?
// }

// var _ MQTopic = (*memoryMQ)(nil)

// func CreateMemoryMQ[T any]() MQTopic {
// 	m := &memoryMQ[T]{
// 		messageCh: make(chan []byte),
// 	}

// 	go func() {
// 		for message := range m.messageCh {
// 			m.observers.Range(func(_ any, value func(*T)) bool {
// 				var v T
// 				if err := json.Unmarshal(message, &v); err != nil {
// 					panic(err) //TODO
// 				}
// 				value(&v)
// 				return true
// 			})
// 		}
// 	}()

// 	return m
// }

// // Done implements MQTopic.
// func (*memoryMQ[T]) Done() <-chan struct{} {
// 	panic("unimplemented")
// }

// // Err implements MQTopic.
// func (*memoryMQ[T]) Err() error {
// 	panic("unimplemented")
// }

// func (m *memoryMQ[T]) Produce(ctx context.Context, message Message) error {
// 	marshal, err := message.Marshal()
// 	if err != nil {
// 		return errors.Wrap(err, "marshal message failed")
// 	}
// 	m.messageCh <- marshal
// 	return nil
// }

// func (*memoryMQ[T]) Shutdown() bool {
// 	// TODO
// 	return true
// }

// func (m *memoryMQ[T]) Subscribe(key string, notify readermanager.Notify, options ...readermanager.ObserverOption) *readermanager.Observer {
// 	m.observers.Store(key, func(t *T) {
//         notify
//     })
// 	return nil // TODO
// }

// // UnSubscribe implements MQTopic.
// func (*memoryMQ[T]) UnSubscribe(observer *readermanager.Observer) {
// 	panic("unimplemented")
// }
