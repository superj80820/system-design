package domain

import (
	context "context"
	"time"

	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/kit/core/endpoint"
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
	ReferenceID int

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
	OrderID   int
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
	TradingResultStatusTransfer
	TradingResultStatusDeposit
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

type SyncTradingUseCase interface {
	CreateOrder(ctx context.Context, messages *TradingEvent) (*MatchResult, error)
	CancelOrder(ctx context.Context, tradingEvent *TradingEvent) error
	Transfer(ctx context.Context, tradingEvent *TradingEvent) error

	Deposit(ctx context.Context, tradingEvent *TradingEvent) error

	GetSequenceID() int
	RecoverBySnapshot(*TradingSnapshot) error
}

type ExchangeRequestType string

const (
	UnknownExchangeRequestType      ExchangeRequestType = ""
	TickerExchangeRequestType       ExchangeRequestType = "ticker"
	FoundsExchangeRequestType       ExchangeRequestType = "funds"
	CandlesExchangeRequestType      ExchangeRequestType = "candles"
	Candles60ExchangeRequestType    ExchangeRequestType = "candles_60"
	Candles3600ExchangeRequestType  ExchangeRequestType = "candles_3600"
	Candles86400ExchangeRequestType ExchangeRequestType = "candles_86400"
	MatchExchangeRequestType        ExchangeRequestType = "match"
	Level2ExchangeRequestType       ExchangeRequestType = "level2"
	OrderExchangeRequestType        ExchangeRequestType = "order"
	PingExchangeRequestType         ExchangeRequestType = "ping"
)

type TradingNotifyRequest struct {
	Type        ExchangeRequestType `json:"type"`
	ProductIds  []string            `json:"product_ids,omitempty"`
	CurrencyIDs []string            `json:"currency_ids,omitempty"`
	Channels    []string            `json:"channels"`
	Token       string              `json:"token"` // TODO: what this?
}

type TradingUseCase interface {
	ConsumeTradingEventThenProduce(context.Context)
	ProduceCreateOrderTradingEvent(ctx context.Context, userID int, direction DirectionEnum, price, quantity decimal.Decimal) (*TradingEvent, error)
	ProduceCancelOrderTradingEvent(ctx context.Context, userID, orderID int) (*TradingEvent, error)
	ProduceDepositOrderTradingEvent(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TradingEvent, error)

	GetHistoryMatchDetails(maxResults int) ([]*MatchOrderDetail, error)
	GetUserHistoryMatchDetails(userID, orderID int) ([]*MatchOrderDetail, error)

	GetLatestSnapshot(context.Context) (*TradingSnapshot, error)
	SaveSnapshot(context.Context, *TradingSnapshot) error
	GetHistorySnapshot(context.Context) (*TradingSnapshot, error)
	RecoverBySnapshot(*TradingSnapshot) error

	Notify(ctx context.Context, userID int, stream endpoint.Stream[TradingNotifyRequest, any]) error

	Done() <-chan struct{}
	Err() error
	Shutdown() error
	// TODO
	// NewOrder(order *OrderEntity) (*MatchResult, error)
	// CancelOrder(ts time.Time, order *OrderEntity) error
	// GetMarketPrice() decimal.Decimal
	// GetLatestSequenceID() int
}
