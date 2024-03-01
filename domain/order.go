package domain

import (
	context "context"
	"time"

	"github.com/shopspring/decimal"
)

type OrderType int

const (
	OrderTypeUnknown OrderType = iota
	OrderTypeMarket
	OrderTypeLimit
)

func (o OrderType) String() string {
	switch o {
	case OrderTypeMarket:
		return "market"
	case OrderTypeLimit:
		return "limit"
	case OrderTypeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

type DirectionEnum int

const (
	DirectionUnknown DirectionEnum = iota
	DirectionBuy
	DirectionSell
)

func (d DirectionEnum) String() string {
	switch d {
	case DirectionBuy:
		return "buy"
	case DirectionSell:
		return "sell"
	case DirectionUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

type OrderStatusEnum int

const (
	OrderStatusUnknown OrderStatusEnum = iota
	OrderStatusFullyFilled
	OrderStatusPartialFilled
	OrderStatusPending
	OrderStatusFullyCanceled
	OrderStatusPartialCanceled
)

func (o OrderStatusEnum) String() string {
	switch o {
	case OrderStatusUnknown:
		return "unknown"
	case OrderStatusFullyFilled:
		return "fully-filled"
	case OrderStatusPartialFilled:
		return "partial-filled"
	case OrderStatusPending:
		return "pending"
	case OrderStatusFullyCanceled:
		return "fully-canceled"
	case OrderStatusPartialCanceled:
		return "partial-canceled"
	default:
		return "unknown"
	}
}

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
	CreateOrder(ctx context.Context, sequenceID int, orderID, userID int, direction DirectionEnum, price, quantity decimal.Decimal, ts time.Time) (*OrderEntity, *TransferResult, error)
	RemoveOrder(ctx context.Context, orderID int) error
	GetOrder(orderID int) (*OrderEntity, error)
	GetUserOrders(userID int) (map[int]*OrderEntity, error)
	GetHistoryOrder(userID, orderID int) (*OrderEntity, error)
	GetHistoryOrders(userID, maxResults int) ([]*OrderEntity, error)
	GetOrdersData() ([]*OrderEntity, error)
	RecoverBySnapshot(*TradingSnapshot) error
	ConsumeOrderResultToSave(ctx context.Context, key string)
}

type OrderRepo interface {
	GetHistoryOrder(userID, orderID int) (*OrderEntity, error)
	GetHistoryOrders(userID, maxResults int) ([]*OrderEntity, error)
	SaveHistoryOrdersWithIgnore([]*OrderEntity) error

	ProduceOrderMQByTradingResult(ctx context.Context, tradingResult *TradingResult) error
	ConsumeOrderMQBatch(ctx context.Context, key string, notify func(sequenceID int, order []*OrderEntity) error)
}
