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
	ErrGetDuplicateEvent    = errors.New("get duplicate event error")
	ErrMissEvent            = errors.New("miss event error")
	ErrPreviousIDNotCorrect = errors.New("message previous id not correct")
)

type MatchingRepo interface {
	ProduceOrderBook(ctx context.Context, orderBook *OrderBookEntity) error
	ConsumeOrderBook(ctx context.Context, key string, notify func(*OrderBookEntity) error)

	ProduceMatchOrderMQByMatchResult(ctx context.Context, matchResult *MatchResult) error
	ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*MatchOrderDetail) error)

	SaveMatchingDetailsWithIgnore(context.Context, []*MatchOrderDetail) error
	GetMatchingDetails(orderID int) ([]*MatchOrderDetail, error)
	GetMatchingHistory(maxResults int) ([]*MatchOrderDetail, error)
}

type MatchingUseCase interface {
	NewOrder(ctx context.Context, o *OrderEntity) (*MatchResult, error)
	CancelOrder(ts time.Time, o *OrderEntity) error
	GetOrderBook(maxDepth int) *OrderBookEntity

	GetMatchesData() (*MatchData, error)
	RecoverBySnapshot(*TradingSnapshot) error

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
