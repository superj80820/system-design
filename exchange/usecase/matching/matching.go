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

func (m *matchResult) add() {} // TODO

type matchingUseCase struct {
	buyBook     *orderBook
	sellBook    *orderBook
	marketPrice decimal.Decimal
	sequenceId  int // TODO: use long
}

func CreateMatchingUseCase() domain.MatchingUseCase {
	return &matchingUseCase{
		buyBook:     CreateOrderBook(domain.DirectionBuy),
		sellBook:    CreateOrderBook(domain.DirectionSell),
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

func (m *matchingUseCase) processOrder(takerOrder *order, markerBook, anotherBook *orderBook) (*matchResult, error) {
	m.sequenceId = takerOrder.SequenceID
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
		matchResult.add()                                                          // TODO
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
