package delivery

import (
	"context"

	"github.com/pkg/errors"

	"github.com/go-kit/kit/endpoint"
	"github.com/superj80820/system-design/domain"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	httpTransportKit "github.com/superj80820/system-design/kit/http/transport"
)

type DeleteReservesScheduleRequest struct {
	Key string `json:"key"`
}

type ReservesScheduleRequest struct {
	ReserveSchedules []*domain.TicketPlusReserveSchedule `json:"reserve_schedules"`
}

type ReservesScheduleResponse struct {
	ProcessReserveStatus []domain.ProcessReserveStatusResultEnum `json:"process_reserve_status"`
}

type GetReservesScheduleResponse struct {
	TicketPlusReserveSchedule map[string]*domain.TicketPlusReserveSchedule `json:"ticket_plus_reserve_schedule"`
}

var (
	DecodeDeleteReservesScheduleRequest  = httpTransportKit.DecodeJsonRequest[DeleteReservesScheduleRequest]
	EncodeDeleteReservesScheduleResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)

	DecodeReservesScheduleRequest  = httpTransportKit.DecodeJsonRequest[ReservesScheduleRequest]
	EncodeReservesScheduleResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)

	DecodeGetReservesScheduleRequest  = httpTransportKit.DecodeEmptyRequest
	EncodeGetReservesScheduleResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeJsonResponse)

	DecodeUpdateConfigRequest  = httpTransportKit.DecodeJsonRequest[domain.TicketPlusConfig]
	EncodeUpdateConfigResponse = httpMiddlewareKit.EncodeResponseSetSuccessHTTPCode(httpTransportKit.EncodeEmptyResponse)
)

func MakeUpdateConfigEndpoint(svc domain.ConfigService[domain.TicketPlusConfig]) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.TicketPlusConfig)
		if !svc.Set(req) {
			return nil, errors.New("update failed")
		}
		return nil, nil
	}
}

func MakeReservesScheduleEndpoint(svc domain.TicketPlusService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(ReservesScheduleRequest)
		reserveResult, err := svc.Reserves(ctx, req.ReserveSchedules)
		if err != nil {
			return nil, errors.New("update failed")
		}
		return reserveResult, nil
	}
}

func MakeGetReservesScheduleEndpoint(svc domain.TicketPlusService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		result := svc.GetReservesSchedule()
		return result, nil
	}
}

func MakeDeleteReservesScheduleEndpoint(svc domain.TicketPlusService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(DeleteReservesScheduleRequest)
		err = svc.DeleteReservesSchedule(req.Key)
		if err != nil {
			return nil, errors.Wrap(err, "not exist")
		}
		return nil, nil
	}
}
