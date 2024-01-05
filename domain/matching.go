package domain

import (
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

type DirectionEnum int

const (
	DirectionUnknown DirectionEnum = iota
	DirectionBuy
	DirectionSell
)

type OrderStatusEnum int

const (
	OrderStatusUnknown OrderStatusEnum = iota
	OrderStatusFullyFilled
	OrderStatusPartialFilled
	OrderStatusPending
)

var ErrNoOrder = errors.New("no order error")

type MatchingUseCase interface {
	NewOrder(order *Order) (*MatchResult, error)
	CancelOrder()
}

type Order struct {
	SequenceId       int // TODO: use long
	Price            decimal.Decimal
	Direction        DirectionEnum
	Quantity         decimal.Decimal
	UnfilledQuantity decimal.Decimal
	Status           OrderStatusEnum
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MatchResult struct {
	TakerOrder   *Order
	MatchDetails []*MatchDetailRecord
}

type MatchDetailRecord struct {
	price      decimal.Decimal
	quantity   decimal.Decimal
	takerOrder *Order
	makerOrder *Order
}
