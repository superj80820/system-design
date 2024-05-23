# system-design

```
.
├── app: 實際啟動的server
│
├── domain: domain interface與entity，不具邏輯，所有層都靠此domain interface傳輸entity進行溝通
│
├ - - - - - - - - - - - - - - - - - - - - - - - - - -┐
├── auth: 驗證服務                   依照domain實作的服務
├── actress: 人臉辨識服務
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

## clean-architecture

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) Clean Architecture，每個 domain 依 repository、usecase、delivery 三層設計



切出每個 domain 的邊界，此 monorepo 可先以 monolithic 部署，如果未來有 horizontal scaling 需求，再以 domain 來 deliver 給不同 microservice，避免一開始就使用 microservice 過度設計。

* repository: 可輕鬆替換底層。repository 使用 mq 時，可採用`kit/mq/kafka`或`kit/mq/memory`，以應付不同場景或減低測試成本
* usecase: 可輕鬆替換repository。usecase 使用 repository 依照 domain interface 操作，如果要`memory`替換成`mysql`，只需實作出符合 interface 的 repository
* delivery: delivery 使用 usecase 依照 domain interface 操作，如果要`gin`替換成`go-kit` server，不會修改到業務邏輯

reuse 方便，application 可以從 DIP 不同 domain，來完成產品需求。例如: `app/exchange-gitbitex`是組合`auth`與`exchange`domain。

測試方便，以 testcontainers 測試，更貼近真實情境進行測試。

### 教學

* [你的 Backend 可以更有彈性一點 - Clean Architecture 概念篇](https://blog.messfar.com/post/k8s-note/k8s-note-clean-architecture-part1)
* [奔放的 Golang，卻隱藏著有紀律的架構！ - Clean Architecture 實作篇](https://blog.messfar.com/post/k8s-note/k8s-note-clean-architecture-part2)
* [讓你的 Backend 萬物皆虛，萬事皆可測 - Clean Architecture 測試篇](https://blog.messfar.com/post/k8s-note/k8s-note-clean-architecture-part3)

### Q&A

* monorepo 雖然更新方便，但怎麼管理套件版本？
  * 可用`git tag`處理

## matching-system

![](./doc/exchange-arch.png)

後端[exchange](https://github.com/superj80820/system-design/tree/master/exchange)與開源前端[gitbitex-web](https://github.com/gitbitex/gitbitex-web)整合

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

1. [如何設計一個撮合系統](https://blog.messfar.com/post/system-design/system-design-1-architecture)
2. [Sequence 定序模組](https://blog.messfar.com/post/system-design/system-design-2-sequence)
3. [Asset 資產模組](https://blog.messfar.com/post/system-design/system-design-3-asset)
4. [Order 訂單模組](https://blog.messfar.com/post/system-design/system-design-4-order)
5. [Matching 撮合模組](https://blog.messfar.com/post/system-design/system-design-5-matching)
6. [Clearing 清算模組](https://blog.messfar.com/post/system-design/system-design-6-clearing)
7. [整合撮合系統](https://blog.messfar.com/post/system-design/system-design-7-integration)

### 運行

* require:
  * golang v1.20
  * docker

* development:
```
$ cd app/exchange-gitbitex
$ make dev
```
