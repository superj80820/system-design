package domain

import (
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

type TransferEvent struct {
	FromUserID int
	ToUserID   int
	AssetID    int
	Amount     decimal.Decimal
	Sufficient bool // TODO: what this?
}

type TradingUseCase interface {
	ProcessMessages(messages []*TradingEvent) error
	CreateOrder(tradingEvent *TradingEvent) error
	CancelOrder(tradingEvent *TradingEvent) error
	Transfer(tradingEvent *TradingEvent) error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
