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

type OrderBookL1Entity struct {
	SequenceID int
	Price      decimal.Decimal
	BestAsk    *OrderBookL1ItemEntity
	BestBid    *OrderBookL1ItemEntity
}

type OrderBookL1ItemEntity struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

type OrderBookL2Entity struct {
	SequenceID int
	Price      decimal.Decimal
	Sell       []*OrderBookL2ItemEntity
	Buy        []*OrderBookL2ItemEntity
}

type OrderBookL2ItemEntity struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

type OrderBookL3Entity struct {
	SequenceID int
	Price      decimal.Decimal
	Sell       []*OrderBookL3ItemEntity
	Buy        []*OrderBookL3ItemEntity
}

type OrderBookL3ItemEntity struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Orders   []*OrderL3Entity
}

type OrderL3Entity struct {
	SequenceID int
	OrderID    int
	Quantity   decimal.Decimal
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

func (o *OrderEntity) Clone() *OrderEntity {
	return &OrderEntity{
		ID:               o.ID,
		SequenceID:       o.SequenceID,
		UserID:           o.UserID,
		Price:            o.Price,
		Direction:        o.Direction,
		Status:           o.Status,
		Quantity:         o.Quantity,
		UnfilledQuantity: o.UnfilledQuantity,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
	}
}

type OrderUseCase interface {
	CreateOrder(ctx context.Context, sequenceID int, orderID, userID int, direction DirectionEnum, price, quantity decimal.Decimal, ts time.Time) (*OrderEntity, *TransferResult, error)
	RemoveOrder(ctx context.Context, orderID int) error
	UpdateOrder(ctx context.Context, orderID int, unfilledQuantity decimal.Decimal, status OrderStatusEnum, updatedAt time.Time) error
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
	SaveHistoryOrdersWithIgnore(sequenceID int, orders []*OrderEntity) error

	ProduceOrderMQByTradingResult(ctx context.Context, tradingResult *TradingResult) error
	ConsumeOrderMQBatch(ctx context.Context, key string, notify func(sequenceID int, order []*OrderEntity, commitFn func() error) error)
}
