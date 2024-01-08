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
	OrderStatusFullyCanceled
	OrderStatusPartialCanceled
)

var (
	ErrEmptyOrderBook = errors.New("empty order book error")
	ErrNoOrder        = errors.New("order not found error")
)

type MatchingUseCase interface {
	NewOrder(o *Order) (*MatchResult, error)
	CancelOrder(ts time.Time, o *Order) error
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
