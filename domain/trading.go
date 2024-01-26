package domain

import (
	context "context"
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
	ReferenceID int64

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
	TradingLogResultStatusCancelType
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
	SendTradeMessages([]*TradingEvent)
	Done() <-chan struct{}
	Err() error
	Shutdown()
}

type TradingResultStatus int

const (
	TradingResultStatusCreate TradingResultStatus = iota + 1
	TradingResultStatusCancel
)

type TradingResult struct {
	TradingResultStatus TradingResultStatus
	TradingEvent        *TradingEvent
	MatchResult         *MatchResult
}

type AsyncTradingUseCase interface {
	SubscribeTradingResult(fn func(tradingResult *TradingResult))
	GetLatestOrderBook() *OrderBookEntity
	Done() <-chan struct{}
	Shutdown() error
}

type TradingSequencerUseCase interface {
	ConsumeTradingEvent(context.Context)
	ProduceTradingEvent(context.Context, *TradingEvent) error
	Done() <-chan struct{}
	Err() error
}

type TradingUseCase interface {
	CreateOrder(messages *TradingEvent) (*MatchResult, error)
	CancelOrder(tradingEvent *TradingEvent) error
	Transfer(tradingEvent *TradingEvent) error
	Shutdown() error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
