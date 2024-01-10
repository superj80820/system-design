package domain

import (
	"time"

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

func (o OrderStatusEnum) IsFinalStatus() bool {
	switch o {
	case OrderStatusFullyFilled:
		return true
	case OrderStatusPartialFilled:
		return true
	case OrderStatusPending:
		return false
	case OrderStatusFullyCanceled:
		return true
	case OrderStatusPartialCanceled:
		return false
	default:
		return false
	}
}

type OrderBookEntity struct {
	SequenceID int
	Price      decimal.Decimal
	Sell       []*OrderBookItemEntity
	Buy        []*OrderBookItemEntity
}

type OrderBookItemEntity struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

type OrderEntity struct {
	ID         int
	SequenceID int
	UserID     int

	Price     decimal.Decimal
	Direction DirectionEnum
	Status    OrderStatusEnum

	Quantity         decimal.Decimal
	UnfilledQuantity decimal.Decimal

	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrderUseCase interface {
	CreateOrder(sequenceID int, orderID, userID int, direction DirectionEnum, price, quantity decimal.Decimal, ts time.Time) (*OrderEntity, error)
	RemoveOrder(orderID int) error
	GetOrder(orderID int) (*OrderEntity, error)
	GetUserOrders(userID int) (map[int]*OrderEntity, error)
}
