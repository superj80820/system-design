package websocket

import (
	"context"
	"fmt"
	"net/http"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/websocket"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/kit/core/endpoint"
)

type ServerOptionFunc interface {
	appendBefore(before ...httptransport.RequestFunc)
	setErrorEncoder(ee ErrorEncoder)
	addHTTPResponseHeader(http.Header)
}

type ServerOption func(ServerOptionFunc)

func ServerBefore(before ...httptransport.RequestFunc) ServerOption {
	return func(s ServerOptionFunc) {
		s.appendBefore(before...)
	}
}

type ErrorEncoder func(ctx context.Context, err error, conn *websocket.Conn)

func ServerErrorEncoder(ee ErrorEncoder) ServerOption {
	return func(s ServerOptionFunc) { s.setErrorEncoder(ee) }
}

func AddHTTPResponseHeader(header http.Header) ServerOption {
	return func(s ServerOptionFunc) { s.addHTTPResponseHeader(header) }
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type Server[IN, OUT any] struct {
	e endpoint.BiStream[IN, OUT]

	dec endpoint.DecodeFunc[IN]
	enc endpoint.EncodeFunc[OUT]

	errorEncoder       ErrorEncoder
	before             []httptransport.RequestFunc
	httpResponseHeader http.Header
}

func (s *Server[IN, OUT]) appendBefore(before ...httptransport.RequestFunc) {
	s.before = append(s.before, before...)
}

func (s *Server[IN, OUT]) setErrorEncoder(ee ErrorEncoder) {
	s.errorEncoder = ee
}

func (s *Server[IN, OUT]) addHTTPResponseHeader(header http.Header) {
	s.httpResponseHeader = header
}

func NewServer[IN, OUT any](
	e endpoint.BiStream[IN, OUT],
	dec endpoint.DecodeFunc[IN],
	enc endpoint.EncodeFunc[OUT],
	options ...ServerOption,
) *Server[IN, OUT] {
	s := &Server[IN, OUT]{
		e:   e,
		dec: dec,
		enc: enc,
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Server[IN, OUT]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// TODO
	// if len(s.finalizer) > 0 {
	// 	iw := &interceptingWriter{w, http.StatusOK, 0}
	// 	defer func() {
	// 		ctx = context.WithValue(ctx, ContextKeyResponseHeaders, iw.Header())
	// 		ctx = context.WithValue(ctx, ContextKeyResponseSize, iw.written)
	// 		for _, f := range s.finalizer {
	// 			f(ctx, iw.code, r)
	// 		}
	// 	}()
	// 	w = iw.reimplementInterfaces()
	// }

	for _, f := range s.before {
		ctx = f(ctx, r)
	}

	// TODO
	// for _, f := range s.after {
	// 	ctx = f(ctx, w)
	// }

	// ws, err := upgrader.Upgrade(w, r, nil) // TODO: figure out why js WebSocket connection to 'ws://127.0.0.1:9090/ws' failed
	ws, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
	if err != nil {
		// 	s.errorHandler.Handle(ctx, err) // TODO
		return
	}

	inCh, outCh, doneCh := make(chan IN), make(chan OUT), make(chan bool)
	stream := endpoint.CreateServerStream[IN, OUT](inCh, outCh, doneCh)

	ctx, cancel := context.WithCancel(ctx)
	var g run.Group
	interruptFunc := func(err error) {
		cancel()
		// 	s.errorHandler.Handle(ctx, err) // TODO
		// fmt.Println("get err(TODO)", err)
		s.errorEncoder(ctx, err, ws)
		ws.Close()
	}
	g.Add(func() error {
		defer close(inCh)
		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				return err
			}

			in, err := s.dec(ctx, endpoint.MessageType(mt), msg)
			if err != nil {
				return errors.Wrap(err, "decode failed")
			}

			select {
			case <-ctx.Done():
				return nil
			case inCh <- in:
			}
		}
	}, func(err error) {
		interruptFunc(err)
	})

	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case out, ok := <-outCh:
				if !ok {
					return nil
				}
				msg, mt, err := s.enc(ctx, out)
				if err != nil {
					return errors.Wrap(err, "encode failed")
				}
				if err := ws.WriteMessage(int(mt), msg); err != nil {
					return errors.Wrap(err, "send message failed")
				}
			}
		}
	}, func(err error) {
		interruptFunc(err)
	})

	g.Add(func() error {
		if err := s.e(ctx, stream); err != nil {
			return err
		}
		return nil
	}, func(err error) {
		interruptFunc(err)
	})

	g.Run()

	close(doneCh)
	close(outCh)

	fmt.Println("close all")
}
