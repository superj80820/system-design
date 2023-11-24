package endpoint

import (
	"context"
	"net/http"
)

type MessageType int

type Stream[IN, OUT any] struct {
	InCh   chan *IN
	OutCh  chan *OUT
	DoneCh chan bool
	ErrCh  chan error
}

func (s *Stream[IN, OUT]) SendToOut(out *OUT) {
	select {
	case <-s.DoneCh:
	case s.OutCh <- out:
	}
}

func (s *Stream[IN, OUT]) RecvFromIn() (*IN, bool) {
	select {
	case <-s.DoneCh:
		return nil, false
	case in, ok := <-s.InCh:
		if !ok {
			return nil, false
		}
		return in, true
	}
}

func (s *Stream[IN, OUT]) AddInMiddleware(fn func(in *IN) (*IN, error)) Stream[IN, OUT] { // TODO: pointer
	nextInCh := make(chan *IN)
	go func() {
		defer close(nextInCh)
		for in := range s.InCh {
			in, err := fn(in)
			if err != nil {
				s.ErrCh <- err
				return
			}
			nextInCh <- in
		}
	}()
	return Stream[IN, OUT]{
		InCh:   nextInCh,
		OutCh:  s.OutCh,
		DoneCh: s.DoneCh,
		ErrCh:  s.ErrCh,
	}
}

func (s *Stream[IN, OUT]) AddOutMiddleware(fn func(out *OUT) (*OUT, error)) {
	nextOutCh := make(chan *OUT)
	go func() {
		defer close(nextOutCh)
		for out := range s.OutCh {
			out, err := fn(out)
			if err != nil {
				s.ErrCh <- err
			}
			nextOutCh <- out
		}
	}()
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
