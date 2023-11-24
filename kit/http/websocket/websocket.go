package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/superj80820/system-design/kit/code"
	"go.opentelemetry.io/otel/trace"

	"github.com/superj80820/system-design/kit/core/endpoint"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func CustomHeaderFromCtx(ctx context.Context) http.Header {
	return http.Header{
		"X-B3-TraceId": {trace.SpanContextFromContext(ctx).TraceID().String()},
	}
}

func JsonDecodeRequest[T any](ctx context.Context, messageType endpoint.MessageType, data []byte) (*T, error) {
	var req T
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, errors.Wrap(err, "json unmarshal failed")
	}
	return &req, nil
}

func JsonEncodeResponse[T any](ctx context.Context, resp *T) ([]byte, endpoint.MessageType, error) {
	data, err := json.Marshal(*resp)
	if err != nil {
		return nil, 0, errors.Wrap(err, "json marshal failed")
	}
	return data, websocket.TextMessage, nil
}

func EncodeWSErrorResponse() func(ctx context.Context, err error, conn *websocket.Conn) {
	return func(ctx context.Context, err error, conn *websocket.Conn) {
		if err == nil {
			panic("encodeError with nil error")
		}

		errorCode := code.CreateWebsocketError(ctx, code.ParseErrorCode(err))
		jsonData, err := json.Marshal(errorCode)
		if err != nil {
			panic("create websocket error failed") // TODO
		}

		conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(errorCode.WebsocketCode, string(jsonData)),
			time.Now().Add(time.Second),
		) // TODO: error handling
	}
}
