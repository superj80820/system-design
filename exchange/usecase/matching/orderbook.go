package matching

import (
	"sync"

	"github.com/superj80820/system-design/domain"
	treemapKit "github.com/superj80820/system-design/kit/util/treemap"
)

type orderBook struct {
	direction domain.DirectionEnum
	book      *treemapKit.GenericTreeMap[*orderKey, *order]
	lock      *sync.RWMutex
}

func createOrderBook(direction domain.DirectionEnum) *orderBook {
	return &orderBook{
		direction: direction,
		book:      treemapKit.NewWith[*orderKey, *order](directionEnum(direction).compare), // TODO: think performance
		lock:      new(sync.RWMutex),
	}
}

func (ob *orderBook) getFirst() (*order, error) {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	if ob.book.Empty() {
		return nil, domain.ErrEmptyOrderBook
	} else {
		_, value := ob.book.Min()
		return value, nil
	}
}

func (ob *orderBook) remove(o *order) bool {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	key := &orderKey{sequenceId: o.SequenceID, price: o.Price}
	_, found := ob.book.Get(key) // TODO: about performance
	if !found {
		return false
	}
	ob.book.Remove(key) // TODO: need check? about performance
	return true
}

func (ob *orderBook) add(o *order) bool {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	ob.book.Put(&orderKey{sequenceId: o.SequenceID, price: o.Price}, o)
	return true // TODO: need check? about performance
}

func (ob *orderBook) getOrderBooksID() []int {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	orderBooksID := make([]int, 0, ob.book.Size())
	ob.book.Each(func(key *orderKey, value *order) {
		orderBooksID = append(orderBooksID, value.ID)
	})
	return orderBooksID
}

func (ob *orderBook) getOrderBook(maxDepth int) []*domain.OrderBookItemEntity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	orderBookItems := make([]*domain.OrderBookItemEntity, 0, maxDepth)
	var (
		prevOrderBookItem *domain.OrderBookItemEntity
		isMaxDepth        bool
	)
	ob.book.Each(func(key *orderKey, value *order) {
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
			prevOrderBookItem = &domain.OrderBookItemEntity{
				Price:    value.Price,
				Quantity: value.UnfilledQuantity,
			}
			orderBookItems = append(orderBookItems, prevOrderBookItem)
		}
	})
	return orderBookItems
}
