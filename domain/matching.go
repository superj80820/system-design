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
	ProduceOrderBook(ctx context.Context, orderBook *OrderBookL2Entity) error
	ConsumeOrderBook(ctx context.Context, key string, notify func(*OrderBookL2Entity) error)

	ProduceMatchOrderMQByTradingResult(ctx context.Context, tradingResult *TradingResult) error
	ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*MatchOrderDetail) error)

	SaveMatchingDetailsWithIgnore(context.Context, []*MatchOrderDetail) error
	GetMatchingDetails(orderID int) ([]*MatchOrderDetail, error)
	GetMatchingHistory(maxResults int) ([]*MatchOrderDetail, error)
}

type MatchingOrderBookRepo interface {
	GetOrderBookFirst(direction DirectionEnum) (*OrderEntity, error)
	AddOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	RemoveOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	MatchOrder(orderID int, matchedQuantity decimal.Decimal, orderStatus OrderStatusEnum, updatedAt time.Time) error
	UpdateOrderStatus(orderID int, orderStatus OrderStatusEnum, updatedAt time.Time) error

	GetL1OrderBook() *OrderBookL1Entity
	// GetL2OrderBook maxDepth is -1 will return all order book
	GetL2OrderBook(maxDepth int) *OrderBookL2Entity
	// GetL3OrderBook maxDepth is -1 will return all order book
	GetL3OrderBook(maxDepth int) *OrderBookL3Entity

	GetMarketPrice() decimal.Decimal
	SetMarketPrice(marketPrice decimal.Decimal)
	GetSequenceID() int
	SetSequenceID(sequenceID int)
}

type MatchingUseCase interface {
	NewOrder(ctx context.Context, o *OrderEntity) (*MatchResult, error)
	CancelOrder(o *OrderEntity, timestamp time.Time) (*CancelResult, error)

	GetOrderBook(maxDepth int) *OrderBookL2Entity
	GetL3OrderBook(maxDepth int) *OrderBookL3Entity
	GetMarketPrice() decimal.Decimal
	GetSequenceID() int

	GetMatchesData() (*MatchData, error)
	RecoverBySnapshot(tradingSnapshot *TradingSnapshot) error

	ConsumeMatchResultToSave(ctx context.Context, key string)
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
