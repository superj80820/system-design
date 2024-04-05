# system-design

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) clean architecture，每個 domain 依 repository、usecase、delivery 三層設計

```
.
├── app: 實際啟動的server
│
├── domain: domain interface與entity，不具邏輯，所有層都靠此domain interface傳輸entity進行溝通
│
├── auth: 驗證服務
├── chat: 聊天服務
├── config: 配置服務
├── exchange: 交易服務
├── line: line API服務
├── urshortener: 短網址服務
│
├── kit: 對mysql、mongodb、kafka、redis等底層進行封裝，抽象出介面後使得替換更容易
│
└── instrumenting: prometheus、grafana、opentelemetry、logger等基礎建設
```

- 不同層依照 domain interface 進行 DIP
- 對底層進行抽象，可輕易 LSP
  - repository 方面: repository 使用 mq 時，可採用`kit/mq/kafka`或`kit/mq/memory`，以應付不同場景或減低測試成本
  - usecase 方面: usecase 使用 repository 依照 domain interface 操作，如果要`memory`替換成`mysql`，只需實作出符合 interface 的 repository
  - delivery 方面: delivery 使用 usecase 依照 domain interface 操作，如果要`gin`替換成`go-kit` server，不會修改到業務邏輯
- 切出每個 domain 的邊界，可先以 monolithic 部署，如果未來有 horizontal scaling 需求，再以 domain 來 deliver 給不同 microservice，避免一開始就使用 microservice 過度設計
- 高 reuse，application 可以從組合不同 domain，來完成產品需求，例如`app/exchange-gitbitex`是組合`auth`與`exchange`domain
- monorepo，所有 applications 的底層使用`kit`，更新方便，如果套件需要版本控制也可用`git tag`處理
- 以 testcontainers 測試，更貼近真實情境進行測試

## exchange

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
│   └── usecase
│       ├── asset
│       ├── candle
│       ├── clearing
│       ├── currency
│       ├── matching
│       ├── order
│       ├── quotation
│       ├── sequencer
│       └── trading
└── kit
```

撮合系統。將[exchange domain](https://github.com/superj80820/system-design/tree/master/exchange)與[gitbitex-web](https://github.com/gitbitex/gitbitex-web)串接

- 預覽網頁(❗ 僅用最低效能運行預覽，不是 production 運作規格): https://preview.exchange.messfar.com
- 單一交易對，要實現多個交易對可以架設多個`app/exchange-gitbitex`
- 以 event sourcing 的方式實現，儲存 event 後，撮合引擎為讀取 event 的有限狀態機，可熱備援用多台 server 同時聽取 event，來達到 high availability
- 撮合引擎以記憶體計算，可達到 100,000PRS
- 壓測(k6):
  ![](https://i.imgur.com/V7KFvvC.png)
  - exchange 機器: EC2 c5.18xlarge
  - k6 機器: EC2 m5.8xlarge
  - RPS (max): 102,988.52
  - 情境: 單機啟動 server、mysql、kafka、redis、mongodb，並進行買賣單搓合，如果將 mysql 或 kafka 等服務獨立出來，理論上可用更便宜的機器

### 教學

撰寫於[部落格](https://blog.messfar.com/golang-%E7%B3%BB%E7%B5%B1%E8%A8%AD%E8%A8%88)

### 運行

- require:

  - golang v1.20
  - docker

- development:
  ```
  $ cd app/exchange-gitbitex
  $ make dev
  ```

## urlshortener

短網址服務

## chat

聊天服務
