package eventsourcebotservice

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"

	"github.com/gorilla/websocket"
)

type eventSource[T any] struct {
	conn *websocket.Conn
}

func CreateEventSource[T any](websocketURL string) (domain.EventSourceRepo[T], error) {
	c, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "connect websocket failed")
	}

	return &eventSource[T]{conn: c}, nil
}

func (e *eventSource[T]) AddEvent(event T) {
	// no need
}

func (e *eventSource[T]) ReadEvent(key string) (chan T, error) {
	readCh := make(chan T)

	go func() {
		defer close(readCh)

		for {
			_, messageBytes, err := e.conn.ReadMessage()
			if err != nil {
				return
			}
			var message T
			if err := json.Unmarshal(messageBytes, &message); err != nil {
				return
			}
			readCh <- message
		}
	}()

	return readCh, nil
}
