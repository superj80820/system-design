package matching

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type matchingUseCase struct {
	matchingRepo       domain.MatchingRepo
	isOrderBookChanged *atomic.Bool

	orderBookMaxDepth int
}

func CreateMatchingUseCase(ctx context.Context, matchingRepo domain.MatchingRepo, orderBookMaxDepth int) domain.MatchingUseCase {
	m := &matchingUseCase{
		matchingRepo:       matchingRepo,
		orderBookMaxDepth:  orderBookMaxDepth,
		isOrderBookChanged: new(atomic.Bool),
	}

	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if !m.isOrderBookChanged.Load() {
				continue
			}

			m.matchingRepo.ProduceOrderBook(ctx, m.GetOrderBook(m.orderBookMaxDepth))

			m.isOrderBookChanged.Store(false)
		}
	}()

	return m
}

func (m *matchingUseCase) GetMarketPrice() decimal.Decimal {
	return m.matchingRepo.GetMarketPrice()
}

func (m *matchingUseCase) GetSequenceID() int {
	return m.matchingRepo.GetSequenceID()
}

func (m *matchingUseCase) NewOrder(ctx context.Context, takerOrder *domain.OrderEntity) (*domain.MatchResult, error) {
	var makerDirection, takerDirection domain.DirectionEnum
	switch takerOrder.Direction {
	case domain.DirectionSell:
		makerDirection = domain.DirectionBuy
		takerDirection = domain.DirectionSell
	case domain.DirectionBuy:
		makerDirection = domain.DirectionSell
		takerDirection = domain.DirectionBuy
	default:
		return nil, errors.New("not define direction")
	}

	m.matchingRepo.SetSequenceID(takerOrder.SequenceID)
	matchResult := createMatchResult(takerOrder)

	takerUnfilledQuantity := takerOrder.Quantity
	for {
		makerOrder, err := m.matchingRepo.GetOrderBookFirst(makerDirection)
		if errors.Is(err, domain.ErrEmptyOrderBook) {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "get first order book order failed")
		}
		if takerOrder.Direction == domain.DirectionBuy && takerOrder.Price.Cmp(makerOrder.Price) < 0 {
			break
		} else if takerOrder.Direction == domain.DirectionSell && takerOrder.Price.Cmp(makerOrder.Price) > 0 {
			break
		}
		m.matchingRepo.SetMarketPrice(makerOrder.Price)
		matchedQuantity := min(takerUnfilledQuantity, makerOrder.UnfilledQuantity)
		addForMatchResult(matchResult, makerOrder.Price, matchedQuantity, makerOrder)
		takerUnfilledQuantity = takerUnfilledQuantity.Sub(matchedQuantity)
		makerUnfilledQuantity := makerOrder.UnfilledQuantity.Sub(matchedQuantity)
		if makerUnfilledQuantity.Equal(decimal.Zero) {
			updateOrder(makerOrder, makerUnfilledQuantity, domain.OrderStatusFullyFilled, takerOrder.CreatedAt)
			m.matchingRepo.RemoveOrderBookOrder(makerDirection, makerOrder)
		} else {
			updateOrder(makerOrder, makerUnfilledQuantity, domain.OrderStatusPartialFilled, takerOrder.CreatedAt)
		}
		if takerUnfilledQuantity.Equal(decimal.Zero) {
			updateOrder(takerOrder, takerUnfilledQuantity, domain.OrderStatusFullyFilled, takerOrder.CreatedAt)
			break
		}
	}
	if takerUnfilledQuantity.GreaterThan(decimal.Zero) {
		orderStatus := domain.OrderStatusPending
		if takerUnfilledQuantity.Cmp(takerOrder.Quantity) != 0 {
			orderStatus = domain.OrderStatusPartialFilled
		}
		updateOrder(takerOrder, takerUnfilledQuantity, orderStatus, takerOrder.CreatedAt)
		m.matchingRepo.AddOrderBookOrder(takerDirection, takerOrder)
	}

	m.isOrderBookChanged.Store(true)

	return matchResult, nil
}

func (m *matchingUseCase) CancelOrder(timestamp time.Time, order *domain.OrderEntity) error {
	if err := m.matchingRepo.RemoveOrderBookOrder(order.Direction, order); err != nil {
		return errors.Wrap(err, "remove order failed")
	}

	status := domain.OrderStatusFullyCanceled
	if !(order.UnfilledQuantity.Cmp(order.Quantity) == 0) {
		status = domain.OrderStatusPartialCanceled
	}

	updateOrder(order, order.UnfilledQuantity, status, timestamp)

	m.isOrderBookChanged.Store(true)

	return nil
}

func (m *matchingUseCase) GetOrderBook(maxDepth int) *domain.OrderBookEntity {
	return m.matchingRepo.GetOrderBook(maxDepth)
}

func (m *matchingUseCase) GetMatchesData() (*domain.MatchData, error) {
	sellBook, buyBook := m.matchingRepo.GetOrderBooksID()
	return &domain.MatchData{
		Buy:         sellBook,
		Sell:        buyBook,
		MarketPrice: m.matchingRepo.GetMarketPrice(),
	}, nil
}

func (m *matchingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	if err := m.matchingRepo.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot")
	}
	return nil
}

func (m *matchingUseCase) ConsumeMatchResultToSave(ctx context.Context, key string) {
	m.matchingRepo.ConsumeMatchOrderMQBatch(ctx, key, func(matchOrderDetails []*domain.MatchOrderDetail) error { // TODO: error handle
		if err := m.matchingRepo.SaveMatchingDetailsWithIgnore(ctx, matchOrderDetails); err != nil {
			return errors.Wrap(err, "save matching details failed")
		}
		return nil
	})
}

func createMatchResult(takerOrder *domain.OrderEntity) *domain.MatchResult {
	return &domain.MatchResult{
		SequenceID: takerOrder.SequenceID,
		TakerOrder: takerOrder,
		CreatedAt:  takerOrder.CreatedAt,
	}
}

func min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

func addForMatchResult(matchResult *domain.MatchResult, price decimal.Decimal, matchedQuantity decimal.Decimal, makerOrder *domain.OrderEntity) {
	matchResult.MatchDetails = append(matchResult.MatchDetails, &domain.MatchDetail{
		Price:      price,
		Quantity:   matchedQuantity,
		TakerOrder: matchResult.TakerOrder,
		MakerOrder: makerOrder,
	})
}

func updateOrder(order *domain.OrderEntity, unfilledQuantity decimal.Decimal, orderStatus domain.OrderStatusEnum, updatedAt time.Time) {
	order.UnfilledQuantity = unfilledQuantity
	order.Status = orderStatus
	order.UpdatedAt = updatedAt
}
