# system-design

![](https://raw.githubusercontent.com/bxcodec/go-clean-arch/master/clean-arch.png)

依照[go-clean-arch-v3](https://github.com/bxcodec/go-clean-arch/tree/v3) clean architecture，每個 domain 依 repository、usecase、delivery 三層設計

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

## 教學

撰寫於[部落格](https://blog.messfar.com/golang-%E7%B3%BB%E7%B5%B1%E8%A8%AD%E8%A8%88#04041b7b152746549eda5de6e1180a5d)

## exchange

![](./doc/exchange-arch.png)

將後端[exchange domain](https://github.com/superj80820/system-design/tree/master/exchange)與開源前端[gitbitex-web](https://github.com/gitbitex/gitbitex-web)串接

* 預覽網頁(❗僅用最低效能運行預覽，不是 production 運作規格): https://preview.exchange.messfar.com
* 可達到 100,000PRS。撮合引擎以記憶體計算
* 可回放事件。以 event sourcing 的方式實現，撮合引擎為讀取 event 的有限狀態機，可熱備援多台 server 聽取 event，來達到 high availability
* 可分散式。不同的domain可部署至不同機器

## 壓測:

![](https://raw.githubusercontent.com/superj80820/system-design/master/doc/exchange-stress-test.png)

單機啟動 server、mysql、kafka、redis、mongodb，並進行買賣單搓合，並以k6壓測:
* exchange 機器: EC2 c5.18xlarge
* k6 機器: EC2 m5.8xlarge
* RPS (max): 102,988.52

如果將 mysql 或 kafka 等服務獨立出來，理論上可用更便宜的機器

### 教學

撰寫於[部落格](https://blog.messfar.com/golang-%E7%B3%BB%E7%B5%B1%E8%A8%AD%E8%A8%88#11d29f38617742a197259aa928ce8a0f)

### 運行

* require:
  * golang v1.20
  * docker

* development:
```
$ cd app/exchange-gitbitex
$ make dev
```
