package memoryandredis

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"

	rbtree "github.com/emirpasic/gods/trees/redblacktree"
	goRedis "github.com/redis/go-redis/v9"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	"github.com/superj80820/system-design/kit/mq"
)

const (
	redisL3OrderBookLastSequenceIDKey = "_L3OrderBookSeq_"
	redisL3OrderBookKey               = "_L3OrderBook_"
	redisL2OrderBookLastSequenceIDKey = "_L2OrderBookSeq_"
	redisL2OrderBookKey               = "_L2OrderBook_"
	redisL1OrderBookLastSequenceIDKey = "_L1OrderBookSeq_"
	redisL1OrderBookKey               = "_L1OrderBook_"
)

// args
// 1: seqID
// 2: orderBook raw data
const saveOrderBookScript = `
local KEY_LAST_SEQ = KEYS[1]
local KEY_ORDER_BOOK = KEYS[2]

local seqID = ARGV[1]
local orderBookRawData = ARGV[2]

local lastSeqID = redis.call('GET', KEY_LAST_SEQ)

if not lastSeqID or tonumber(seqID) > tonumber(lastSeqID) then
	redis.call('SET', KEY_LAST_SEQ, seqID)
	redis.call('SET', KEY_ORDER_BOOK, orderBookRawData)
	return true
end

return false
`

type orderBookRepo struct {
	sequenceID    int
	sellBook      *bookEntity
	buyBook       *bookEntity
	orderMap      map[int]*list.Element
	priceLevelMap map[string]*rbtree.Node
	marketPrice   decimal.Decimal
	lock          *sync.RWMutex

	cache              *redisKit.Cache
	orderBookMQTopic   mq.MQTopic
	l1OrderBookMQTopic mq.MQTopic
	l2OrderBookMQTopic mq.MQTopic
	l3OrderBookMQTopic mq.MQTopic
}

type bookEntity struct {
	direction   domain.DirectionEnum
	orderLevels *rbtree.Tree // <priceLevelEntity.price, priceLevelEntity>
	bestPrice   *rbtree.Node
}

type priceLevelEntity struct {
	price                 decimal.Decimal
	totalUnfilledQuantity decimal.Decimal
	orders                *list.List
}

func CreateOrderBookRepo(cache *redisKit.Cache, orderBookMQTopic, l1OrderBookMQTopic, l2OrderBookMQTopic, l3OrderBookMQTopic mq.MQTopic) domain.MatchingOrderBookRepo {
	return &orderBookRepo{
		sellBook:      createBook(domain.DirectionSell),
		buyBook:       createBook(domain.DirectionBuy),
		orderMap:      make(map[int]*list.Element),
		priceLevelMap: make(map[string]*rbtree.Node),
		lock:          new(sync.RWMutex),

		cache:              cache,
		orderBookMQTopic:   orderBookMQTopic,
		l1OrderBookMQTopic: l1OrderBookMQTopic,
		l2OrderBookMQTopic: l2OrderBookMQTopic,
		l3OrderBookMQTopic: l3OrderBookMQTopic,
	}
}

func (ob *orderBookRepo) AddOrderBookOrder(direction domain.DirectionEnum, order *domain.OrderEntity) error {
	ob.lock.Lock()
	defer ob.lock.Unlock()

	var priceLevel *priceLevelEntity
	if _, ok := ob.priceLevelMap[order.Price.String()]; !ok {
		priceLevel = &priceLevelEntity{
			price:  order.Price,
			orders: list.New(),
		}

		book, err := ob.getBookByDirection(direction)
		if err != nil {
			return errors.Wrap(err, "get book failed")
		}

		curNode := book.addPrice(order.Price, priceLevel)
		ob.priceLevelMap[order.Price.String()] = curNode
	} else {
		priceLevel = ob.priceLevelMap[order.Price.String()].Value.(*priceLevelEntity)
	}
	priceLevel.totalUnfilledQuantity = priceLevel.totalUnfilledQuantity.Add(order.UnfilledQuantity)

	orderElement := priceLevel.orders.PushBack(order)
	ob.orderMap[order.ID] = orderElement

	return nil
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
	priceLevel := priceLevelNode.Value.(*priceLevelEntity)

	if priceLevel.orders.Remove(orderElement) == nil {
		return errors.New("remove order failed")
	}

	if priceLevel.orders.Len() == 0 {
		book, err := ob.getBookByDirection(direction)
		if err != nil {
			return errors.Wrap(err, "get book failed")
		}

		book.removePrice(order.Price)
		delete(ob.priceLevelMap, order.Price.String())
	}

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
			Price:    sellBestPrice.price,
			Quantity: sellBestPrice.totalUnfilledQuantity,
		},
		BestBid: &domain.OrderBookL1ItemEntity{
			Price:    buyBestPrice.price,
			Quantity: buyBestPrice.totalUnfilledQuantity,
		},
	}
}

func (ob *orderBookRepo) GetL2OrderBook() *domain.OrderBookL2Entity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	formatFn := func(iterator *rbtree.Iterator, size int) []*domain.OrderBookL2ItemEntity {
		var orderBookItems []*domain.OrderBookL2ItemEntity
		orderBookItems = make([]*domain.OrderBookL2ItemEntity, 0, size)
		for iterator.Next() {
			value := iterator.Value()
			priceLevel := value.(*priceLevelEntity)

			orderBookItem := &domain.OrderBookL2ItemEntity{
				Price:    priceLevel.price,
				Quantity: priceLevel.totalUnfilledQuantity,
			}

			orderBookItems = append(orderBookItems, orderBookItem)
		}
		return orderBookItems
	}

	return &domain.OrderBookL2Entity{
		SequenceID: ob.sequenceID,
		Price:      ob.marketPrice,
		Sell:       formatFn(ob.sellBook.getOrderBookIteratorAndSize()),
		Buy:        formatFn(ob.buyBook.getOrderBookIteratorAndSize()),
	}
}

func (ob *orderBookRepo) GetL3OrderBook() *domain.OrderBookL3Entity {
	ob.lock.RLock()
	defer ob.lock.RUnlock()

	formatFn := func(iterator *rbtree.Iterator, size int) []*domain.OrderBookL3ItemEntity {
		var orderBookItems []*domain.OrderBookL3ItemEntity
		orderBookItems = make([]*domain.OrderBookL3ItemEntity, 0, size)
		for iterator.Next() {
			value := iterator.Value()
			priceLevel := value.(*priceLevelEntity)

			orderBookItem := &domain.OrderBookL3ItemEntity{
				Price:    priceLevel.price,
				Quantity: priceLevel.totalUnfilledQuantity,
			}

			for value := priceLevel.orders.Front(); value != nil; value = value.Next() {
				order := value.Value.(*domain.OrderEntity)
				orderBookItem.Orders = append(orderBookItem.Orders, &domain.OrderL3Entity{
					SequenceID: order.SequenceID,
					Quantity:   order.UnfilledQuantity,
				})
			}

			orderBookItems = append(orderBookItems, orderBookItem)
		}
		return orderBookItems
	}

	return &domain.OrderBookL3Entity{
		SequenceID: ob.sequenceID,
		Price:      ob.marketPrice,
		Sell:       formatFn(ob.sellBook.getOrderBookIteratorAndSize()),
		Buy:        formatFn(ob.buyBook.getOrderBookIteratorAndSize()),
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
	priceLevel := priceLevelNode.Value.(*priceLevelEntity)

	order.UnfilledQuantity = order.UnfilledQuantity.Sub(matchedQuantity)
	order.Status = orderStatus
	order.UpdatedAt = updatedAt
	priceLevel.totalUnfilledQuantity = priceLevel.totalUnfilledQuantity.Sub(matchedQuantity)

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

func (ob *orderBookRepo) GetHistoryL3OrderBook(ctx context.Context, maxDepth int) (*domain.OrderBookL3Entity, error) {
	val, exist, err := ob.cache.Get(ctx, redisL3OrderBookKey)
	if err != nil {
		return nil, errors.Wrap(err, "get l3 order book failed")
	}
	if !exist {
		return nil, errors.Wrap(domain.ErrNoData, "no order book data")
	}

	var l3OrderBook domain.OrderBookL3Entity
	if err := json.Unmarshal([]byte(val), &l3OrderBook); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	if maxDepth == -1 {
		return &l3OrderBook, nil
	}

	l3OrderBookWithMaxDepth := &domain.OrderBookL3Entity{
		SequenceID: l3OrderBook.SequenceID,
		Price:      l3OrderBook.Price,
	}

	for i := 0; i < maxDepth; i++ {
		if len(l3OrderBook.Sell) <= i && len(l3OrderBook.Buy) <= i {
			break
		}
		if len(l3OrderBook.Sell) > i {
			l3OrderBookWithMaxDepth.Sell = append(l3OrderBookWithMaxDepth.Sell, l3OrderBook.Sell[i])
		}
		if len(l3OrderBook.Buy) > i {
			l3OrderBookWithMaxDepth.Buy = append(l3OrderBookWithMaxDepth.Buy, l3OrderBook.Buy[i])
		}
	}

	return l3OrderBookWithMaxDepth, nil
}

func (ob *orderBookRepo) GetHistoryL2OrderBook(ctx context.Context, maxDepth int) (*domain.OrderBookL2Entity, error) {
	val, exist, err := ob.cache.Get(ctx, redisL2OrderBookKey)
	if err != nil {
		return nil, errors.Wrap(err, "get l2 order book failed")
	}
	if !exist {
		return nil, errors.Wrap(domain.ErrNoData, "no order book data")
	}

	var l2OrderBook domain.OrderBookL2Entity
	if err := json.Unmarshal([]byte(val), &l2OrderBook); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	if maxDepth == -1 {
		return &l2OrderBook, nil
	}

	l2OrderBookWithMaxDepth := &domain.OrderBookL2Entity{
		SequenceID: l2OrderBook.SequenceID,
		Price:      l2OrderBook.Price,
	}

	for i := 0; i < maxDepth; i++ {
		if len(l2OrderBook.Sell) <= i && len(l2OrderBook.Buy) <= i {
			break
		}
		if len(l2OrderBook.Sell) > i {
			l2OrderBookWithMaxDepth.Sell = append(l2OrderBookWithMaxDepth.Sell, l2OrderBook.Sell[i])
		}
		if len(l2OrderBook.Buy) > i {
			l2OrderBookWithMaxDepth.Buy = append(l2OrderBookWithMaxDepth.Buy, l2OrderBook.Buy[i])
		}
	}

	return l2OrderBookWithMaxDepth, nil
}

func (ob *orderBookRepo) GetHistoryL1OrderBook(ctx context.Context) (*domain.OrderBookL1Entity, error) {
	val, exist, err := ob.cache.Get(ctx, redisL1OrderBookKey)
	if err != nil {
		return nil, errors.Wrap(err, "get l1 order book failed")
	}
	if !exist {
		return nil, errors.Wrap(domain.ErrNoData, "no order book data")
	}

	var l1OrderBook domain.OrderBookL1Entity
	if err := json.Unmarshal([]byte(val), &l1OrderBook); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	return &l1OrderBook, nil
}

func (ob *orderBookRepo) SaveHistoryL1OrderBookByL3OrderBook(ctx context.Context, l3OrderBook *domain.OrderBookL3Entity) (*domain.OrderBookL1Entity, error) {
	l1OrderBook := &domain.OrderBookL1Entity{
		SequenceID: l3OrderBook.SequenceID,
		Price:      l3OrderBook.Price,
	}
	if len(l3OrderBook.Sell) > 0 {
		l1OrderBook.BestAsk = &domain.OrderBookL1ItemEntity{
			Price:    l3OrderBook.Sell[0].Price,
			Quantity: l3OrderBook.Sell[0].Quantity,
		}
	}
	if len(l3OrderBook.Buy) > 0 {
		l1OrderBook.BestBid = &domain.OrderBookL1ItemEntity{
			Price:    l3OrderBook.Buy[0].Price,
			Quantity: l3OrderBook.Buy[0].Quantity,
		}
	}

	l1OrderBookMarshal, err := json.Marshal(*l1OrderBook)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	err = ob.cache.RunLua(ctx, saveOrderBookScript, []string{redisL1OrderBookLastSequenceIDKey, redisL1OrderBookKey}, l1OrderBook.SequenceID, string(l1OrderBookMarshal)).Err()
	if err != nil && !errors.Is(err, goRedis.Nil) {
		return nil, errors.Wrap(err, "save failed")
	}
	return l1OrderBook, nil
}

func (ob *orderBookRepo) SaveHistoryL2OrderBookByL3OrderBook(ctx context.Context, l3OrderBook *domain.OrderBookL3Entity) (*domain.OrderBookL2Entity, error) {
	l2OrderBook := &domain.OrderBookL2Entity{
		SequenceID: l3OrderBook.SequenceID,
		Price:      l3OrderBook.Price,
	}
	l2OrderBook.Sell = make([]*domain.OrderBookL2ItemEntity, len(l3OrderBook.Sell))
	for idx, val := range l3OrderBook.Sell {
		l2OrderBook.Sell[idx] = &domain.OrderBookL2ItemEntity{
			Price:    val.Price,
			Quantity: val.Quantity,
		}
	}
	l2OrderBook.Buy = make([]*domain.OrderBookL2ItemEntity, len(l3OrderBook.Buy))
	for idx, val := range l3OrderBook.Buy {
		l2OrderBook.Buy[idx] = &domain.OrderBookL2ItemEntity{
			Price:    val.Price,
			Quantity: val.Quantity,
		}
	}

	l2OrderBookMarshal, err := json.Marshal(*l2OrderBook)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	err = ob.cache.RunLua(ctx, saveOrderBookScript, []string{redisL2OrderBookLastSequenceIDKey, redisL2OrderBookKey}, l2OrderBook.SequenceID, string(l2OrderBookMarshal)).Err()
	if err != nil && !errors.Is(err, goRedis.Nil) {
		return nil, errors.Wrap(err, "save failed")
	}
	return l2OrderBook, nil
}

func (ob *orderBookRepo) SaveHistoryL3OrderBook(ctx context.Context, l3OrderBook *domain.OrderBookL3Entity) error {
	l3OrderBookMarshal, err := json.Marshal(*l3OrderBook)
	if err != nil {
		return errors.Wrap(err, "marshal failed")
	}
	err = ob.cache.RunLua(ctx, saveOrderBookScript, []string{redisL3OrderBookLastSequenceIDKey, redisL3OrderBookKey}, l3OrderBook.SequenceID, string(l3OrderBookMarshal)).Err()
	if err != nil && !errors.Is(err, goRedis.Nil) {
		return errors.Wrap(err, "save failed")
	}
	return nil
}

func (o *orderBookRepo) getBookByDirection(direction domain.DirectionEnum) (*bookEntity, error) {
	switch direction {
	case domain.DirectionSell:
		return o.sellBook, nil
	case domain.DirectionBuy:
		return o.buyBook, nil
	default:
		return nil, errors.New("unknown direction")
	}
}

func createBook(direction domain.DirectionEnum) *bookEntity {
	return &bookEntity{
		direction:   direction,
		orderLevels: rbtree.NewWith(directionEnum(direction).compare),
	}
}

func (b *bookEntity) removePrice(price decimal.Decimal) {
	b.orderLevels.Remove(price)

	if price.Equal(b.bestPrice.Value.(*priceLevelEntity).price) {
		iterator := b.orderLevels.Iterator()
		iterator.Next()
		b.bestPrice = iterator.Node()
	}
}

func (b *bookEntity) getOrderBookFirst() (*domain.OrderEntity, error) {
	if b.orderLevels.Empty() {
		return nil, errors.Wrap(domain.ErrEmptyOrderBook, "get empty book")
	} else {
		order := b.bestPrice.Value.(*priceLevelEntity).orders.Front().Value.(*domain.OrderEntity)
		return order, nil
	}
}

func (ob *bookEntity) addPrice(price decimal.Decimal, priceLevel *priceLevelEntity) *rbtree.Node {
	key := price
	ob.orderLevels.Put(key, priceLevel)
	curNode := ob.orderLevels.GetNode(key) // TODO: optimize. maybe push and return node in one function call
	if ob.bestPrice == nil {
		ob.bestPrice = curNode
	} else {
		if directionEnum(ob.direction).compare(ob.bestPrice.Value.(*priceLevelEntity).price, price) == 1 {
			ob.bestPrice = curNode
		}
	}
	return curNode
}

func (ob *bookEntity) getBestPrice() *priceLevelEntity {
	return ob.bestPrice.Value.(*priceLevelEntity)
}

func (ob *bookEntity) getOrderBookIteratorAndSize() (*rbtree.Iterator, int) {
	iterator := ob.orderLevels.Iterator()
	return &iterator, ob.orderLevels.Size()
}
