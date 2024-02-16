package ws

import (
	"context"
	"errors"

	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	httpKit "github.com/superj80820/system-design/kit/http"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
)

var DecodeStreamExchangeRequest = wsKit.JsonDecodeRequest[domain.TradingNotifyRequest]

func MakeExchangeEndpoint(svc domain.TradingUseCase) endpoint.BiStream[domain.TradingNotifyRequest, any] {
	return func(ctx context.Context, s endpoint.Stream[domain.TradingNotifyRequest, any]) error {
		userID := httpKit.GetUserID(ctx)
		if userID == 0 {
			return errors.New("not found user id") // TODO: delete
		}
		return svc.Notify(ctx, userID, s)
	}
}
