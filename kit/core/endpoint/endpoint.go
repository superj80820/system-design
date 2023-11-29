package endpoint

import (
	"context"
	"errors"
	"net/http"
)

type MessageType int

type Stream[IN, OUT any] interface {
	Send(out *OUT)
	Recv() (*IN, error)
}

func CreateServerStream[IN, OUT any](inCh chan *IN, outCh chan *OUT, doneCh chan bool) *ServerStream[IN, OUT] {
	return &ServerStream[IN, OUT]{
		inCh:   inCh,
		outCh:  outCh,
		doneCh: doneCh,
	}
}

type ServerStream[IN, OUT any] struct { // TODO: think name
	inCh   chan *IN
	outCh  chan *OUT
	doneCh chan bool
}

func (s *ServerStream[IN, OUT]) Send(out *OUT) {
	select {
	case <-s.doneCh:
	case s.outCh <- out:
	}
}

func (s *ServerStream[IN, OUT]) Recv() (*IN, error) {
	select {
	case <-s.doneCh:
		return nil, errors.New("stream already done")
	case in, ok := <-s.inCh:
		if !ok {
			return nil, errors.New("receive input failed")
		}
		return in, nil
	}
}

type Middleware[IN, OUT any] func(BiStream[IN, OUT]) BiStream[IN, OUT]

type Responser[T any] struct {
	Bid   func() (*T, error) // TODO: name correct?
	Final func() (*T, error)
}

type BiStream[IN, OUT any] func(context.Context, Stream[IN, OUT]) error

type RequestFunc func(context.Context, *http.Request) context.Context

type DecodeFunc[T any] func(context.Context, MessageType, []byte) (*T, error)
type EncodeFunc[T any] func(context.Context, *T) ([]byte, MessageType, error)

func Chain[IN, OUT any](outer Middleware[IN, OUT], others ...Middleware[IN, OUT]) Middleware[IN, OUT] { // TODO: no need generic
	return func(next BiStream[IN, OUT]) BiStream[IN, OUT] {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}
		return outer(next)
	}
}
