package http

import (
	"context"
	"strconv"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type accountRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type accountRegisterResponse struct {
	ID                      string     `json:"id"`
	Email                   string     `json:"email"`
	Name                    *string    `json:"name"`
	ProfilePhoto            *string    `json:"profilePhoto"`
	CreatedAt               *time.Time `json:"createdAt"`
	TwoStepVerificationType *string    `json:"twoStepVerificationType"`
	Band                    bool       `json:"band"`
}

var (
	DecodeAccountRegisterRequest  = httpTransportKit.DecodeJsonRequest[accountRegisterRequest]
	EncodeAccountRegisterResponse = httpTransportKit.EncodeJsonResponse
)

func MakeAccountRegisterEndpoint(svc domain.AccountService, tradingUseCase domain.TradingUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(accountRegisterRequest)
		account, err := svc.Register(req.Email, req.Password)
		if err != nil {
			return nil, err
		}
		if _, err := tradingUseCase.ProduceDepositOrderTradingEvent(ctx, int(account.ID), currencyUseCase.GetBaseCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		if _, err := tradingUseCase.ProduceDepositOrderTradingEvent(ctx, int(account.ID), currencyUseCase.GetQuoteCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		return &accountRegisterResponse{
			ID:    strconv.FormatInt(account.ID, 10),
			Email: req.Email,
		}, nil
	}
}
