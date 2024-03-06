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
	SequenceID          int
	TradingResultStatus TradingResultStatus
	TradingEvent        *TradingEvent
	MatchResult         *MatchResult
	CancelOrderResult   *CancelResult
	TransferResult      *TransferResult
}

type SyncTradingUseCase interface {
	CreateOrder(ctx context.Context, messages *TradingEvent) (*MatchResult, *TransferResult, error)
	CancelOrder(ctx context.Context, tradingEvent *TradingEvent) (*CancelResult, *TransferResult, error)
	Transfer(ctx context.Context, tradingEvent *TradingEvent) (*TransferResult, error)

	Deposit(ctx context.Context, tradingEvent *TradingEvent) (*TransferResult, error)

	GetSequenceID() int
	RecoverBySnapshot(*TradingSnapshot) error
}

type ExchangeRequestType string

const (
	UnknownExchangeRequestType ExchangeRequestType = ""
	TickerExchangeRequestType  ExchangeRequestType = "ticker"
	AssetsExchangeRequestType  ExchangeRequestType = "assets"
	CandlesExchangeRequestType ExchangeRequestType = "candles"
	MatchExchangeRequestType   ExchangeRequestType = "match"
	OrderExchangeRequestType   ExchangeRequestType = "order"
	PingExchangeRequestType    ExchangeRequestType = "ping"
)

type ExchangeResponseType string

const (
	UnknownExchangeResponseType   ExchangeResponseType = ""
	TickerExchangeResponseType    ExchangeResponseType = "ticker"
	AssetExchangeResponseType     ExchangeResponseType = "asset"
	MatchExchangeResponseType     ExchangeResponseType = "match"
	OrderBookExchangeResponseType ExchangeResponseType = "order_book"
	OrderExchangeResponseType     ExchangeResponseType = "order"
	PongExchangeResponseType      ExchangeResponseType = "pong"
	CandleExchangeResponseType    ExchangeResponseType = "candle"
)

type TradingNotifyRequest struct {
	Type        ExchangeRequestType `json:"type"`
	ProductIds  []string            `json:"product_ids,omitempty"`
	CurrencyIDs []string            `json:"currency_ids,omitempty"`
	Channels    []string            `json:"channels"`
	Token       string              `json:"token"` // TODO: what this?
}

type TradingNotifyAsset struct {
	*UserAsset
	CurrencyName string
}

type TradingNotifyResponse struct {
	Type      ExchangeResponseType `json:"type"`
	ProductID string               `json:"product_id,omitempty"`

	UserAsset        *TradingNotifyAsset `json:"user_asset,omitempty"`
	Tick             *TickEntity         `json:"tick,omitempty"`
	MatchOrderDetail *MatchOrderDetail   `json:"match_order_detail,omitempty"`
	OrderBook        *OrderBookL2Entity  `json:"order_book,omitempty"`
	Order            *OrderEntity        `json:"order,omitempty"`
	CandleBar        *CandleBar          `json:"candle,omitempty"`
}

type TradingNotifyStream interface {
	Send(out TradingNotifyResponse) error
	Recv() (TradingNotifyRequest, error)
}

type TradingUseCase interface {
	ConsumeGlobalSequencer(context.Context)
	ConsumeTradingEvent(ctx context.Context, key string)
	ProduceCreateOrderTradingEvent(ctx context.Context, userID int, direction DirectionEnum, price, quantity decimal.Decimal) (*TradingEvent, error)
	ProduceCancelOrderTradingEvent(ctx context.Context, userID, orderID int) (*TradingEvent, error)
	ProduceDepositOrderTradingEvent(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TradingEvent, error)
	EnableBackupSnapshot(ctx context.Context, duration time.Duration)

	GetHistoryMatchDetails(maxResults int) ([]*MatchOrderDetail, error)
	GetUserHistoryMatchDetails(userID, orderID int) ([]*MatchOrderDetail, error)

	GetLatestSnapshot(context.Context) (*TradingSnapshot, error)
	SaveSnapshot(context.Context, *TradingSnapshot) error
	GetHistorySnapshot(context.Context) (*TradingSnapshot, error)
	RecoverBySnapshot(*TradingSnapshot) error

	NotifyForPublic(ctx context.Context, stream TradingNotifyStream) error
	NotifyForUser(ctx context.Context, userID int, stream TradingNotifyStream) error

	Done() <-chan struct{}
	Err() error
	Shutdown() error
}
