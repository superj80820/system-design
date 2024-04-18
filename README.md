# system-design

## clean-architecture

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) Clean Architecture，每個 domain 依 repository、usecase、delivery 三層設計

```
.
├── app: 實際啟動的server
│
├── domain: domain interface與entity，不具邏輯，所有層都靠此domain interface傳輸entity進行溝通
│
├ - - - - - - - - - - - - - - - - - - - - - - - - - -┐
├── auth: 驗證服務                   依照domain實作的服務
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

切出每個 domain 的邊界，此 monorepo 可先以 monolithic 部署，如果未來有 horizontal scaling 需求，再以 domain 來 deliver 給不同 microservice，避免一開始就使用 microservice 過度設計。

* repository: 可輕鬆替換底層。repository 使用 mq 時，可採用`kit/mq/kafka`或`kit/mq/memory`，以應付不同場景或減低測試成本
* usecase: 可輕鬆替換repository。usecase 使用 repository 依照 domain interface 操作，如果要`memory`替換成`mysql`，只需實作出符合 interface 的 repository
* delivery: delivery 使用 usecase 依照 domain interface 操作，如果要`gin`替換成`go-kit` server，不會修改到業務邏輯

reuse 方便，application 可以從 DIP 不同 domain，來完成產品需求。例如: `app/exchange-gitbitex`是組合`auth`與`exchange`domain。

測試方便，以 testcontainers 測試，更貼近真實情境進行測試。

### 教學

Clean Architecture的細節介紹可看我寫的此教學[文章](https://blog.messfar.com/golang-%E7%B3%BB%E7%B5%B1%E8%A8%AD%E8%A8%88#f3f6d329435d4bceb50ec37bb4c36984)

### Q&A

* monorepo 雖然更新方便，但怎麼管理套件版本？
  * 可用`git tag`處理

## matching-system

![](./doc/exchange-arch.png)

將後端[exchange domain](https://github.com/superj80820/system-design/tree/master/exchange)與開源前端[gitbitex-web](https://github.com/gitbitex/gitbitex-web)串接

* 預覽網頁(❗僅用最低效能運行預覽，不是 production 運作規格): https://preview.exchange.messfar.com
* 可達到 100,000PRS。撮合引擎以記憶體計算
* 可回放事件。以 event sourcing 的方式實現，撮合引擎為讀取 event 的有限狀態機，可 warm backup 多台 server 聽取 event，來達到 high availability
* 可分散式。不同的domain可部署至不同機器

### 壓測:

![](https://raw.githubusercontent.com/superj80820/system-design/master/doc/exchange-stress-test.png)

單機啟動 server、mysql、kafka、redis、mongodb，並進行買賣單搓合，並以k6壓測:
* exchange 機器: EC2 c5.18xlarge
* k6 機器: EC2 m5.8xlarge
* RPS (max): 102,988.52

如果將 mysql 或 kafka 等服務獨立出來，理論上可用更便宜的機器

### 教學

`Sequence 定序模組`、`Asset 資產模組`、`Order 訂單模組`、`Matching 撮合模組`、`Clearing 清算模組`如何設計的教學，撰寫於我的部落格[文章](https://blog.messfar.com/golang-%E7%B3%BB%E7%B5%B1%E8%A8%AD%E8%A8%88#378531212808413583831bc7c0b8cbe1)

### 運行

* require:
  * golang v1.20
  * docker

* development:
```
$ cd app/exchange-gitbitex
$ make dev
```
