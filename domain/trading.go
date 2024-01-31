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
	TradingEventDepositType
)

// TODO: abstract
type TradingEvent struct {
	ReferenceID int64

	EventType  TradingEventTypeEnum
	SequenceID int
	PreviousID int
	UniqueID   int // TODO: need?

	OrderRequestEvent *OrderRequestEvent
	OrderCancelEvent  *OrderCancelEvent
	TransferEvent     *TransferEvent
	DepositEvent      *DepositEvent

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

type DepositEvent struct {
	ToUserID   int
	AssetID    int
	Amount     decimal.Decimal
	Sufficient bool // TODO: what this?
}

type TradingRepo interface {
	SubscribeTradeEvent(key string, notify func(*TradingEvent))
	SendTradeEvent(context.Context, []*TradingEvent)

	SubscribeTradingResult(key string, notify func(*TradingResult))
	SendTradingResult(context.Context, *TradingResult) error

	SaveMatchingDetailsWithIgnore(context.Context, []*MatchOrderDetail) error
	GetMatchingDetails(orderID int) ([]*MatchOrderDetail, error)

	GetHistorySnapshot(context.Context) (*TradingSnapshot, error)
	SaveSnapshot(ctx context.Context, sequenceID int, usersAssetsData map[int]map[int]*UserAsset, ordersData []*OrderEntity, matchesData *MatchData) error

	Done() <-chan struct{}
	Err() error
	Shutdown()
}

type TradingResultStatus int

const (
	TradingResultStatusCreate TradingResultStatus = iota + 1
	TradingResultStatusCancel
)

type TradingSnapshot struct { // TODO: minify
	SequenceID  int                        `bson:"sequence_id"`
	UsersAssets map[int]map[int]*UserAsset `bson:"users_assets"`
	Orders      []*OrderEntity             `bson:"orders"`
	MatchData   *MatchData                 `bson:"match_data"`
}

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

	Deposit(tradingEvent *TradingEvent) error

	GetSequenceID() int
	RecoverBySnapshot(*TradingSnapshot) error
}

type TradingUseCase interface {
	CreateOrder(messages *TradingEvent) (*MatchResult, error)
	CancelOrder(tradingEvent *TradingEvent) error
	Transfer(tradingEvent *TradingEvent) error

	Deposit(tradingEvent *TradingEvent) error

	GetLatestOrderBook() *OrderBookEntity
	GetSequenceID() int
	ConsumeTradingResult(key string)
	GetHistoryMatchDetails(userID, orderID int) ([]*MatchOrderDetail, error)
	GetLatestSnapshot(context.Context) (*TradingSnapshot, error)
	SaveSnapshot(context.Context, *TradingSnapshot) error
	GetHistorySnapshot(context.Context) (*TradingSnapshot, error)
	RecoverBySnapshot(*TradingSnapshot) error
	Done() <-chan struct{}
	Shutdown() error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
