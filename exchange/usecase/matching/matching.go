package matching

import (
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type matchResult struct {
	domain.MatchResult
}

func createMatchResult(o *order) *matchResult {
	return &matchResult{domain.MatchResult{
		TakerOrder: o.OrderEntity,
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
	buyBook     *orderBook
	sellBook    *orderBook
	marketPrice decimal.Decimal
	sequenceID  int // TODO: use long
}

func CreateMatchingUseCase() domain.MatchingUseCase {
	return &matchingUseCase{
		buyBook:     createOrderBook(domain.DirectionBuy),
		sellBook:    createOrderBook(domain.DirectionSell),
		marketPrice: decimal.Zero, // TODO: check correct?
	}
}

func (m *matchingUseCase) NewOrder(o *domain.OrderEntity) (*domain.MatchResult, error) {
	switch o.Direction {
	case domain.DirectionBuy:
		matchResult, err := m.processOrder(&order{o}, m.sellBook, m.buyBook)
		if err != nil {
			return nil, errors.Wrap(err, "process order failed")
		}
		return &matchResult.MatchResult, nil
	case domain.DirectionSell:
		matchResult, err := m.processOrder(&order{o}, m.buyBook, m.sellBook)
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

func (m *matchingUseCase) processOrder(takerOrder *order, markerBook, anotherBook *orderBook) (*matchResult, error) {
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
	return matchResult, nil
}

func min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

// package usecase

// import "github.com/superj80820/system-design/domain"

// // class Book<Side> {
// //     private Side side;
// //     private Map<Price, PriceLevel> limitMap;
// // }

// type PriceLevel struct {
// 	// private Price limitPrice; // TODO: check?
// 	// private long totalVolume; // TODO: check?
// 	orders []*Order
// }

// type Order struct {
// 	price           int
// 	quantity        int
// 	matchedQuantity int // TODO: check ?
// }

// type bookSide int

// const (
// 	bookBuySide bookSide = iota + 1
// 	bookSellSide
// )

// type Book struct {
// 	side     bookSide
// 	limitMap map[int]*PriceLevel
// }

// type OrderBook struct {
// 	buyBook  *book
// 	sellBook *book
// 	//  bestBid
// 	//  bestOffer
// 	//  orderMap
// }

// type matchingUseCase struct{}

// func CreateOrderUseCase() domain.MatchingUseCase {
// 	return &matchingUseCase{}
// }

// func (m *matchingUseCase) NewOrder() {
// 	if BUY.equals(order.side) {
// 		return match(orderBook.sellBook, order)
// 	} else {
// 		return match(orderBook.buyBook, order)
// 	}
// }

// func (m *matchingUseCase) CancelOrder() {
// 	panic("unimplemented")
// }

// func match(book Book, order Order) {
// 	leavesQuantity := order.quantity - order.matchedQuantity
// 	limitIter := book.limitMap[order.price].orders
// 	for _, val := range limitIter {
// 		if leavesQuantity <= 0 {
// 			break
// 		}
// 		matched := min(val.quantity, order.quantity)
// 		order.matchedQuantity += matched
// 		leavesQuantity = order.quantity - order.matchedQuantity
// 		val.quantity -= matched // TODO: check?
// 		// generateMatchedFill() // TODO: check?
// 	}
// 	// while (limitIter.hasNext() && leavesQuantity > 0) {
// 	//     Quantity matched = min(limitIter.next.quantity, order.quantity);
// 	//     order.matchedQuantity += matched;
// 	//     leavesQuantity = order.quantity - order.matchedQuantity;
// 	//     remove(limitIter.next);
// 	//     generateMatchedFill();
// 	// }
// 	// return SUCCESS(MATCH_SUCCESS, order);
// }

// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
