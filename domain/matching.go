package domain

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrEmptyOrderBook       = errors.New("empty order book error")
	ErrNoOrder              = errors.New("order not found error")
	ErrNoPrice              = errors.New("price not found error")
	ErrGetDuplicateEvent    = errors.New("get duplicate event error")
	ErrMissEvent            = errors.New("miss event error")
	ErrPreviousIDNotCorrect = errors.New("message previous id not correct")
)

type MatchingRepo interface {
	ProduceMatchOrderMQByTradingResults(ctx context.Context, tradingResults []*TradingResult) error
	ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*MatchOrderDetail) error)
	ConsumeMatchOrderMQBatchWithCommit(ctx context.Context, key string, notify func(matchOrderDetails []*MatchOrderDetail, commitFn func() error) error)

	SaveMatchingDetailsWithIgnore(context.Context, []*MatchOrderDetail) error
	GetMatchingDetails(orderID int) ([]*MatchOrderDetail, error)
	GetMatchingHistory(maxResults int) ([]*MatchOrderDetail, error)
}

type MatchingNotifyRepo interface {
	ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*MatchOrderDetail) error)
	StopConsume(ctx context.Context, key string)
}

type MatchingOrderBookRepo interface {
	GetOrderBookFirst(direction DirectionEnum) (*OrderEntity, error)
	AddOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	RemoveOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	MatchOrder(orderID int, matchedQuantity decimal.Decimal, orderStatus OrderStatusEnum, updatedAt time.Time) error
	UpdateOrderStatus(orderID int, orderStatus OrderStatusEnum, updatedAt time.Time) error

	GetL1OrderBook() *OrderBookL1Entity
	GetL2OrderBook() *OrderBookL2Entity
	GetL3OrderBook() *OrderBookL3Entity

	GetMarketPrice() decimal.Decimal
	SetMarketPrice(marketPrice decimal.Decimal)
	GetSequenceID() int
	SetSequenceID(sequenceID int)

	SaveHistoryL1OrderBookByL3OrderBook(context.Context, *OrderBookL3Entity) (*OrderBookL1Entity, error)
	SaveHistoryL2OrderBookByL3OrderBook(context.Context, *OrderBookL3Entity) (*OrderBookL2Entity, error)
	SaveHistoryL3OrderBook(context.Context, *OrderBookL3Entity) error

	GetHistoryL1OrderBook(ctx context.Context) (*OrderBookL1Entity, error)
	// GetHistoryL2OrderBook maxDepth is -1 will return all order book
	GetHistoryL2OrderBook(ctx context.Context, maxDepth int) (*OrderBookL2Entity, error)
	// GetHistoryL3OrderBook maxDepth is -1 will return all order book
	GetHistoryL3OrderBook(ctx context.Context, maxDepth int) (*OrderBookL3Entity, error)

	ProduceOrderBook(ctx context.Context, orderBook *OrderBookL3Entity) error
	ConsumeOrderBookWithCommit(ctx context.Context, key string, notify func(orderBook *OrderBookL3Entity, commitFn func() error) error)

	ProduceL1OrderBook(ctx context.Context, orderBook *OrderBookL1Entity) error
	ConsumeL1OrderBook(ctx context.Context, key string, notify func(*OrderBookL1Entity) error)

	ProduceL2OrderBook(ctx context.Context, orderBook *OrderBookL2Entity) error
	ConsumeL2OrderBook(ctx context.Context, key string, notify func(*OrderBookL2Entity) error)

	ProduceL3OrderBook(ctx context.Context, orderBook *OrderBookL3Entity) error
	ConsumeL3OrderBook(ctx context.Context, key string, notify func(*OrderBookL3Entity) error)
}

type MatchingOrderBookNotifyRepo interface {
	ConsumeL2OrderBookWithRingBuffer(ctx context.Context, key string, notify func(*OrderBookL2Entity) error)
	StopConsume(ctx context.Context, key string)
}

type MatchingUseCase interface {
	NewOrder(ctx context.Context, o *OrderEntity) (*MatchResult, error)
	CancelOrder(o *OrderEntity, timestamp time.Time) (*CancelResult, error)

	GetHistoryL1OrderBook(ctx context.Context) (*OrderBookL1Entity, error)
	// GetHistoryL2OrderBook maxDepth is -1 will return all order book
	GetHistoryL2OrderBook(ctx context.Context, maxDepth int) (*OrderBookL2Entity, error)
	// GetHistoryL3OrderBook maxDepth is -1 will return all order book
	GetHistoryL3OrderBook(ctx context.Context, maxDepth int) (*OrderBookL3Entity, error)
	GetMarketPrice() decimal.Decimal
	GetSequenceID() int

	GetMatchesData() (*MatchData, error)
	RecoverBySnapshot(tradingSnapshot *TradingSnapshot) error

	ConsumeMatchResultToSave(ctx context.Context, key string)
	ConsumeOrderBookToSave(ctx context.Context, key string)
}

type MatchType int

const (
	MatchTypeTaker MatchType = iota + 1
	MatchTypeMaker
)

type MatchData struct {
	Buy         []int
	Sell        []int
	MarketPrice decimal.Decimal
}

type MatchResult struct {
	SequenceID   int
	TakerOrder   *OrderEntity
	MatchDetails []*MatchDetail
	CreatedAt    time.Time
}

type CancelResult struct {
	SequenceID  int
	CancelOrder *OrderEntity
	CreatedAt   time.Time
}

type MatchDetail struct {
	Price      decimal.Decimal
	Quantity   decimal.Decimal
	TakerOrder *OrderEntity
	MakerOrder *OrderEntity
}

type MatchOrderDetail struct {
	ID             int
	SequenceID     int
	OrderID        int
	CounterOrderID int
	UserID         int
	CounterUserID  int

	Type      MatchType
	Direction DirectionEnum
	Price     decimal.Decimal
	Quantity  decimal.Decimal

	CreatedAt time.Time
}
