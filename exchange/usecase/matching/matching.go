package matching

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type matchResult struct {
	domain.MatchResult
}

func createMatchResult(takerOrder *order) *matchResult {
	return &matchResult{domain.MatchResult{
		SequenceID: takerOrder.SequenceID,
		TakerOrder: takerOrder.OrderEntity,
		CreatedAt:  takerOrder.CreatedAt,
	}}
}

func (m *matchResult) add(price decimal.Decimal, matchedQuantity decimal.Decimal, makerOrder *domain.OrderEntity) {
	m.MatchDetails = append(m.MatchDetails, &domain.MatchDetail{
		Price:      price,
		Quantity:   matchedQuantity,
		TakerOrder: m.TakerOrder,
		MakerOrder: makerOrder,
	})
}

type matchingUseCase struct {
	matchingRepo       domain.MatchingRepo
	quotationRepo      domain.QuotationRepo
	candleRepo         domain.CandleRepo
	isOrderBookChanged atomic.Bool

	buyBook           *orderBook
	sellBook          *orderBook
	marketPrice       decimal.Decimal
	sequenceID        int // TODO: use long
	orderBookMaxDepth int
}

func CreateMatchingUseCase(ctx context.Context, matchingRepo domain.MatchingRepo, quotationRepo domain.QuotationRepo, candleRepo domain.CandleRepo, orderBookMaxDepth int) domain.MatchingUseCase {
	m := &matchingUseCase{
		quotationRepo:     quotationRepo,
		matchingRepo:      matchingRepo,
		candleRepo:        candleRepo,
		buyBook:           createOrderBook(domain.DirectionBuy),
		sellBook:          createOrderBook(domain.DirectionSell),
		marketPrice:       decimal.Zero, // TODO: check correct?
		orderBookMaxDepth: orderBookMaxDepth,
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

func (m *matchingUseCase) NewOrder(ctx context.Context, o *domain.OrderEntity) (*domain.MatchResult, error) {
	switch o.Direction {
	case domain.DirectionBuy:
		matchResult, err := m.processOrder(ctx, &order{o}, m.sellBook, m.buyBook)
		if err != nil {
			return nil, errors.Wrap(err, "process order failed")
		}
		return &matchResult.MatchResult, nil
	case domain.DirectionSell:
		matchResult, err := m.processOrder(ctx, &order{o}, m.buyBook, m.sellBook)
		if err != nil {
			return nil, errors.Wrap(err, "process order failed")
		}
		return &matchResult.MatchResult, nil
	default:
		return nil, errors.New("unknown direction")
	}
}

func (m *matchingUseCase) CancelOrder(ts time.Time, o *domain.OrderEntity) error {
	orderInstance := &order{o}
	book := m.buyBook
	if o.Direction == domain.DirectionSell {
		book = m.sellBook
	}
	if !book.remove(orderInstance) {
		return domain.ErrNoOrder
	}
	status := domain.OrderStatusFullyCanceled
	if !(o.UnfilledQuantity.Cmp(o.Quantity) == 0) {
		status = domain.OrderStatusPartialCanceled
	}
	orderInstance.updateOrder(o.UnfilledQuantity, status, ts)

	m.isOrderBookChanged.Store(true)

	return nil
}

func (m *matchingUseCase) GetOrderBook(maxDepth int) *domain.OrderBookEntity {
	return &domain.OrderBookEntity{
		SequenceID: m.sequenceID,
		Price:      m.marketPrice,
		Sell:       m.sellBook.getOrderBook(maxDepth),
		Buy:        m.buyBook.getOrderBook(maxDepth),
	}
}

func (m *matchingUseCase) GetMatchesData() (*domain.MatchData, error) {
	cloneBuyBooksID := m.buyBook.getOrderBooksID()
	cloneSellBooksID := m.sellBook.getOrderBooksID()
	return &domain.MatchData{
		Buy:         cloneBuyBooksID,
		Sell:        cloneSellBooksID,
		MarketPrice: m.marketPrice,
	}, nil
}

func (m *matchingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	orderMap := make(map[int]*domain.OrderEntity)
	for _, order := range tradingSnapshot.Orders {
		orderMap[order.ID] = order
	}
	for _, orderID := range tradingSnapshot.MatchData.Buy {
		m.buyBook.add(&order{OrderEntity: orderMap[orderID]})
	}
	for _, orderID := range tradingSnapshot.MatchData.Sell {
		m.sellBook.add(&order{OrderEntity: orderMap[orderID]})
	}
	m.sequenceID = tradingSnapshot.SequenceID
	m.marketPrice = tradingSnapshot.MatchData.MarketPrice
	return nil
}

func (m *matchingUseCase) processOrder(ctx context.Context, takerOrder *order, markerBook, anotherBook *orderBook) (*matchResult, error) {
	m.sequenceID = takerOrder.SequenceID
	ts := takerOrder.CreatedAt // TODO: name?
	matchResult := createMatchResult(takerOrder)

	takerUnfilledQuantity := takerOrder.Quantity
	for {
		makerOrder, err := markerBook.getFirst()
		if errors.Is(err, domain.ErrEmptyOrderBook) {
			break
		} else if err != nil {
			break // TODO: check correct?
		}
		if takerOrder.Direction == domain.DirectionBuy && takerOrder.Price.Cmp(makerOrder.Price) < 0 {
			break
		} else if takerOrder.Direction == domain.DirectionSell && takerOrder.Price.Cmp(makerOrder.Price) > 0 {
			break
		}
		m.marketPrice = makerOrder.Price
		matchedQuantity := min(takerUnfilledQuantity, makerOrder.UnfilledQuantity) // TODO: think logic // TODO: test quantity or unfilledQuantity
		matchResult.add(makerOrder.Price, matchedQuantity, makerOrder.OrderEntity)
		takerUnfilledQuantity = takerUnfilledQuantity.Sub(matchedQuantity)
		makerUnfilledQuantity := makerOrder.UnfilledQuantity.Sub(matchedQuantity)
		if makerUnfilledQuantity.Equal(decimal.Zero) { // TODO: check correct
			makerOrder.updateOrder(makerUnfilledQuantity, domain.OrderStatusFullyFilled, ts)
			markerBook.remove(makerOrder)
		} else {
			makerOrder.updateOrder(makerUnfilledQuantity, domain.OrderStatusPartialFilled, ts)
		}
		if takerUnfilledQuantity.Equal(decimal.Zero) { // TODO: check correct
			takerOrder.updateOrder(takerUnfilledQuantity, domain.OrderStatusFullyFilled, ts)
			break
		}
	}
	if takerUnfilledQuantity.GreaterThan(decimal.Zero) { // TODO: check correct
		orderStatus := domain.OrderStatusPending
		if takerUnfilledQuantity.Cmp(takerOrder.Quantity) != 0 {
			orderStatus = domain.OrderStatusPartialFilled
		}
		takerOrder.updateOrder(takerUnfilledQuantity, orderStatus, ts)
		anotherBook.add(takerOrder)
	}

	m.isOrderBookChanged.Store(true)

	return matchResult, nil
}

func (m *matchingUseCase) ConsumeMatchResultToSave(ctx context.Context, key string) {
	m.matchingRepo.ConsumeMatchOrderMQBatch(ctx, key, func(matchOrderDetails []*domain.MatchOrderDetail) error { // TODO: error handle
		if err := m.matchingRepo.SaveMatchingDetailsWithIgnore(ctx, matchOrderDetails); err != nil {
			return errors.Wrap(err, "save matching details failed")
		}
		return nil
	})
}

func min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}
