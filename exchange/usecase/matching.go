package usecase

import (
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	treemapKit "github.com/superj80820/system-design/kit/util/treemap"
)

type directionEnum domain.DirectionEnum

func (d directionEnum) compare(a, b *orderKey) int {
	switch domain.DirectionEnum(d) { //TODO: think performance
	case domain.DirectionBuy:
		cmp := b.price.Cmp(a.price)
		if cmp == 0 {
			if a.sequenceId > b.sequenceId {
				return 1
			} else if a.sequenceId < b.sequenceId {
				return -1
			} else {
				return 0
			}
		}
		return cmp
	case domain.DirectionSell:
		cmp := a.price.Cmp(b.price)
		if cmp == 0 {
			if a.sequenceId > b.sequenceId {
				return 1
			} else if a.sequenceId < b.sequenceId {
				return -1
			} else {
				return 0
			}
		}
		return cmp
	case domain.DirectionUnknown:
		panic("unknown direction")
	default:
		panic("unknown direction")
	}
}

type orderKey struct {
	sequenceId int // TODO: use long
	price      decimal.Decimal
}

type order struct {
	*domain.Order
}

func createOrder(sequenceId int, price decimal.Decimal, direction domain.DirectionEnum, quantity decimal.Decimal) *order {
	return &order{
		&domain.Order{
			SequenceId:       sequenceId,
			Price:            price,
			Direction:        direction,
			Quantity:         quantity,
			UnfilledQuantity: quantity,
			Status:           domain.OrderStatusPending,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		},
	}
}

func (o *order) updateOrder(unfilledQuantity decimal.Decimal, orderStatus domain.OrderStatusEnum, updatedAt time.Time) {
	o.UnfilledQuantity = unfilledQuantity
	o.Status = orderStatus
	o.UpdatedAt = updatedAt
}

type orderBook struct {
	direction domain.DirectionEnum
	book      *treemapKit.GenericTreeMap[*orderKey, *order]
}

func CreateOrderBook(direction domain.DirectionEnum) *orderBook {
	return &orderBook{
		direction: direction,
		book:      treemapKit.NewWith[*orderKey, *order](directionEnum(direction).compare), // TODO: think performance
	}
}

func (ob *orderBook) getFirst() (*order, error) {
	if ob.book.Empty() {
		return nil, domain.ErrNoOrder
	} else {
		_, value := ob.book.Min()
		return value, nil
	}
}

func (ob *orderBook) remove(o *order) bool {
	ob.book.Remove(&orderKey{sequenceId: o.SequenceId, price: o.Price})
	return true // TODO: need check? about performance
}

func (ob *orderBook) add(o *order) bool {
	ob.book.Put(&orderKey{sequenceId: o.SequenceId, price: o.Price}, o)
	return true // TODO: need check? about performance
}

type matchResult struct {
	domain.MatchResult
}

func createMatchResult(o *order) *matchResult {
	return &matchResult{domain.MatchResult{
		TakerOrder: o.Order,
	}}
}

func (m *matchResult) add() {} // TODO

type matchEngine struct {
	buyBook     *orderBook
	sellBook    *orderBook
	marketPrice decimal.Decimal
	sequenceId  int // TODO: use long
}

func createMatchEngine() *matchEngine {
	return &matchEngine{
		buyBook:     CreateOrderBook(domain.DirectionBuy),
		sellBook:    CreateOrderBook(domain.DirectionSell),
		marketPrice: decimal.Zero, // TODO: check correct?
	}
}

func (m *matchEngine) ProcessOrder(o *order) (*matchResult, error) {
	switch o.Direction {
	case domain.DirectionBuy:
		return m.processOrder(o, m.sellBook, m.buyBook)
	case domain.DirectionSell:
		return m.processOrder(o, m.buyBook, m.sellBook)
	default:
		return nil, errors.New("unknown direction")
	}
}

func (m *matchEngine) processOrder(takerOrder *order, markerBook, anotherBook *orderBook) (*matchResult, error) {
	m.sequenceId = takerOrder.SequenceId
	ts := takerOrder.CreatedAt // TODO: name?
	matchResult := createMatchResult(takerOrder)
	takerUnfilledQuantity := takerOrder.Quantity
	for {
		makerOrder, err := markerBook.getFirst()
		if errors.Is(err, domain.ErrNoOrder) {
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

type matchingUseCase struct {
	matchEngine *matchEngine
}

func CreateOrderUseCase() domain.MatchingUseCase {
	return &matchingUseCase{
		matchEngine: createMatchEngine(),
	}
}

// CancelOrder implements domain.MatchingUseCase.
func (*matchingUseCase) CancelOrder() {
	panic("unimplemented")
}

func (m *matchingUseCase) NewOrder(o *domain.Order) (*domain.MatchResult, error) {
	matchResult, err := m.matchEngine.ProcessOrder(&order{o})
	if err != nil {
		return nil, errors.Wrap(err, "process order failed")
	}
	return &matchResult.MatchResult, nil
}
