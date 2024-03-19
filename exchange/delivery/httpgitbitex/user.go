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
	utilKit "github.com/superj80820/system-design/kit/util"
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

func MakeAccountRegisterEndpoint(svc domain.AccountUseCase, sequenceTradingUseCase domain.SequenceTradingUseCase, currencyUseCase domain.CurrencyUseCase) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(accountRegisterRequest)
		account, err := svc.Register(req.Email, req.Password)
		if err != nil {
			return nil, err
		}
		userID, err := utilKit.SafeInt64ToInt(account.ID)
		if err != nil {
			return nil, errors.Wrap(err, "safe int64 to int failed")
		}
		if _, err := sequenceTradingUseCase.ProduceDepositOrderTradingEvent(ctx, userID, currencyUseCase.GetBaseCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		if _, err := sequenceTradingUseCase.ProduceDepositOrderTradingEvent(ctx, userID, currencyUseCase.GetQuoteCurrencyID(), decimal.NewFromInt(10000000000)); err != nil {
			return nil, errors.Wrap(err, "produce trading event failed")
		}
		return &accountRegisterResponse{
			ID:    strconv.Itoa(userID),
			Email: req.Email,
		}, nil
	}
}
