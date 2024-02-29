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
  * usecase方面: usecase使用repository依照domain interface使用，如果要`memory`替換成`mysql`，只需實作出符合interface的repository
  * delivery方面: delivery使用usecase依照domain interface使用，如果要`gin`替換成`go-kit` server，不會修改到業務邏輯
* 切出每個domain的邊界，可先以monolithic部署，如果未來有horizontal scaling需求，再以domain來deliver給不同microservice，避免一開始就使用microservice過度設計
* 高reuse，application可以從組合不同domain，來完成產品需求，例如`app/exchange`是組合`auth`與`exchange`domain
* monorepo，所有applications的底層使用`kit`，更新方便，如果套件需要版本控制也可用`git tag`處理

## exchange

![](https://i.imgur.com/KKnKXUi.png)

![](https://i.imgur.com/V7KFvvC.png)

* 壓測:
  * 機器: EC2 c5.18xlarge
  * RPS (max): 102,988.52
* 預覽網頁(❗僅用最低效能運行預覽，不是production運作規格): https://preview.exchange.messfar.com

```
┌─────────────┐                                                                 
│             │                                                                 
│             │                                                                 
│             │                                                                 
│ order event ├─────────────┐                                                   
│             │             │                                                   
│             │             │                                                   
│             │             │                                                   
└─────────────┘             │                                                   
                            ▼                                                   
┌─────────────┐      ┌─────────────┐      ┌─────────────┐                       
│             │      │             │      │             │                       
│             │      │             │      │             │                       
│             │      │             │      │             │                       
│deposit event├─────►│  sequencer  ├─────►│   trading   │                       
│             │      │             │      │             │                       
│             │      │             │      │             │                       
│             │      │             │      │             │                       
└─────────────┘      └─────────────┘      └──────┬──────┘                       
                            ▲                    │                              
┌─────────────┐             │                    ▼                              
│             │             │             ┌─────────────┐        ┌─────────────┐
│             │             │             │             │        │             │
│             │             │             │             │        │             │
│   transfer  ├─────────────┘             │             │        │             │
│    event    │                           │    asset    ├───────►│  asset mq   │
│             │                           │             │        │             │
│             │                           │             │        │             │
└─────────────┘                           │             │        │             │
                                          └──────┬──────┘        └─────────────┘
                                                 │                              
                                                 ▼                              
                                          ┌─────────────┐        ┌─────────────┐
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          │    order    │        │  ordes mq   │
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          └──────┬──────┘        └─────────────┘
                                                 │                              
                                                 ▼                              
                                          ┌─────────────┐        ┌─────────────┐
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          │   matching  │        │ matching mq │
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          └──────┬──────┘        └─────────────┘
                                                 │                              
                                                 ▼                              
                                          ┌─────────────┐        ┌─────────────┐
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          │   clearing  │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          │             │        │             │
                                          └─────────────┘        └─────────────┘
```