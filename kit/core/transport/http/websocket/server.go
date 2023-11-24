package websocket

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/websocket"
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
	upgrader = websocket.Upgrader{}
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

	ws, err := upgrader.Upgrade(w, r, s.httpResponseHeader)
	if err != nil {
		// 	s.errorHandler.Handle(ctx, err) // TODO
		return
	}

	inCh, outCh, doneCh, errCh := make(chan *IN), make(chan *OUT), make(chan bool), make(chan error)
	goWithErrorHandle := func(fn func() error) {
		go func() {
			select {
			case <-doneCh:
			case errCh <- fn():
			}
		}()
	}

	stream := endpoint.Stream[IN, OUT]{
		InCh:   inCh,
		OutCh:  outCh,
		DoneCh: doneCh,
		ErrCh:  errCh,
	}

	wg := new(sync.WaitGroup)
	wg.Add(3)
	goWithErrorHandle(func() error {
		defer wg.Done()
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
			case <-doneCh:
				return nil
			case inCh <- in:
			}
		}
	})

	goWithErrorHandle(func() error {
		defer wg.Done()

		for {
			select {
			case <-doneCh:
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
	})

	goWithErrorHandle(func() error {
		defer wg.Done()

		if err := s.e(ctx, stream); err != nil {
			return err
		}

		return nil
	})

	go func() {
		err := <-errCh
		// 	s.errorHandler.Handle(ctx, err) // TODO
		s.errorEncoder(ctx, err, ws)
		close(doneCh)
		close(outCh)
		ws.Close()
		return
	}()

	wg.Wait()

	fmt.Println("close all")

	// go func() { // close after doneCh case
	// 	<-ctx.Done()
	// 	close(doneCh)
	// 	close(outCh)
	// 	ws.Close()
	// 	return nil
	// }()

	// if err := eg.Wait(); err != nil {
	// 	once.Do(func() {
	// 		// 	s.errorHandler.Handle(ctx, err) // TODO
	// 		s.errorEncoder(ctx, err, ws)
	// 	})
	// 	return
	// }
}
