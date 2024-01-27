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
	SubscribeTradeMessage(key string, notify func(*TradingEvent))
	SendTradeMessages(context.Context, []*TradingEvent)
	SaveMatchingDetailsWithIgnore(context.Context, []*MatchOrderDetail) error
	GetMatchingDetails(orderID int) ([]*MatchOrderDetail, error)
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

type TradingSequencerUseCase interface {
	ConsumeTradingEventThenProduce(context.Context)
	ProduceTradingEvent(context.Context, *TradingEvent) error
	Done() <-chan struct{}
	Err() error
}

type SyncTradingUseCase interface {
	CreateOrder(messages *TradingEvent) (*MatchResult, error)
	CancelOrder(tradingEvent *TradingEvent) error
	Transfer(tradingEvent *TradingEvent) error
}

type TradingUseCase interface {
	SyncTradingUseCase
	SubscribeTradingResult(fn func(tradingResult *TradingResult))
	GetLatestOrderBook() *OrderBookEntity
	SaveHistoryMatchDetailsFromTradingResult(*TradingResult)
	GetHistoryMatchDetails(orderID int) ([]*MatchOrderDetail, error)
	Done() <-chan struct{}
	Shutdown() error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
