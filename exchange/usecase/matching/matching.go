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
	// 如果taker是賣單，maker對手盤的為買單簿
	// 如果taker是買單，maker對手盤的為賣單簿
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

	// 設置此次撮合的sequence id
	m.matchingOrderBookRepo.SetSequenceID(takerOrder.SequenceID)
	// 紀錄此次撮合的成交的result
	matchResult := createMatchResult(takerOrder)

	// taker不斷與對手盤的最佳價格撮合直到無法撮合
	for {
		// 取得對手盤的最佳價格
		makerOrder, err := m.matchingOrderBookRepo.GetOrderBookFirst(makerDirection)
		// 如果沒有最佳價格則退出撮合
		if errors.Is(err, domain.ErrEmptyOrderBook) {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "get first order book order failed")
		}

		// 如果賣單低於對手盤最佳價格則退出撮合
		if takerOrder.Direction == domain.DirectionSell && takerOrder.Price.Cmp(makerOrder.Price) > 0 {
			break
			// 如果買單高於對手盤最佳價格則退出撮合
		} else if takerOrder.Direction == domain.DirectionBuy && takerOrder.Price.Cmp(makerOrder.Price) < 0 {
			break
		}

		// 撮合成功，設置撮合價格為對手盤的最佳價格
		m.matchingOrderBookRepo.SetMarketPrice(makerOrder.Price)
		// 撮合數量為taker或maker兩者的最小數量
		matchedQuantity := min(takerOrder.UnfilledQuantity, makerOrder.UnfilledQuantity)
		// 新增撮合紀錄
		addForMatchResult(matchResult, makerOrder.Price, matchedQuantity, makerOrder)
		// taker的數量減去撮合數量
		takerOrder.UnfilledQuantity = takerOrder.UnfilledQuantity.Sub(matchedQuantity)
		// maker的數量減去撮合數量
		makerOrder.UnfilledQuantity = makerOrder.UnfilledQuantity.Sub(matchedQuantity)
		// 如果maker數量減至0，代表已完全成交(Fully Filled)，更新maker order並從order-book移除
		if makerOrder.UnfilledQuantity.Equal(decimal.Zero) {
			makerOrder.Status = domain.OrderStatusFullyFilled
			if err := m.matchingOrderBookRepo.MatchOrder(makerOrder.ID, matchedQuantity, domain.OrderStatusFullyFilled, takerOrder.CreatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			if err := m.matchingOrderBookRepo.RemoveOrderBookOrder(makerDirection, makerOrder); err != nil {
				return nil, errors.Wrap(err, "remove order book order failed")
			}
			// 如果maker數量不為零，代表已部分成交(Partial Filled)，更新maker order
		} else {
			makerOrder.Status = domain.OrderStatusPartialFilled
			if err := m.matchingOrderBookRepo.MatchOrder(makerOrder.ID, matchedQuantity, domain.OrderStatusPartialFilled, takerOrder.CreatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
		}
		// 如果taker數量減至0，則完全成交，退出撮合
		if takerOrder.UnfilledQuantity.Equal(decimal.Zero) {
			takerOrder.Status = domain.OrderStatusFullyFilled
			break
		}
	}
	// 如果taker數量不為0，檢查原始數量與剩餘數量來設置status
	if takerOrder.UnfilledQuantity.GreaterThan(decimal.Zero) {
		// 預設status為等待成交(Pending)
		status := domain.OrderStatusPending
		// 如果原始數量與剩餘數量不等，status為部分成交(Partial Filled)
		if takerOrder.UnfilledQuantity.Cmp(takerOrder.Quantity) != 0 {
			status = domain.OrderStatusPartialFilled
		}
		takerOrder.Status = status
		// 新增order至taker方向的order book
		m.matchingOrderBookRepo.AddOrderBookOrder(takerDirection, takerOrder)
	}

	// 設置order-book為已改變
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
	m.matchingRepo.ConsumeMatchOrderMQBatchWithCommit(ctx, key, func(matchOrderDetails []*domain.MatchOrderDetail, commitFn func() error) error { // TODO: error handle
		if err := m.matchingRepo.SaveMatchingDetailsWithIgnore(ctx, matchOrderDetails); err != nil {
			return errors.Wrap(err, "save matching details failed")
		}
		if err := commitFn(); err != nil {
			return errors.Wrap(err, "commit failed")
		}
		return nil
	})
}

func (m *matchingUseCase) ConsumeOrderBookToSave(ctx context.Context, key string) {
	m.matchingOrderBookRepo.ConsumeOrderBookWithCommit(ctx, key, func(l3OrderBook *domain.OrderBookL3Entity, commitFn func() error) error { // TODO: error handle
		if err := m.matchingOrderBookRepo.SaveHistoryL3OrderBook(ctx, l3OrderBook); err != nil {
			return errors.Wrap(err, "save history l3 order book failed")
		}
		l2OrderBook, err := m.matchingOrderBookRepo.SaveHistoryL2OrderBookByL3OrderBook(ctx, l3OrderBook)
		if err != nil {
			return errors.Wrap(err, "save history l2 order book failed")
		}
		l1OrderBook, err := m.matchingOrderBookRepo.SaveHistoryL1OrderBookByL3OrderBook(ctx, l3OrderBook)
		if err != nil {
			return errors.Wrap(err, "save history l1 order book failed")
		}
		if err := m.matchingOrderBookRepo.ProduceL2OrderBook(ctx, l2OrderBook); err != nil {
			return errors.Wrap(err, "produce l2 order book failed")
		}
		if err := m.matchingOrderBookRepo.ProduceL1OrderBook(ctx, l1OrderBook); err != nil {
			return errors.Wrap(err, "produce l1 order book failed")
		}
		if err := commitFn(); err != nil {
			return errors.Wrap(err, "commit failed")
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
