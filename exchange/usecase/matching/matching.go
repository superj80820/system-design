package matching

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type matchingUseCase struct {
	matchingRepo          domain.MatchingRepo
	matchingOrderBookRepo domain.MatchingOrderBookRepo
	isOrderBookChanged    *atomic.Bool
}

func CreateMatchingUseCase(ctx context.Context, matchingRepo domain.MatchingRepo, matchingOrderBookRepo domain.MatchingOrderBookRepo) domain.MatchingUseCase {
	m := &matchingUseCase{
		matchingRepo:          matchingRepo,
		matchingOrderBookRepo: matchingOrderBookRepo,
		isOrderBookChanged:    new(atomic.Bool),
	}

	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if !m.isOrderBookChanged.Load() {
				continue
			}

			m.matchingOrderBookRepo.ProduceOrderBook(ctx, m.matchingOrderBookRepo.GetL3OrderBook())

			m.isOrderBookChanged.Store(false)
		}
	}()

	return m
}

func (m *matchingUseCase) GetMarketPrice() decimal.Decimal {
	return m.matchingOrderBookRepo.GetMarketPrice()
}

func (m *matchingUseCase) GetSequenceID() int {
	return m.matchingOrderBookRepo.GetSequenceID()
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

	m.matchingOrderBookRepo.SetSequenceID(takerOrder.SequenceID)
	matchResult := createMatchResult(takerOrder)

	for {
		makerOrder, err := m.matchingOrderBookRepo.GetOrderBookFirst(makerDirection)
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
		m.matchingOrderBookRepo.SetMarketPrice(makerOrder.Price)
		matchedQuantity := min(takerOrder.UnfilledQuantity, makerOrder.UnfilledQuantity)
		addForMatchResult(matchResult, makerOrder.Price, matchedQuantity, makerOrder)
		takerOrder.UnfilledQuantity = takerOrder.UnfilledQuantity.Sub(matchedQuantity)
		makerOrder.UnfilledQuantity = makerOrder.UnfilledQuantity.Sub(matchedQuantity)
		if makerOrder.UnfilledQuantity.Equal(decimal.Zero) {
			makerOrder.Status = domain.OrderStatusFullyFilled
			if err := m.matchingOrderBookRepo.MatchOrder(makerOrder.ID, matchedQuantity, domain.OrderStatusFullyFilled, takerOrder.CreatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			if err := m.matchingOrderBookRepo.RemoveOrderBookOrder(makerDirection, makerOrder); err != nil {
				return nil, errors.Wrap(err, "remove order book order failed")
			}
		} else {
			makerOrder.Status = domain.OrderStatusPartialFilled
			if err := m.matchingOrderBookRepo.MatchOrder(makerOrder.ID, matchedQuantity, domain.OrderStatusPartialFilled, takerOrder.CreatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
		}
		if takerOrder.UnfilledQuantity.Equal(decimal.Zero) {
			takerOrder.Status = domain.OrderStatusFullyFilled
			break
		}
	}
	if takerOrder.UnfilledQuantity.GreaterThan(decimal.Zero) {
		status := domain.OrderStatusPending
		if takerOrder.UnfilledQuantity.Cmp(takerOrder.Quantity) != 0 {
			status = domain.OrderStatusPartialFilled
		}
		takerOrder.Status = status
		m.matchingOrderBookRepo.MatchOrder(takerOrder.ID, decimal.Zero, status, takerOrder.CreatedAt)
		m.matchingOrderBookRepo.AddOrderBookOrder(takerDirection, takerOrder)
	}

	m.isOrderBookChanged.Store(true)

	return matchResult, nil
}

func (m *matchingUseCase) CancelOrder(order *domain.OrderEntity, timestamp time.Time) (*domain.CancelResult, error) {
	status := domain.OrderStatusFullyCanceled
	if !(order.UnfilledQuantity.Cmp(order.Quantity) == 0) {
		status = domain.OrderStatusPartialCanceled
	}

	order.Status = status

	m.matchingOrderBookRepo.UpdateOrderStatus(order.ID, status, timestamp)

	if err := m.matchingOrderBookRepo.RemoveOrderBookOrder(order.Direction, order); err != nil {
		return nil, errors.Wrap(err, "remove order failed")
	}

	m.isOrderBookChanged.Store(true)

	return &domain.CancelResult{
		CancelOrder: order,
	}, nil
}

func (m *matchingUseCase) GetMatchesData() (*domain.MatchData, error) {
	orderBook := m.matchingOrderBookRepo.GetL3OrderBook()
	var sellOrderIDs, buyOrderIDs []int
	for _, item := range orderBook.Sell {
		for _, order := range item.Orders {
			sellOrderIDs = append(sellOrderIDs, order.OrderID)
		}
	}
	for _, item := range orderBook.Buy {
		for _, order := range item.Orders {
			buyOrderIDs = append(buyOrderIDs, order.OrderID)
		}
	}

	return &domain.MatchData{
		Buy:         buyOrderIDs,
		Sell:        sellOrderIDs,
		MarketPrice: m.matchingOrderBookRepo.GetMarketPrice(),
	}, nil
}

func (m *matchingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	orderMap := make(map[int]*domain.OrderEntity)
	for _, order := range tradingSnapshot.Orders {
		orderMap[order.ID] = order
	}
	for _, orderID := range tradingSnapshot.MatchData.Buy {
		m.matchingOrderBookRepo.AddOrderBookOrder(domain.DirectionBuy, orderMap[orderID])
	}
	for _, orderID := range tradingSnapshot.MatchData.Sell {
		m.matchingOrderBookRepo.AddOrderBookOrder(domain.DirectionSell, orderMap[orderID])
	}
	m.matchingOrderBookRepo.SetSequenceID(tradingSnapshot.SequenceID)
	m.matchingOrderBookRepo.SetMarketPrice(tradingSnapshot.MatchData.MarketPrice)

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

func (m *matchingUseCase) ConsumeOrderBookToSave(ctx context.Context, key string) {
	m.matchingOrderBookRepo.ConsumeOrderBook(ctx, key, func(l3OrderBook *domain.OrderBookL3Entity) error { // TODO: error handle
		if err := m.matchingOrderBookRepo.SaveHistoryL3OrderBook(ctx, l3OrderBook); err != nil {
			fmt.Println("york1")
			return errors.Wrap(err, "save history l3 order book failed")
		}
		l2OrderBook, err := m.matchingOrderBookRepo.SaveHistoryL2OrderBookByL3OrderBook(ctx, l3OrderBook)
		if err != nil {
			fmt.Println("york2")
			return errors.Wrap(err, "save history l2 order book failed")
		}
		l1OrderBook, err := m.matchingOrderBookRepo.SaveHistoryL1OrderBookByL3OrderBook(ctx, l3OrderBook)
		if err != nil {
			fmt.Println("york3")
			return errors.Wrap(err, "save history l1 order book failed")
		}
		if err := m.matchingOrderBookRepo.ProduceL2OrderBook(ctx, l2OrderBook); err != nil {
			fmt.Println("york4")
			return errors.Wrap(err, "produce l2 order book failed")
		}
		if err := m.matchingOrderBookRepo.ProduceL1OrderBook(ctx, l1OrderBook); err != nil {
			fmt.Println("york5")
			return errors.Wrap(err, "produce l1 order book failed")
		}
		return nil
	})
}

func (m *matchingUseCase) GetHistoryL1OrderBook(ctx context.Context) (*domain.OrderBookL1Entity, error) {
	l1OrderBook, err := m.matchingOrderBookRepo.GetHistoryL1OrderBook(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get history l1 order book failed")
	}
	return l1OrderBook, nil
}

func (m *matchingUseCase) GetHistoryL2OrderBook(ctx context.Context, maxDepth int) (*domain.OrderBookL2Entity, error) { // TODO: maxDepth
	l2OrderBook, err := m.matchingOrderBookRepo.GetHistoryL2OrderBook(ctx, maxDepth)
	if err != nil {
		return nil, errors.Wrap(err, "get history l2 order book failed")
	}
	return l2OrderBook, nil
}

func (m *matchingUseCase) GetHistoryL3OrderBook(ctx context.Context, maxDepth int) (*domain.OrderBookL3Entity, error) { // TODO: maxDepth
	l3OrderBook, err := m.matchingOrderBookRepo.GetHistoryL3OrderBook(ctx, maxDepth)
	if err != nil {
		return nil, errors.Wrap(err, "get history l1 order book failed")
	}
	return l3OrderBook, nil
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

func addForMatchResult(matchResult *domain.MatchResult, price decimal.Decimal, matchedQuantity decimal.Decimal, makerOrder *domain.OrderEntity) { // TODO: test and think
	matchResult.MatchDetails = append(matchResult.MatchDetails, &domain.MatchDetail{
		Price:      price,
		Quantity:   matchedQuantity,
		TakerOrder: matchResult.TakerOrder,
		MakerOrder: makerOrder,
	})
}
