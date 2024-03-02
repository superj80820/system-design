# system-design

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) clean architecture，每個domain依repository、usecase、delivery三層設計


* app: 實際啟動的server
* domain: domain interface與entity，不具邏輯，所有層都靠此domain interface傳輸entity進行溝通
* kit: 對mysql、mongodb、kafka、redis等底層進行封裝，抽象出介面後始得替換更容易
* instrumenting: prometheus、grafana、opentelemetry、logger等基礎建設
* 其餘資料夾: 依照repository、usecase、delivery實作的domain

```
.
├── app
│   ├── botclient
│   ├── chat
│   ├── exchange
│   ├── exchange-gitbitex
│   ├── simple
│   ├── urlshortener
│   └── user
├── domain
├── auth
│   ├── delivery
│   ├── repository
│   │   ├── account
│   │   └── auth
│   └── usecase
│       ├── account
│       └── auth
├── chat
│   ├── delivery
│   │   └── http
│   │       └── websocket
│   ├── repository
│   └── usecase
├── config
│   └── repository
├── exchange
│   ├── delivery
│   ├── repository
│   │   ├── asset
│   │   ├── candle
│   │   ├── matching
│   │   ├── order
│   │   ├── quotation
│   │   ├── sequencer
│   │   └── trading
│   └── usecase
│       ├── asset
│       ├── candle
│       ├── clearing
│       ├── currency
│       ├── matching
│       ├── order
│       ├── quotation
│       └── trading
├── line
│   └── repository
├── urshortener
│   ├── delivery
│   ├── repository
│   └── usecase
├── kit
│   ├── cache
│   ├── code
│   ├── core
│   │   ├── endpoint
│   │   └── transport
│   │       └── http
│   │           └── websocket
│   ├── http
│   │   ├── middleware
│   │   ├── transport
│   │   └── websocket
│   │       └── middleware
│   ├── logger
│   ├── mq
│   │   ├── kafka
│   │   └── memory
│   ├── orm
│   ├── ratelimit
│   │   ├── memory
│   │   └── redis
│   ├── testing
│   │   ├── kafka
│   │   │   └── container
│   │   ├── mongo
│   │   │   ├── container
│   │   │   └── memory
│   │   ├── mysql
│   │   │   └── container
│   │   ├── postgres
│   │   │   └── container
│   │   └── redis
│   │       └── container
│   ├── trace
│   └── util
└── instrumenting
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

下游服務都須考慮idempotency 冪等性，可透過sequence id來判斷event是否重複或超前，如果重複就拋棄，超前就必須讀取先前的event。

### Sequence 定序模組

![](./sequencer.jpg)

如何快速儲存event是影響系統寫入速度的關鍵，kafka是可考慮的選項之一。

kafka為append-only logs，不需像RDBMS在需查找與更新索引會增加磁碟I/O操作，並且使用zero-copy快速寫入磁碟來persistent。

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

	if err := t.sequencerRepo.SendTradeSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}
```

#### Explicit Commit

kafka sequence topic的consume到event後，需為event定序，將一批已經定序events的透過`sequencerRepo.SaveEvents()`儲存，儲存過程中有可能會有失敗，如失敗就不對kafka進行commit，下次consume會消費到同批events重試，直到成功在commit。

如果是`sequencerRepo.SaveEvents()`儲存成功，但commit失敗，下次consume也會消費到同批events，這時需ignore掉已儲存的events，只儲存新的events，在用最新的event進行commit。

```go
t.sequencerRepo.SubscribeGlobalTradeSequenceMessages(func(tradingEvents []*domain.TradingEvent, commitFn func() error) {
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
    // code in https://github.com/superj80820/system-design/blob/7342610e010c4fcf15e1b4215007ec7666a1e58f/exchange/usecase/trading/trading.go#L229-L264
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



### Order 訂單模組
### Matching 撮合模組
### Clearing 清算模組

### 運行

* require:
  * golang v1.20
  * docker

* development:
  ```
  $ cd app/exchange-gitbitex
  $ make dev
  ```

### 參考

* [廖雪峰-设计交易引擎](https://www.liaoxuefeng.com/wiki/1252599548343744/1491662232616993)
* [gitbitex](https://github.com/gitbitex)

## urlshortener

短網址服務

## chat

聊天服務