package mysqlandmq

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	treemapKit "github.com/superj80820/system-design/kit/util/treemap"
)

type orderKey struct {
	sequenceId int
	price      decimal.Decimal
}

type orderBook struct {
	direction domain.DirectionEnum
	book      *treemapKit.GenericTreeMap[*orderKey, *domain.OrderEntity]
	lock      *sync.RWMutex
}

func CreateOrderBook(direction domain.DirectionEnum) *orderBook {
	return &orderBook{
		direction: direction,
		book:      treemapKit.NewWith[*orderKey, *domain.OrderEntity](directionEnum(direction).compare), // TODO: think performance
		lock:      new(sync.RWMutex),
	}
}

func (ob *orderBook) getFirst() (*domain.OrderEntity, error) {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	if ob.book.Empty() {
		return nil, domain.ErrEmptyOrderBook
	} else {
		_, value := ob.book.Min()
		return value, nil
	}
}

func (ob *orderBook) remove(order *domain.OrderEntity) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	key := &orderKey{sequenceId: order.SequenceID, price: order.Price}
	_, found := ob.book.Get(key)
	if !found {
		return errors.Wrap(domain.ErrNoOrder, "not found order")
	}
	ob.book.Remove(key)
	return nil
}

func (ob *orderBook) add(order *domain.OrderEntity) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	ob.book.Put(&orderKey{sequenceId: order.SequenceID, price: order.Price}, order)

	return nil
}

func (ob *orderBook) getOrderBooksID() []int {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	orderBooksID := make([]int, 0, ob.book.Size())
	ob.book.Each(func(key *orderKey, value *domain.OrderEntity) {
		orderBooksID = append(orderBooksID, value.ID)
	})
	return orderBooksID
}

func (ob *orderBook) getOrderBook(maxDepth int) []*domain.OrderBookL2ItemEntity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	orderBookItems := make([]*domain.OrderBookL2ItemEntity, 0, maxDepth)
	var (
		prevOrderBookItem *domain.OrderBookL2ItemEntity
		isMaxDepth        bool
	)
	ob.book.Each(func(key *orderKey, value *domain.OrderEntity) {
		if isMaxDepth {
			return
		}

		if prevOrderBookItem != nil && value.Price.Cmp(prevOrderBookItem.Price) == 0 {
			prevOrderBookItem.Quantity = prevOrderBookItem.Quantity.Add(value.UnfilledQuantity)
		} else {
			if len(orderBookItems) >= maxDepth {
				isMaxDepth = true
				return
			}
			prevOrderBookItem = &domain.OrderBookL2ItemEntity{
				Price:    value.Price,
				Quantity: value.UnfilledQuantity,
			}
			orderBookItems = append(orderBookItems, prevOrderBookItem)
		}
	})
	return orderBookItems
}
