package websocket

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
)

func MakeReadEventEndpoint(readEventKey string, eventSourceService domain.EventSourceService[*domain.TicketPlusEvent]) endpoint.BiStream[any, *domain.TicketPlusEvent] {
	return func(ctx context.Context, s endpoint.Stream[any, *domain.TicketPlusEvent]) error {
		getTicketPlusStatus, err := eventSourceService.GetEvent(readEventKey)
		if err != nil {
			return errors.Wrap(err, "get ticket plus status failed")
		}

		for {
			for val := range getTicketPlusStatus { // TODO: test
				if err := s.Send(val); err != nil {
					return err
				}
			}
		}
	}
}
