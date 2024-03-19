# system-design

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) clean architecture，每個domain依repository、usecase、delivery三層設計



```
.
├── app: 實際啟動的server
│
├── domain: domain interface與entity，不具邏輯，所有層都靠此domain interface傳輸entity進行溝通
│
├── auth: 依照repository、usecase、delivery實作的domain
├── chat: 同上
├── config: 同上
├── exchange: 同上
├── line: 同上
├── urshortener: 同上
│
├── kit: 對mysql、mongodb、kafka、redis等底層進行封裝，抽象出介面後始得替換更容易
│
└── instrumenting: prometheus、grafana、opentelemetry、logger等基礎建設
```

* 不同層依照domain interface進行DIP
* 對底層進行抽象，可輕易LSP
  * repository方面: repository使用mq時，可採用`kit/mq/kafka`或`kit/mq/memory`，以應付不同場景或減低測試成本
  * usecase方面: usecase使用repository依照domain interface操作，如果要`memory`替換成`mysql`，只需實作出符合interface的repository
  * delivery方面: delivery使用usecase依照domain interface操作，如果要`gin`替換成`go-kit` server，不會修改到業務邏輯
* 切出每個domain的邊界，可先以monolithic部署，如果未來有horizontal scaling需求，再以domain來deliver給不同microservice，避免一開始就使用microservice過度設計
* 高reuse，application可以從組合不同domain，來完成產品需求，例如`app/exchange-gitbitex`是組合`auth`與`exchange`domain
* monorepo，所有applications的底層使用`kit`，更新方便，如果套件需要版本控制也可用`git tag`處理
* 以testcontainers測試，更貼近真實情境進行測試

## exchange-gitbitex

![](https://i.imgur.com/KKnKXUi.png)

```
.
├── app
│   └── exchange-gitbitex
├── domain
├── auth
│   ├── delivery
│   ├── repository
│   └── usecase
│       ├── account
│       └── auth
├── exchange
│   ├── delivery
│   ├── repository
│   │   ├── sequencer
│   └── usecase
│       ├── asset
│       ├── candle
│       ├── clearing
│       ├── currency
│       ├── matching
│       ├── order
│       ├── quotation
│       └── trading
└── kit
```

撮合系統。將[exchange domain](https://github.com/superj80820/system-design/tree/master/exchange)與[gitbitex-web](https://github.com/gitbitex/gitbitex-web)串接

* 單一交易對，要實現多個交易對可以架設多個`app/exchange-gitbitex`
* 以event sourcing的方式實現，儲存event後，撮合引擎為讀取event的有限狀態機，可熱備援用多台server同時聽取event，來達到high availability
* 撮合引擎以記憶體計算，可達到100,000PRS
* 預覽網頁(❗僅用最低效能運行預覽，不是production運作規格): https://preview.exchange.messfar.com

### 壓測

使用k6進行

![](https://i.imgur.com/V7KFvvC.png)
  * exchange機器: EC2 c5.18xlarge
  * k6機器: EC2 m5.8xlarge
  * RPS (max): 102,988.52
  * 情境: 單機啟動server、mysql、kafka、redis、mongodb，並進行買賣單搓合，如果將mysql或kafka等服務獨立出來，理論上可用更便宜的機器

### 系統架構

![](./exchange-arch.png)

撮合系統主要由Sequence(定序模組)、Asset(資產模組)、Order(訂單模組)、Matching(撮合模組)、Clearing(清算模組)組成。

以訂單事件來舉例:

1. 大量併發的訂單請求進入服務
2. 定序模組會將訂單以有序的方式儲存
3. 資產模組凍結訂單所需資產
4. 訂單模組產生訂單
5. 撮合模組獲取訂單進行撮合，更新order book後產生match result
6. 清算模組依照match result來transfer、unfreeze資產
7. 各模組產生的result組成trading result，produce給下游服務，下游服務透過這些資料cache與persistent data來達到eventual consistency

由於需要快速計算撮合內容，計算都會直接在memory完成，過程中不會persistent data，但如果撮合系統崩潰，memory資料都會遺失，所以才需定序模組將訂單event都儲存好，再進入撮合系統，如此一來，如果系統崩潰也可以靠已儲存的event來recover撮合系統。

下游服務都須考慮idempotency冪等性，可透過sequence id來判斷event是否重複或超前，如果重複就拋棄，超前就必須讀取先前的event。

### Sequence 定序模組

![](./sequencer.jpg)

如何快速儲存event是影響系統寫入速度的關鍵，kafka是可考慮的選項之一。

kafka為append-only logs，不需像RDBMS在需查找與更新索引會增加磁碟I/O操作，並且使用zero-copy快速寫入磁碟來persistent。

* TODO: 講解uniq去重

create order API只需將snowflake的`orderID`、`referenceID`(全局參考ID)等metadata帶入event，並傳送給kafka sequence topic，即完成了創建訂單的事項，可回傳`200 OK`給客戶端。

```go
func (t *tradingUseCase) ProduceCreateOrderTradingEvent(ctx context.Context, userID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}
	orderID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventCreateOrderType,
		OrderRequestEvent: &domain.OrderRequestEvent{
			UserID:    userID,
			OrderID:   orderID,
			Direction: direction,
			Price:     price,
			Quantity:  quantity,
		},
	}

	if err := t.sequencerRepo.ProduceSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}
```

#### Explicit Commit

kafka sequence topic的consume到event後，需為event定序，將一批已經定序events的透過`sequencerRepo.SaveEvents()`儲存，儲存過程中有可能會有失敗，如失敗就不對kafka進行commit，下次consume會消費到同批events重試，直到成功在commit。

如果是`sequencerRepo.SaveEvents()`儲存成功，但commit失敗，下次consume也會消費到同批events，這時需ignore掉已儲存的events，只儲存新的events，在用最新的event進行commit。

雖然沒辦法保證每次consume都成功處理，但我們可以確保consume失敗後會重試直到成功再commit。

```go
t.sequencerRepo.SubscribeGlobalSequenceMessages(func(tradingEvents []*domain.TradingEvent, commitFn func() error) {
  sequencerEvents := make([]*domain.SequencerEvent, len(tradingEvents))
  tradingEventsClone := make([]*domain.TradingEvent, len(tradingEvents))
  for idx := range tradingEvents {
    sequencerEvent, tradingEvent, err := t.sequenceMessage(tradingEvents[idx])
    if err != nil {
      setErrAndDone(errors.Wrap(err, "sequence message failed"))
      return
    }
    sequencerEvents[idx] = sequencerEvent
    tradingEventsClone[idx] = tradingEvent
  }

  err := t.sequencerRepo.SaveEvents(sequencerEvents)
  if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
    // if duplicate, filter events then retry
    // another code...
    return
  } else if err != nil {
    panic(errors.Wrap(err, "save event failed"))
  }

  if err := commitFn(); err != nil {
    setErrAndDone(errors.Wrap(err, "commit latest message failed"))
    return
  }

  t.tradingRepo.SendTradeEvent(ctx, tradingEventsClone)
})
```

### Asset 資產模組

![](./asset.jpg)

用戶資產除了基本的UserID、AssetID、Available(可用資產)，還需Frozen(凍結資產)欄位，以實現用戶下單時把下單資金凍結。

```go
type UserAsset struct {
	UserID    int
	AssetID   int
	Available decimal.Decimal
	Frozen    decimal.Decimal
}
```

一個交易對會有多個用戶，並且每個用戶都會有多個資產，如果資產都需存在memory當中，可以用兩個hash table來實作asset repository。

由於在撮合時也有機會透過API讀取memory的用戶資產，所以須用read-write-lock保護。

```go
type assetRepo struct {
	usersAssetsMap map[int]map[int]*domain.UserAsset
	lock           *sync.RWMutex

	// another code...
}
```

用戶資產的use case主要有轉帳`Transfer()`、凍結`Freeze()`、解凍`Unfreeze()`，另外我們還需liability user(負債帳戶)，他代表整個系統實際的資產，也可用來對帳，負債帳戶透過`LiabilityUserTransfer()`將資產從系統轉給用戶。

```go
type UserAssetUseCase interface {
	LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	TransferFrozenToAvailable(ctx context.Context, fromUserID, toUserID int, assetID int, amount decimal.Decimal) (*TransferResult, error)
	TransferAvailableToAvailable(ctx context.Context, fromUserID, toUserID int, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)
	Unfreeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*TransferResult, error)

	// ..another functions
}
```

如果liability user id為1、用戶A user id為100、btc id為1、usdt id為2，其他用戶在系統已存入10btc與100000usdt。

此時用戶A對系統deposit儲值100000 usdt，並下了1顆btc價格為62000usdt的買單，此時需凍結62000usdt，將資產直接展開成一個二維表顯示如下:

|user id|asset id|available|frozen|
|---|---|---|---|
|1|1|-10|0|
|1|2|-200000|0|
|100|1|0|0|
|100|2|38000|62000|
|其他用戶...||||

在deposit時會呼叫`LiabilityUserTransfer()`，`assetRepo`呼叫`GetAssetWithInit()`，如果用戶資產存在則返回，不存在則初始化創建，資產模組在需要使用用戶資產時才創建他，不需先預載用戶資料表，也因為如此，在進入撮合系統前的auth非常重要，必須是認證過的用戶才能進入資產模組。

liability user呼叫`SubAssetAvailable()`減少資產，用戶呼叫`AddAssetAvailable()`獲取資產，資產的變動會呼叫`transferResult.addUserAsset()`添加至`transferResult`，最後回應給下游服務。

```go
func (u *userAsset) LiabilityUserTransfer(ctx context.Context, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("can not operate less or equal zero amount")
	}

	transferResult := createTransferResult()

	liabilityUserAsset, err := u.assetRepo.GetAssetWithInit(domain.LiabilityUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}
	toUserAsset, err := u.assetRepo.GetAssetWithInit(toUserID, assetID)
	if err != nil {
		return nil, errors.Wrap(err, "get asset with init failed")
	}

	u.assetRepo.SubAssetAvailable(liabilityUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(liabilityUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}
```

其他的use case都與`LiabilityUserTransfer()`類似，只是資產的轉移有差異，最後都會透過`transferResult`將變動的資產回應給下游服務。

在創建訂單時呼叫`Freeze()`，先檢查用戶是否有足夠資產`userAsset.Available.Cmp(amount) < 0`，如果足夠則呼叫`SubAssetAvailable()`減少資產，並呼叫`AddAssetFrozen()`增加凍結資產。

```go
func (u *userAsset) Freeze(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	// another code...

	if userAsset.Available.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetAvailable(userAsset, amount)
	u.assetRepo.AddAssetFrozen(userAsset, amount)

	transferResult.addUserAsset(userAsset)

	return transferResult.TransferResult, nil
}
```

在撮合訂單時呼叫`TransferFrozenToAvailable()`，先檢查用戶(fromUser)是否有足夠的凍結資產後呼叫`SubAssetFrozen()`減少凍結用戶(fromUser)資產，並呼叫`AddAssetAvailable()`增加用戶(toUser)資產。

```go
func (u *userAsset) TransferFrozenToAvailable(ctx context.Context, fromUserID, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	// another code...

	if fromUserAsset.Frozen.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetFrozen(fromUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(fromUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}
```

在用戶轉帳時呼叫`TransferAvailableToAvailable()`，檢查用戶(fromUser)是否有足夠資產後呼叫`SubAssetAvailable()`減少資產並呼叫`AddAssetAvailable()`增加用戶(toUser)資產。

```go
func (u *userAsset) TransferAvailableToAvailable(ctx context.Context, fromUserID, toUserID, assetID int, amount decimal.Decimal) (*domain.TransferResult, error) {
	// another code...

	if fromUserAsset.Available.Cmp(amount) < 0 {
		return nil, errors.Wrap(domain.LessAmountErr, "less amount err")
	}
	u.assetRepo.SubAssetAvailable(fromUserAsset, amount)
	u.assetRepo.AddAssetAvailable(toUserAsset, amount)

	transferResult.addUserAsset(fromUserAsset)
	transferResult.addUserAsset(toUserAsset)

	return transferResult.TransferResult, nil
}
```

### Order 訂單模組

![](./order.jpg)

活動訂單需由訂單模組管理，可以用hash table以`orderID: order`儲存所有活動訂單，在需要以用戶ID取得相關活動訂單時，可用兩層hash table以`userID: orderID: order`取得訂單。

由於在撮合時也有機會透過API讀取memory的活動訂單，所以須用read-write-lock保護。

```go
type orderUseCase struct {
	lock          *sync.RWMutex
	activeOrders  map[int]*domain.OrderEntity
	userOrdersMap map[int]map[int]*domain.OrderEntity

  // another code...
}
```

活動訂單的欄位介紹如下:

```go
type OrderEntity struct {
	ID         int // 訂單ID
	SequenceID int // 訂單由定序模組所定的ID
	UserID     int // 用戶ID

	Price     decimal.Decimal // 價格
	Direction DirectionEnum   // 買單還是賣單

	// 狀態:
	// 完全成交(Fully Filled)、
	// 部分成交(Partial Filled)、
	// 等待成交(Pending)、
	// 完全取消(Fully Canceled)、
	// 部分取消(Partial Canceled)
	Status OrderStatusEnum

	Quantity         decimal.Decimal // 數量
	UnfilledQuantity decimal.Decimal // 未成交數量

	CreatedAt time.Time // 創建時間
	UpdatedAt time.Time // 更新時間
}
```

訂單模組主要的use case有創建訂單`CreateOrder()`、刪除訂單`RemoveOrder()`、取得訂單`GetUserOrders()`，由於在創建時需要凍結資產、刪除時需要解凍資產，所以須注入資產模組`UserAssetUseCase`。

```go
type OrderUseCase interface {
	CreateOrder(ctx context.Context, sequenceID int, orderID, userID int, direction DirectionEnum, price, quantity decimal.Decimal, ts time.Time) (*OrderEntity, *TransferResult, error)
	RemoveOrder(ctx context.Context, orderID int) error
	GetUserOrders(userID int) (map[int]*OrderEntity, error)

  // another code...
}
```

資產模組在此處的命名為`o.assetUseCase`。

在創建訂單時呼叫`CreateOrder()`，需先判斷買賣單凍結用戶資產`o.assetUseCase.Freeze()`，之後創建`order`存入`o.activeOrders`與`o.userOrdersMap`，由於過程中有資產的變化，所以除了`order`以外，也需將`transferResult`回應給下游服務。

```go
func (o *orderUseCase) CreateOrder(ctx context.Context, sequenceID int, orderID int, userID int, direction domain.DirectionEnum, price decimal.Decimal, quantity decimal.Decimal, ts time.Time) (*domain.OrderEntity, *domain.TransferResult, error) {
	var err error
	transferResult := new(domain.TransferResult)

	switch direction {
	case domain.DirectionSell:
		transferResult, err = o.assetUseCase.Freeze(ctx, userID, o.baseCurrencyID, quantity)
		if err != nil {
			return nil, nil, errors.Wrap(err, "freeze base currency failed")
		}
	case domain.DirectionBuy:
		transferResult, err = o.assetUseCase.Freeze(ctx, userID, o.quoteCurrencyID, price.Mul(quantity))
		if err != nil {
			return nil, nil, errors.Wrap(err, "freeze base currency failed")
		}
	default:
		return nil, nil, errors.New("unknown direction")
	}

	order := domain.OrderEntity{
		ID:               orderID,
		SequenceID:       sequenceID,
		UserID:           userID,
		Direction:        direction,
		Price:            price,
		Quantity:         quantity,
		UnfilledQuantity: quantity,
		Status:           domain.OrderStatusPending,
		CreatedAt:        ts,
		UpdatedAt:        ts,
	}

	o.lock.Lock()
	o.activeOrders[orderID] = &order
	if _, ok := o.userOrdersMap[userID]; !ok {
		o.userOrdersMap[userID] = make(map[int]*domain.OrderEntity)
	}
	o.userOrdersMap[userID][orderID] = &order
	o.lock.Unlock()

	return &order, transferResult, nil
}
```

在撮合訂單完全成交、取消訂單時會呼叫`RemoveOrder()`，`o.activeOrders`刪除訂單，而`o.userOrdersMap`透過訂單的userID查找也刪除訂單。

```go
func (o *orderUseCase) RemoveOrder(ctx context.Context, orderID int) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	removedOrder, ok := o.activeOrders[orderID]
	if !ok {
		return errors.New("order not found in active orders")
	}
	delete(o.activeOrders, orderID)

	userOrders, ok := o.userOrdersMap[removedOrder.UserID]
	if !ok {
		return errors.New("user orders not found")
	}
	_, ok = userOrders[orderID]
	if !ok {
		return errors.New("order not found in user orders")
	}
	delete(userOrders, orderID)

	return nil
}
```

API取得訂單會呼叫`GetUserOrders()`，為了避免race-condition，查找到訂單後需將訂單clone再回傳。

```go
func (o *orderUseCase) GetUserOrders(userId int) (map[int]*domain.OrderEntity, error) {
	o.lock.RLock()
	defer o.lock.RUnlock()

	userOrders, ok := o.userOrdersMap[userId]
	if !ok {
		return nil, errors.New("get user orders failed")
	}
	userOrdersClone := make(map[int]*domain.OrderEntity, len(userOrders))
	for orderID, order := range userOrders {
		userOrdersClone[orderID] = order.Clone()
	}
	return userOrdersClone, nil
}
```

### Matching 撮合模組

![](./matching.jpg)

撮合模組是整個交易系統最核心的部分。

主要核心是維護一個買單book一個賣單book

![](./order-book.jpg)

訂單簿必須是「價格優先、序列號次要」，直觀上會把這兩個book當成兩個list來看
  * 賣單簿: 4.45, 4.52, 4.63, 4.64, 4.65
  * 買單簿: 4.15, 4.08, 4.06, 4.05, 4.01

但如果就把order-book用2個double-link-list來設計，這樣效能並不好，以查詢、插入、刪除來看:
  * 查詢: O(n)
  * 插入: O(n)，雖然插入是O(1)，但要先找到適合的價格插入，也要耗費O(n)
  * 刪除: O(n)，雖然刪除是O(1)，但要先查詢到價格再刪除，也要耗費O(n)

如果搭配hash-table，查詢跟刪除是可以達到O(1)，但插入還是O(n)，有沒有更適合的資料結構呢？

我們可用平衡二元搜尋樹，可考慮red-black-tree(以下簡稱rbtree)或AVL-tree，rb-tree沒有AVL-tree來的平衡，但插入與刪除時為了平衡的旋轉較少，兩者可簡化判斷要用在哪些場景:
  * rbtree: 插入與刪除較多
  * AVL-tree: 查詢較多

order-book會有大量訂單插入與撮合刪除場景，會較適合選擇rbtree，這樣插入速度就變快了，如下:
    * 查詢: O(logn)
    * 插入: O(logn)
    * 刪除: O(logn)
    
但這樣其他操作的速度就變慢了，有沒有方法可以更加優化呢？

我們可以rbtree+double-link-list(以下簡稱list)+hash-table，rbtree的node儲存的不是一個order而是存有相同價格的order list，而hash-table有兩個，一個儲存list在tree上的位置，一個儲存order在list上的位置。
如此一來在第1次插入此價格或從訂單簿移除整個價格時速度為O(logn)，但在建立node後，後續的插入與刪除實際上是對list的操作，由於整個撮合模組是依照序列號有序輸入order的，所以只需直接對list的尾端insert order即可，速度為O(1)。

所以統整速度可以得到以下:
* 查詢: O(1)
* 第1次插入價格: O(logn)
* 移除整個訂單簿價格: O(logn)
* 插入: O(1)，透過hash-table插入list上的order
* 刪除: O(1)，透過hash-table刪除list上的order

所以我們的order-book一側定義為`rbtree<priceLevelEntity.price, priceLevelEntity>`的結構體，`price`為compare的key，`priceLevelEntity`為實際的value。

```go
type bookEntity struct {
	direction   domain.DirectionEnum
	orderLevels *rbtree.Tree // <priceLevelEntity.price, priceLevelEntity>
	bestPrice   *rbtree.Node
}

func createBook(direction domain.DirectionEnum) *bookEntity {
	return &bookEntity{
		direction:   direction,
		orderLevels: rbtree.NewWith(directionEnum(direction).compare),
	}
}
```

`priceLevelEntity`存放此price所有的order list，可以再設一個`totalUnfilledQuantity`，在存放order時也紀錄數量。

```go
type priceLevelEntity struct {
	price                 decimal.Decimal
	totalUnfilledQuantity decimal.Decimal
	orders                *list.List
}
```

買單簿與賣單簿差異只有rbtree排序的不同，依照不同的方向`directionEnum`決定compare的方式，買單簿`domain.DirectionBuy`以`高價在前`，賣單簿`domain.DirectionSell`以`低價在前`。

```go
type directionEnum domain.DirectionEnum

func (d directionEnum) compare(a, b interface{}) int {
	aPrice := a.(decimal.Decimal)
	bPrice := b.(decimal.Decimal)

	switch domain.DirectionEnum(d) {
	case domain.DirectionSell:
		// low price is first
		return aPrice.Cmp(bPrice)
	case domain.DirectionBuy:
		// hight price is first
		return bPrice.Cmp(aPrice)
	case domain.DirectionUnknown:
		panic("unknown direction")
	default:
		panic("unknown direction")
	}
}
```

將不同方向的`bookEntity`定義為`sellBook`、`buyBook`，`orderMap`紀錄order在list的位置以便刪除可以用O(1)的速度，`priceLevelMap`紀錄`priceLevelEntity` Node在樹的位置以便取用可以用O(1)的速度。這樣就有一個order-book了

```go
type orderBookRepo struct {
	sequenceID    int
	sellBook      *bookEntity
	buyBook       *bookEntity
	orderMap      map[int]*list.Element
	priceLevelMap map[string]*rbtree.Node
	
	// another code...
}

func CreateOrderBookRepo(cache *redisKit.Cache, orderBookMQTopic, l1OrderBookMQTopic, l2OrderBookMQTopic, l3OrderBookMQTopic mq.MQTopic) domain.MatchingOrderBookRepo {
	return &orderBookRepo{
		sellBook:      createBook(domain.DirectionSell),
		buyBook:       createBook(domain.DirectionBuy),
		orderMap:      make(map[int]*list.Element),
		priceLevelMap: make(map[string]*rbtree.Node),

		// another code...
	}
}
```

撮合過程是對order-book的一系列操作，有四個重要的methods，以下操作會以括號說明time complexity:
* `GetOrderBookFirst`: 取得特定方向的最佳價格，是為rbtree的最小值，order list的第一個
* `AddOrderBookOrder`: 加入特定方向order，如果rbtree不存在此價格則插入至price level(O(logn))，如果已存在則用`priceLevelMap`直接插入至price level(O(1))
* `RemoveOrderBookOrder`: 刪除特定方向order，用`orderMap`直接刪除order list裡的order(O(1))，用`priceLevelMap`檢查order list是否為0，如果為0則刪除rbtree這價格的Node(O(logn))
* `MatchOrder`: 用`orderMap`更新order的數量與狀態(O(1))

在不須新增或刪除整個價格時，不會對rbtree操作，只操作hash-table與list，這讓速度為O(1)，是一個需注意的細節。

```go
type MatchingOrderBookRepo interface {
	GetOrderBookFirst(direction DirectionEnum) (*OrderEntity, error)
	AddOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	RemoveOrderBookOrder(direction DirectionEnum, order *OrderEntity) error
	MatchOrder(orderID int, matchedQuantity decimal.Decimal, orderStatus OrderStatusEnum, updatedAt time.Time) error

	// another code...
}
```

簡單來說，限價單撮合過程就是taker不斷與對手盤的最佳價格撮合。我直接貼上`NewOrder`的code，透過註解解釋:

```go
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
			// 更新maker order
			makerOrder.Status = domain.OrderStatusFullyFilled
			if err := m.matchingOrderBookRepo.MatchOrder(makerOrder.ID, matchedQuantity, domain.OrderStatusFullyFilled, takerOrder.CreatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			// 從order-book移除
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
```

最後的`MatchResult`我們將傳送給下游模組進行處理。

### Clearing 清算模組

![](./clearing.jpg)

* 撮合模組撮合後，雙方資產還沒有實際交換，訂單也還沒更新，必須再透過清算模組進行處理，故注入資產模組與訂單模組

```go
type clearingUseCase struct {
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase

	// another code...
}
```

* 清算模組需實作的method不多，只需實作`ClearMatchResult()`，將`MatchResult`帶入，資產的轉換結果生成`TransferResult`提供給下游系統使用

```go
type ClearingUseCase interface {
	ClearMatchResult(ctx context.Context, matchResult *MatchResult) (*TransferResult, error)

	// another code...
}
```

將taker訂單更新，並依照taker的方向進行清算，以下分別介紹

```go
func (c *clearingUseCase) ClearMatchResult(ctx context.Context, matchResult *domain.MatchResult) (*domain.TransferResult, error) {
	// 新增`transferResult`紀錄資產轉換結果
	transferResult := new(domain.TransferResult)

	// 更新taker在訂單模組的狀態
	taker := matchResult.TakerOrder
	if err := c.orderUseCase.UpdateOrder(ctx, taker.ID, taker.UnfilledQuantity, taker.Status, taker.UpdatedAt); err != nil {
		return nil, errors.Wrap(err, "update order failed")
	}
	switch matchResult.TakerOrder.Direction {
	// taker是賣單的情形
	case domain.DirectionSell:
		// another code...
	// taker是買單的情形
	case domain.DirectionBuy:
		// another code...
	default:
		return nil, errors.New("unknown direction")
	}

	return transferResult, nil
}
```

如果taker是賣單，依照`MatchResult`的`MatchDetails`逐一將maker訂單更新:
  * 將taker賣單凍結的數量轉換給maker，數量是`matched`
  * 將maker買單凍結的資產轉換給taker，金額是`(maker price)*matched`

並把taker跟maker最後的資產紀錄在`TransferResult`

```go
	// taker是賣單的情形
	case domain.DirectionSell:
		// 依照`MatchDetails`逐一拿出撮合maker的細節來處理
		for _, matchDetail := range matchResult.MatchDetails {
			// 更新maker在訂單模組的狀態
			maker := matchDetail.MakerOrder
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			matched := matchDetail.Quantity

			// 將taker賣單凍結的數量轉換給maker，數量是matched
			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			// 將maker買單凍結的資產轉換給taker，金額是(maker price)*matched
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			// 如果maker買單數量減至0，則移除訂單
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove failed")
				}
			}
		}
		// 如果taker買單數量減至0，則移除訂單
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(ctx, taker.ID); err != nil {
				return nil, errors.Wrap(err, "remove failed")
			}
		}
```

如果taker是買單，則相反:
  * 將taker買單凍結的資產轉換給maker，金額是`(maker price)*matched`
  * 將maker賣單凍結的資產轉換給taker，數量是`matched`

需注意的是，如果taker是買單，凍結的資產金額是(taker price)*matched，須退還`(taker price-maker price)*matched`金額給taker。

例如10usdt price數量3btc的買單，需凍結30usdt，如果撮合到1usdt price數量2btc的賣單，撮合到2btc，taker理應用`1*2=2usdt`會獲得2btc，須退還`(10-1)*2=18usdt`給taker

```go
	// taker是買單的情形
	case domain.DirectionBuy:
		// 依照`MatchDetails`逐一拿出撮合maker的細節來處理
		for _, matchDetail := range matchResult.MatchDetails {
			// 更新maker在訂單模組的狀態
			maker := matchDetail.MakerOrder
			if err := c.orderUseCase.UpdateOrder(ctx, maker.ID, maker.UnfilledQuantity, maker.Status, maker.UpdatedAt); err != nil {
				return nil, errors.Wrap(err, "update order failed")
			}
			matched := matchDetail.Quantity

			// taker買單的價格如果高於maker賣單，則多出來的價格須退還給taker，金額是(taker price-maker price)*matched
			// 退還給taker不需紀錄在`transferResult`，因為後續taker還會從maker拿到資產，taker資產的結果只需紀錄最後一個就可以了
			if taker.Price.Cmp(maker.Price) > 0 {
				unfreezeQuote := taker.Price.Sub(maker.Price).Mul(matched)
				_, err := c.userAssetUseCase.Unfreeze(ctx, taker.UserID, c.quoteCurrencyID, unfreezeQuote)
				if err != nil {
					return nil, errors.Wrap(err, "unfreeze taker failed")
				}
			}
			// 將taker買單凍結的資產轉換給maker，金額是(maker price)*matched
			transferResultOne, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, taker.UserID, maker.UserID, c.quoteCurrencyID, maker.Price.Mul(matched))
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultOne.UserAssets...)
			// 將maker賣單凍結的資產轉換給taker，數量是matched
			transferResultTwo, err := c.userAssetUseCase.TransferFrozenToAvailable(ctx, maker.UserID, taker.UserID, c.baseCurrencyID, matched)
			if err != nil {
				return nil, errors.Wrap(err, "transfer failed")
			}
			// 將轉換結果紀錄
			transferResult.UserAssets = append(transferResult.UserAssets, transferResultTwo.UserAssets...)
			// 如果maker買單數量減至0，則移除訂單
			if maker.UnfilledQuantity.IsZero() {
				if err := c.orderUseCase.RemoveOrder(ctx, maker.ID); err != nil {
					return nil, errors.Wrap(err, "remove maker order failed, maker order id: "+strconv.Itoa(maker.ID))
				}
			}
		}
		// 如果taker買單數量減至0，則移除訂單
		if taker.UnfilledQuantity.IsZero() {
			if err := c.orderUseCase.RemoveOrder(ctx, taker.ID); err != nil {
				return nil, errors.Wrap(err, "remove taker order failed")
			}
		}
```

最後的`TransferResult`我們將傳送給下游模組進行處理。

### 整合交易系統

交易系統分為兩大部分，一個是非同步的`TradingUseCase()`、一個是同步的`syncTradingUseCase()`，`TradingUseCase()`將無序的`TradingEvent`蒐集後，有序的逐筆送入`syncTradingUseCase()`，我們會希望各種IO在`TradingUseCase()`完成，撮合這些需高速計算的部分在純memory的`syncTradingUseCase()`完成並得到trading result，再交給`TradingUseCase()`進行persistent，最後達到eventual consistency。

我們先介紹`tradingUseCase`，再介紹實作相對簡單的`syncTradingUseCase`。

`tradingUseCase`需注入相關IO

```go
type tradingUseCase struct {
	userAssetRepo domain.UserAssetRepo
	tradingRepo   domain.TradingRepo
	candleRepo    domain.CandleRepo
	quotationRepo domain.QuotationRepo
	matchingRepo  domain.MatchingRepo
	orderRepo     domain.OrderRepo

	sequenceTradingUseCase domain.SequenceTradingUseCase
	syncTradingUseCase     domain.SyncTradingUseCase

	lastSequenceID int

	// another code...
}
```

`TradingUseCase`主要實現以下幾個methods

```go
type TradingUseCase interface {
	ConsumeTradingEvents(ctx context.Context, key string)

	ConsumeTradingResult(ctx context.Context, key string)

	ProcessTradingEvents(ctx context.Context, tradingEvents []*TradingEvent) error

	// another code...
}
```

`ConsumeTradingEvents()`: consume已經過定序模組定序過的trading events，並傳入`ProcessTradingEvents()`，如果成功則進行explicit commit

```go
func (t *tradingUseCase) ConsumeTradingEvents(ctx context.Context, key string) {
  // another code...

	t.tradingRepo.ConsumeTradingEvents(ctx, key, func(events []*domain.TradingEvent, commitFn func() error) {
		if err := t.ProcessTradingEvents(ctx, events); err != nil {
			// error handle code...
		}
		if err := commitFn(); err != nil {
			// error handle code...
		}
	})
}
```

`ProcessTradingEvents()`處理trading events，需先透過sequence id是確認idempotency冪等性:
* 如果event丟失: 透過`t.sequenceTradingUseCase.RecoverEvents()`從db讀取event
* 如果event重複: 忽略event

確認訊息順序正確後，才`processTradingEvent()`進行處理

```go
func (t *tradingUseCase) ProcessTradingEvents(ctx context.Context, tes []*domain.TradingEvent) error {
	err := t.sequenceTradingUseCase.CheckEventSequence(tes[0].SequenceID, t.lastSequenceID)
	if errors.Is(err, domain.ErrMissEvent) {
		t.logger.Warn("miss events. first event id", loggerKit.Int("first-event-id", tes[0].SequenceID), loggerKit.Int("last-sequence-id", t.lastSequenceID))
		t.sequenceTradingUseCase.RecoverEvents(t.lastSequenceID, func(tradingEvents []*domain.TradingEvent) error {
			for _, te := range tradingEvents {
				if err := t.processTradingEvent(ctx, te); err != nil {
					return errors.Wrap(err, "process trading event failed")
				}
			}
			return nil
		})
		return nil
	}
	for _, te := range tes {
		err := t.sequenceTradingUseCase.CheckEventSequence(te.SequenceID, t.lastSequenceID)
		if errors.Is(err, domain.ErrGetDuplicateEvent) {
			t.logger.Warn("get duplicate events. first event id", loggerKit.Int("first-event-id", tes[0].SequenceID), loggerKit.Int("last-sequence-id", t.lastSequenceID))
			continue
		}
		if err := t.processTradingEvent(ctx, te); err != nil {
			return errors.Wrap(err, "process trading event failed")
		}
	}
	return nil
}
```

sequence id更新至`t.lastSequenceID`，並依照event type呼叫對應的method給`syncTradingUseCase`獲取對應result，以新增訂單來說，`syncTradingUseCase.CreateOrder()`會獲得`MatchResult`、`TransferResult`，需將他們包裝成`TradingResult`，透過`tradingRepo.ProduceTradingResult()`傳至`trading result MQ`。

這裡的`trading result MQ`，是用memory的MQ，實際上是當做一個`ring buffer`，把trading result蒐集起來，並以batch的方式傳入下游系統MQ，以增加throughput，不然一筆一筆傳入下游，這裡會成為bottleneck。

```go
func (t *tradingUseCase) processTradingEvent(ctx context.Context, te *domain.TradingEvent) error {
	var tradingResult domain.TradingResult

	t.lastSequenceID = te.SequenceID

	switch te.EventType {
	case domain.TradingEventCreateOrderType:
		matchResult, transferResult, err := t.syncTradingUseCase.CreateOrder(ctx, te)
		if errors.Is(err, domain.LessAmountErr) || errors.Is(err, domain.InvalidAmountErr) {
			t.logger.Info(fmt.Sprintf("%+v", err))
			return nil
		} else if err != nil {
			return errors.Wrap(err, "process message get failed")
		}

		tradingResult = domain.TradingResult{
			SequenceID:          te.SequenceID,
			TradingResultStatus: domain.TradingResultStatusCreate,
			TradingEvent:        te,
			MatchResult:         matchResult,
			TransferResult:      transferResult,
		}
	case domain.TradingEventCancelOrderType:
		// another code...
	case domain.TradingEventTransferType:
		// another code...
	case domain.TradingEventDepositType:
		// another code...
	default:
		return errors.New("unknown event type")
	}

	if err := t.tradingRepo.ProduceTradingResult(ctx, &tradingResult); err != nil {
		panic(errors.Wrap(err, "produce trading result failed"))
	}

	return nil
}
```

`ConsumeTradingResult()`batch consume `trading result MQ`，批次將trading results傳入下游`candle MQ`、`asset MQ`等下游系統。

```go
func (t *tradingUseCase) ConsumeTradingResult(ctx context.Context, key string) {
	t.tradingRepo.ConsumeTradingResult(ctx, key, func(tradingResults []*domain.TradingResult) error {
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			if err := t.userAssetRepo.ProduceUserAssetByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce order failed")
			}
			return nil
		})

		eg.Go(func() error {
			if err := t.candleRepo.ProduceCandleMQByTradingResults(ctx, tradingResults); err != nil {
				return errors.Wrap(err, "produce candle failed")
			}
			return nil
		})

    // another code...

		if err := eg.Wait(); err != nil {
			panic(errors.Wrap(err, "produce failed"))
		}

		return nil
	})
}
```

`syncTradingUseCase`相對簡單，需注入資產模組、訂單模組、撮合模組、清算模組來實作`CreateOrder()`等等methods

```go
type syncTradingUseCase struct {
	userAssetUseCase domain.UserAssetUseCase
	orderUseCase     domain.OrderUseCase
	matchingUseCase  domain.MatchingUseCase
	clearingUseCase  domain.ClearingUseCase
}

type SyncTradingUseCase interface {
	CreateOrder(ctx context.Context, tradingEvent *TradingEvent) (*MatchResult, *TransferResult, error)

  // another code...
}
```

`CreateOrder()`即帶入`TradingEvent`後，依序呼叫資產模組、訂單模組、撮合模組、清算模組，並獲得各個results後回傳。

```go
func (t *syncTradingUseCase) CreateOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.MatchResult, *domain.TransferResult, error) {
	order, transferResult, err := t.orderUseCase.CreateOrder(
		ctx,
		tradingEvent.SequenceID,
		tradingEvent.OrderRequestEvent.OrderID,
		tradingEvent.OrderRequestEvent.UserID,
		tradingEvent.OrderRequestEvent.Direction,
		tradingEvent.OrderRequestEvent.Price,
		tradingEvent.OrderRequestEvent.Quantity,
		tradingEvent.CreatedAt,
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create order failed")
	}
	matchResult, err := t.matchingUseCase.NewOrder(ctx, order)
	if err != nil {
		return nil, nil, errors.Wrap(err, "matching order failed")
	}
	clearTransferResult, err := t.clearingUseCase.ClearMatchResult(ctx, matchResult)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clear match order failed")
	}

	transferResult.UserAssets = append(transferResult.UserAssets, clearTransferResult.UserAssets...)

	return matchResult, transferResult, nil
}
```

這樣就完成了一個交易系統，需要注意的是:
* 下游系統是可能收到重複的消息的，需忽略這些消息，可以將last sequence id存至redis，也可將db table設有sequence id欄位來檢查
* 交易系統是有可能崩潰的，雖然我們可以靠儲存在db的sequence trading events來recover，但每次都從頭讀取events會消耗大量時間，我們可以將交易系統memory的狀態每隔一段時間snapshot備份起來，在下次recover的時候先從snapshot讀取，再從db讀取

### 運行

* require:
  * golang v1.20
  * docker

* development:
  ```
  $ cd app/exchange-gitbitex
  $ make dev
  ```

### 後記

感謝以下參考資源，讓我完成此系統，如有錯誤，敬請賜教，謝謝。

### 參考

* [廖雪峰-设计交易引擎](https://www.liaoxuefeng.com/wiki/1252599548343744/1491662232616993)
* [gitbitex](https://github.com/gitbitex)
* [Linux 核心的紅黑樹](https://hackmd.io/@sysprog/linux-rbtree)
* [How to Build a Fast Limit Order Book](https://web.archive.org/web/20110219163448/http://howtohft.wordpress.com/2011/02/15/how-to-build-a-fast-limit-order-book/)
* [用 Go 語言實現固定大小的 Ring Buffer 資料結構](https://blog.wu-boy.com/2023/01/ring-buffer-queue-with-fixed-size-in-golang/)

## urlshortener

短網址服務

## chat

聊天服務