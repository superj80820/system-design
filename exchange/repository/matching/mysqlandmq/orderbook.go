package mysqlandmq

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
)

type orderBookRepo struct {
	sequenceID    int
	sellBook      *bookStruct
	buyBook       *bookStruct
	orderMap      map[int]*list.Element
	priceLevelMap map[string]*rbt.Node
	marketPrice   decimal.Decimal
	lock          *sync.RWMutex
}

func CreateOrderBookRepo() domain.MatchingOrderBookRepo {
	return &orderBookRepo{
		sellBook:      createBook(domain.DirectionSell),
		buyBook:       createBook(domain.DirectionBuy),
		orderMap:      make(map[int]*list.Element),
		priceLevelMap: make(map[string]*rbt.Node),
		lock:          new(sync.RWMutex),
	}
}

func (ob *orderBookRepo) AddOrderBookOrder(direction domain.DirectionEnum, order *domain.OrderEntity) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	book, err := ob.getBookByDirection(direction)
	if err != nil {
		return errors.Wrap(err, "get book failed")
	}

	var (
		priceLevel *priceLevelStruct
		curNode    *rbt.Node
	)
	if _, ok := ob.priceLevelMap[order.Price.String()]; !ok {
		priceLevel = &priceLevelStruct{
			Price:  order.Price,
			Orders: list.New(),
		}
		curNode = book.addPrice(order.Price, priceLevel)
		ob.priceLevelMap[order.Price.String()] = curNode
	} else {
		priceLevel = ob.priceLevelMap[order.Price.String()].Value.(*priceLevelStruct)
	}
	priceLevel.TotalUnfilledQuantity = priceLevel.TotalUnfilledQuantity.Add(order.UnfilledQuantity)
	orderElement := priceLevel.Orders.PushBack(order)
	ob.orderMap[order.ID] = orderElement

	return nil
}

func (ob *orderBookRepo) GetL1OrderBook() *domain.OrderBookL1Entity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	sellBestPrice := ob.sellBook.getBestPrice()
	buyBestPrice := ob.buyBook.getBestPrice()

	return &domain.OrderBookL1Entity{
		SequenceID: ob.sequenceID,
		Price:      ob.marketPrice,
		BestAsk: &domain.OrderBookL1ItemEntity{
			Price:    sellBestPrice.Price,
			Quantity: sellBestPrice.TotalUnfilledQuantity,
		},
		BestBid: &domain.OrderBookL1ItemEntity{
			Price:    buyBestPrice.Price,
			Quantity: buyBestPrice.TotalUnfilledQuantity,
		},
	}
}

func (ob *orderBookRepo) GetL2OrderBook(maxDepth int) *domain.OrderBookL2Entity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	var isUseMaxDepth bool
	if maxDepth > -1 {
		isUseMaxDepth = true
	}

	formatFn := func(iterator *rbt.Iterator) []*domain.OrderBookL2ItemEntity {
		var orderBookItems []*domain.OrderBookL2ItemEntity
		if isUseMaxDepth {
			orderBookItems = make([]*domain.OrderBookL2ItemEntity, 0, maxDepth)
		}
		for iterator.Next() {
			value := iterator.Value()
			priceLevel := value.(*priceLevelStruct)

			orderBookItem := &domain.OrderBookL2ItemEntity{
				Price:    priceLevel.Price,
				Quantity: priceLevel.TotalUnfilledQuantity,
			}

			orderBookItems = append(orderBookItems, orderBookItem)

			if isUseMaxDepth && len(orderBookItems) >= maxDepth {
				break
			}
		}
		return orderBookItems
	}

	return &domain.OrderBookL2Entity{
		SequenceID: ob.sequenceID,
		Price:      ob.marketPrice,
		Sell:       formatFn(ob.sellBook.getOrderBookIterator()),
		Buy:        formatFn(ob.buyBook.getOrderBookIterator()),
	}
}

func (ob *orderBookRepo) GetL3OrderBook(maxDepth int) *domain.OrderBookL3Entity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	var isUseMaxDepth bool
	if maxDepth > -1 {
		isUseMaxDepth = true
	}

	formatFn := func(iterator *rbt.Iterator) []*domain.OrderBookL3ItemEntity {
		var orderBookItems []*domain.OrderBookL3ItemEntity
		if isUseMaxDepth {
			orderBookItems = make([]*domain.OrderBookL3ItemEntity, 0, maxDepth)
		}
		for iterator.Next() {
			value := iterator.Value()
			priceLevel := value.(*priceLevelStruct)

			orderBookItem := &domain.OrderBookL3ItemEntity{
				Price:    priceLevel.Price,
				Quantity: priceLevel.TotalUnfilledQuantity,
			}

			for value := priceLevel.Orders.Front(); value != nil; value = value.Next() {
				order := value.Value.(*domain.OrderEntity)
				orderBookItem.Orders = append(orderBookItem.Orders, &domain.OrderL3Entity{
					SequenceID: order.SequenceID,
					Quantity:   order.UnfilledQuantity,
				})
			}

			orderBookItems = append(orderBookItems, orderBookItem)

			if isUseMaxDepth && len(orderBookItems) >= maxDepth {
				break
			}
		}
		return orderBookItems
	}

	return &domain.OrderBookL3Entity{
		SequenceID: ob.sequenceID,
		Price:      ob.marketPrice,
		Sell:       formatFn(ob.sellBook.getOrderBookIterator()),
		Buy:        formatFn(ob.buyBook.getOrderBookIterator()),
	}
}

func (ob *orderBookRepo) GetMarketPrice() decimal.Decimal {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	return ob.marketPrice
}

func (ob *orderBookRepo) SetMarketPrice(marketPrice decimal.Decimal) {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	ob.marketPrice = marketPrice
}

func (ob *orderBookRepo) SetSequenceID(sequenceID int) {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	ob.sequenceID = sequenceID
}

func (ob *orderBookRepo) GetOrderBookFirst(direction domain.DirectionEnum) (*domain.OrderEntity, error) {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	book, err := ob.getBookByDirection(direction)
	if err != nil {
		return nil, errors.Wrap(err, "get book failed")
	}

	order, err := book.getOrderBookFirst()
	if err != nil {
		return nil, errors.Wrap(err, "get order book first order failed")
	}

	return order.Clone(), nil
}

func (ob *orderBookRepo) GetSequenceID() int {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	return ob.sequenceID
}

func (ob *orderBookRepo) RemoveOrderBookOrder(direction domain.DirectionEnum, order *domain.OrderEntity) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	orderElement, ok := ob.orderMap[order.ID]
	if !ok {
		return errors.Wrap(domain.ErrNoOrder, "not found order")
	}
	priceLevelNode, ok := ob.priceLevelMap[order.Price.String()]
	if !ok {
		return errors.Wrap(domain.ErrNoPrice, "not found price")
	}
	priceLevel := priceLevelNode.Value.(*priceLevelStruct)

	if priceLevel.Orders.Remove(orderElement) == nil {
		return errors.New("remove order failed")
	}

	book, err := ob.getBookByDirection(direction)
	if err != nil {
		return errors.Wrap(err, "get book failed")
	}

	if priceLevel.Orders.Len() == 0 { // TODO: maybe no need // to search
		book.removePrice(order.Price)
		delete(ob.priceLevelMap, order.Price.String())
	}

	return nil
}

func (ob *orderBookRepo) MatchOrder(orderID int, matchedQuantity decimal.Decimal, orderStatus domain.OrderStatusEnum, updatedAt time.Time) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	element, ok := ob.orderMap[orderID]
	if !ok {
		return errors.Wrap(domain.ErrNoOrder, fmt.Sprintf("order not found, order id: %d", orderID))
	}
	order := element.Value.(*domain.OrderEntity)

	priceLevelNode, ok := ob.priceLevelMap[order.Price.String()]
	if !ok {
		return errors.Wrap(domain.ErrNoPrice, "not found price")
	}
	priceLevel := priceLevelNode.Value.(*priceLevelStruct)

	order.UnfilledQuantity = order.UnfilledQuantity.Sub(matchedQuantity)
	order.Status = orderStatus
	order.UpdatedAt = updatedAt
	priceLevel.TotalUnfilledQuantity = priceLevel.TotalUnfilledQuantity.Sub(matchedQuantity)

	return nil
}

func (ob *orderBookRepo) UpdateOrderStatus(orderID int, orderStatus domain.OrderStatusEnum, updatedAt time.Time) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	element, ok := ob.orderMap[orderID]
	if !ok {
		return errors.Wrap(domain.ErrNoOrder, fmt.Sprintf("order not found, order id: %d", orderID))
	}
	order := element.Value.(*domain.OrderEntity)

	order.Status = orderStatus
	order.UpdatedAt = updatedAt

	return nil
}

func (o *orderBookRepo) getBookByDirection(direction domain.DirectionEnum) (*bookStruct, error) {
	switch direction {
	case domain.DirectionSell:
		return o.sellBook, nil
	case domain.DirectionBuy:
		return o.buyBook, nil
	default:
		return nil, errors.New("unknown direction")
	}
}

type priceLevelStruct struct {
	Price                 decimal.Decimal
	TotalUnfilledQuantity decimal.Decimal
	Orders                *list.List
}

type bookStruct struct {
	direction domain.DirectionEnum
	orders    *rbt.Tree
	bestPrice *rbt.Node
}

func createBook(direction domain.DirectionEnum) *bookStruct {
	return &bookStruct{
		direction: direction,
		orders:    rbt.NewWith(directionEnum(direction).compare), // TODO: think performance
	}
}

func (b *bookStruct) removePrice(price decimal.Decimal) {
	b.orders.Remove(price)
	if price.Equal(b.bestPrice.Value.(*priceLevelStruct).Price) {
		iterator := b.orders.Iterator()
		iterator.Next()
		b.bestPrice = iterator.Node()
		// b.bestPrice = min(b.bestPrice.Parent, b.bestPrice.Right)
	}
}

func (b *bookStruct) getOrderBookFirst() (*domain.OrderEntity, error) {
	if b.orders.Empty() {
		return nil, errors.Wrap(domain.ErrEmptyOrderBook, "get empty book")
	} else {
		order := b.bestPrice.Value.(*priceLevelStruct).Orders.Front().Value.(*domain.OrderEntity)
		return order, nil
	}
}

func (ob *bookStruct) addPrice(price decimal.Decimal, priceLevel *priceLevelStruct) *rbt.Node {
	key := price
	ob.orders.Put(key, priceLevel)
	curNode := ob.orders.GetNode(key) // TODO: maybe push and return
	if ob.bestPrice == nil {
		ob.bestPrice = curNode
	} else {
		if directionEnum(ob.direction).compare(ob.bestPrice.Value.(*priceLevelStruct).Price, price) == 1 {
			ob.bestPrice = curNode // TODO: think
		}
	}
	return curNode
}

func (ob *bookStruct) getBestPrice() *priceLevelStruct {
	return ob.bestPrice.Value.(*priceLevelStruct)
}

func (ob *bookStruct) getOrderBookIterator() *rbt.Iterator {
	iterator := ob.orders.Iterator()
	return &iterator
}

func min(parent, right *rbt.Node) *rbt.Node {
	if right == nil {
		return parent
	}
	if parent.Value.(*priceLevelStruct).Price.LessThan(right.Value.(*priceLevelStruct).Price) {
		return parent
	}
	return right
}
