package matching

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type orderKey struct {
	sequenceId int // TODO: use long
	price      decimal.Decimal
}

type order struct {
	*domain.Order
}

func createOrder(sequenceId int, price decimal.Decimal, direction domain.DirectionEnum, quantity decimal.Decimal) *order {
	return &order{
		&domain.Order{
			SequenceId:       sequenceId,
			Price:            price,
			Direction:        direction,
			Quantity:         quantity,
			UnfilledQuantity: quantity,
			Status:           domain.OrderStatusPending,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}
}

func (o *order) updateOrder(unfilledQuantity decimal.Decimal, orderStatus domain.OrderStatusEnum, updatedAt time.Time) {
	o.UnfilledQuantity = unfilledQuantity
	o.Status = orderStatus
	o.UpdatedAt = updatedAt
}
