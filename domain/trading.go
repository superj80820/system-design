package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type TradingEventTypeEnum int

const (
	TradingEventUnknownType TradingEventTypeEnum = iota
	TradingEventCreateOrderType
	TradingEventCancelOrderType
	TradingEventTransferType
)

// TODO: abstract
type TradingEvent struct {
	RefID string // TODO: what this?

	EventType  TradingEventTypeEnum
	SequenceID int
	PreviousID int
	UniqueID   int

	OrderRequestEvent *OrderRequestEvent
	OrderCancelEvent  *OrderCancelEvent
	TransferEvent     *TransferEvent

	CreatedAt time.Time
}

type OrderRequestEvent struct {
	UserID    int
	Direction DirectionEnum
	Price     decimal.Decimal
	Quantity  decimal.Decimal
}

type OrderCancelEvent struct {
	UserID  int
	OrderId int
}

type TradingLogResultStatusTypeEnum int

const (
	TradingLogResultStatusUnknownType TradingLogResultStatusTypeEnum = iota
	TradingLogResultStatusOKType
)

type TradingLogResult struct {
	StatusType TradingLogResultStatusTypeEnum
}

type TransferEvent struct {
	FromUserID int
	ToUserID   int
	AssetID    int
	Amount     decimal.Decimal
	Sufficient bool // TODO: what this?
}

type TradingRepo interface {
	SubscribeTradeMessage(notify func(*TradingEvent))
	SendTradeMessages(*TradingEvent)
	Done() <-chan struct{}
	Err() error
	Shutdown()
}

type AsyncTradingUseCase interface {
	AsyncEventProcess(ctx context.Context) error
	AsyncDBProcess(ctx context.Context) error
	AsyncTickProcess(ctx context.Context) error
	AsyncNotifyProcess(ctx context.Context) error
	AsyncOrderBookProcess(ctx context.Context) error
	AsyncTradingLogResultProcess(ctx context.Context) error
	Done() <-chan struct{}
	Shutdown() error
}

type TradingUseCase interface {
	CreateOrder(messages *TradingEvent) (*MatchResult, error)
	CancelOrder(tradingEvent *TradingEvent) error
	Transfer(tradingEvent *TradingEvent) error
	IsOrderBookChanged() bool
	Shutdown() error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
