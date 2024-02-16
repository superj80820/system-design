package trading

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/core/endpoint"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type tradingUseCase struct {
	logger             loggerKit.Logger
	userAssetUseCase   domain.UserAssetUseCase
	userAssetRepo      domain.UserAssetRepo
	tradingRepo        domain.TradingRepo
	candleRepo         domain.CandleRepo
	matchingUseCase    domain.MatchingUseCase
	syncTradingUseCase domain.SyncTradingUseCase
	orderUseCase       domain.OrderUseCase
	quotationRepo      domain.QuotationRepo
	currencyUseCase    domain.CurrencyUseCase
	matchingRepo       domain.MatchingRepo
	orderRepo          domain.OrderRepo

	orderBookDepth int
	cancel         context.CancelFunc
	doneCh         chan struct{}
	err            error
}

// response example:
//
//	{
//		"close24h": "2",
//		"high24h": "2",
//		"lastSize": "1",
//		"low24h": "2",
//		"open24h": "2",
//		"price": "2",
//		"productId": "BTC-USDT",
//		"sequence": 0,
//		"side": "buy",
//		"time": "2024-02-05T10:41:10.564Z",
//		"tradeId": 1,
//		"type": "ticker",
//		"volume24h": "1",
//		"volume30d": "1"
//	  }
type tradingTickNotify struct {
	Close24H  string    `json:"close24h"`
	High24H   string    `json:"high24h"`
	LastSize  string    `json:"lastSize"`
	Low24H    string    `json:"low24h"`
	Open24H   string    `json:"open24h"`
	Price     string    `json:"price"`
	ProductID string    `json:"productId"`
	Sequence  int       `json:"sequence"`
	Side      string    `json:"side"`
	Time      time.Time `json:"time"`
	TradeID   int       `json:"tradeId"`
	Type      string    `json:"type"`
	Volume24H string    `json:"volume24h"`
	Volume30D string    `json:"volume30d"`
}

// response example:
//
//	{
//		"available": "999999999",
//		"currencyCode": "BTC",
//		"hold": "1",
//		"type": "funds",
//		"userId": "11fa31dd-4933-4caf-9c67-5787c9fe6f21"
//	}
type tradingFoundsNotify struct {
	Available    string `json:"available"`
	CurrencyCode string `json:"currencyCode"`
	Hold         string `json:"hold"`
	Type         string `json:"type"`
	UserID       string `json:"userId"`
}

// response example:
//
//	{
//		"makerOrderId": "b60cdaae-6d2b-4bc9-9bb9-92f0ac48d718",
//		"price": "29",
//		"productId": "BTC-USDT",
//		"sequence": 40,
//		"side": "buy",
//		"size": "1",
//		"takerOrderId": "d4d13109-a7a1-47ec-bc58-cb58c4840695",
//		"time": "2024-02-05T16:34:21.455Z",
//		"tradeId": 5,
//		"type": "match"
//	}
type tradingMatchNotify struct {
	MakerOrderID string    `json:"makerOrderId"`
	Price        string    `json:"price"`
	ProductID    string    `json:"productId"`
	Sequence     int       `json:"sequence"`
	Side         string    `json:"side"`
	Size         string    `json:"size"`
	TakerOrderID string    `json:"takerOrderId"`
	Time         time.Time `json:"time"`
	TradeID      int       `json:"tradeId"`
	Type         string    `json:"type"`
}

// response example:
//
//	{
//		"createdAt": "2024-02-05T16:34:13.488Z",
//		"executedValue": "29",
//		"fillFees": "0",
//		"filledSize": "1",
//		"funds": "29",
//		"id": "b60cdaae-6d2b-4bc9-9bb9-92f0ac48d718",
//		"orderType": "limit",
//		"price": "29",
//		"productId": "BTC-USDT",
//		"settled": false,
//		"side": "buy",
//		"size": "1",
//		"status": "filled",
//		"type": "order",
//		"userId": "11fa31dd-4933-4caf-9c67-5787c9fe6f21"
//	}
type tradingOrderNotify struct {
	CreatedAt     time.Time `json:"createdAt"`
	ExecutedValue string    `json:"executedValue"`
	FillFees      string    `json:"fillFees"`
	FilledSize    string    `json:"filledSize"`
	Funds         string    `json:"funds"`
	ID            string    `json:"id"`
	OrderType     string    `json:"orderType"`
	Price         string    `json:"price"`
	ProductID     string    `json:"productId"`
	Settled       bool      `json:"settled"`
	Side          string    `json:"side"`
	Size          string    `json:"size"`
	Status        string    `json:"status"`
	Type          string    `json:"type"`
	UserID        string    `json:"userId"`
}

type tradingOrderBookNotify struct {
	Asks      [][]float64 `json:"asks"`
	Bids      [][]float64 `json:"bids"`
	ProductID string      `json:"productId"`
	Sequence  int         `json:"sequence"`
	Time      int64       `json:"time"`
	Type      string      `json:"type"`
}

type tradingUserAsset struct {
	Available    string `json:"available"`
	CurrencyCode string `json:"currencyCode"`
	Hold         string `json:"hold"`
	Type         string `json:"type"`
	UserID       string `json:"userId"`
}

type tradingPongNotify struct {
	Type string `json:"type"`
}

func CreateTradingUseCase(
	ctx context.Context,
	tradingRepo domain.TradingRepo,
	matchingRepo domain.MatchingRepo,
	quotationRepo domain.QuotationRepo,
	candleRepo domain.CandleRepo,
	orderRepo domain.OrderRepo,
	userAssetRepo domain.UserAssetRepo,
	orderUseCase domain.OrderUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	syncTradingUseCase domain.SyncTradingUseCase,
	matchingUseCase domain.MatchingUseCase,
	currencyUseCase domain.CurrencyUseCase,
	orderBookDepth int,
	logger loggerKit.Logger,
) domain.TradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingUseCase{
		logger:             logger,
		tradingRepo:        tradingRepo,
		matchingRepo:       matchingRepo,
		quotationRepo:      quotationRepo,
		candleRepo:         candleRepo,
		orderRepo:          orderRepo,
		userAssetRepo:      userAssetRepo,
		orderUseCase:       orderUseCase,
		syncTradingUseCase: syncTradingUseCase,
		matchingUseCase:    matchingUseCase,
		userAssetUseCase:   userAssetUseCase,
		currencyUseCase:    currencyUseCase,

		orderBookDepth: orderBookDepth,
		cancel:         cancel,
		doneCh:         make(chan struct{}),
	}

	t.tradingRepo.SubscribeTradeEvent("global-trader", func(te *domain.TradingEvent) {
		switch te.EventType {
		case domain.TradingEventCreateOrderType:
			matchResult, err := syncTradingUseCase.CreateOrder(ctx, te)
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusCreate,
				TradingEvent:        te,
				MatchResult:         matchResult,
			})
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventCancelOrderType:
			err := syncTradingUseCase.CancelOrder(ctx, te)
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusCancel,
				TradingEvent:        te,
			})
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventTransferType:
			err := syncTradingUseCase.Transfer(ctx, te)
			if errors.Is(err, domain.LessAmountErr) {
				t.logger.Info(fmt.Sprintf("%+v", err))
				return
			} else if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusTransfer,
				TradingEvent:        te,
			})
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		case domain.TradingEventDepositType:
			err := t.Deposit(ctx, te)
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}

			err = t.tradingRepo.SendTradingResult(ctx, &domain.TradingResult{
				TradingResultStatus: domain.TradingResultStatusDeposit,
				TradingEvent:        te,
			})
			if err != nil {
				panic(fmt.Sprintf("process message get error: %+v", err))
			}
		default:
			panic(errors.New("unknown event type"))
		}
	})

	return t
}

func (t *tradingUseCase) CancelOrder(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	err := t.syncTradingUseCase.CancelOrder(ctx, tradingEvent)
	if err != nil {
		return errors.Wrap(err, "cancel order failed")
	}
	return nil
}

func (t *tradingUseCase) CreateOrder(ctx context.Context, tradingEvent *domain.TradingEvent) (*domain.MatchResult, error) {
	matchResult, err := t.syncTradingUseCase.CreateOrder(ctx, tradingEvent)
	if err != nil {
		return nil, errors.Wrap(err, "create order failed")
	}
	return matchResult, nil
}

func (t *tradingUseCase) Transfer(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	err := t.syncTradingUseCase.Transfer(ctx, tradingEvent)
	if err != nil {
		return errors.Wrap(err, "transfer failed")
	}
	return nil
}

func (t *tradingUseCase) Deposit(ctx context.Context, tradingEvent *domain.TradingEvent) error {
	err := t.syncTradingUseCase.Deposit(ctx, tradingEvent)
	if err != nil {
		return errors.Wrap(err, "deposit failed")
	}
	return nil
}

func (t *tradingUseCase) GetHistoryMatchDetails(userID, orderID int) ([]*domain.MatchOrderDetail, error) {
	order, err := t.orderUseCase.GetOrder(orderID)
	if errors.Is(err, domain.ErrNoOrder) {
		_, err := t.orderUseCase.GetHistoryOrder(userID, orderID)
		if errors.Is(err, domain.ErrNoOrder) {
			return nil, errors.New("order not found")
		} else if err != nil {
			return nil, errors.Wrap(err, "get order failed")
		}
	} else if err != nil {
		return nil, errors.Wrap(err, "get order failed")
	} else {
		if userID != order.UserID {
			return nil, errors.New("order not found")
		}
	}

	matchOrderDetails, err := t.matchingRepo.GetMatchingDetails(orderID)
	if err != nil {
		return nil, errors.Wrap(err, "get matching details failed")
	}
	return matchOrderDetails, nil
}

func (t *tradingUseCase) GetSequenceID() int {
	return t.syncTradingUseCase.GetSequenceID()
}

func (t *tradingUseCase) GetLatestSnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	sequenceID := t.syncTradingUseCase.GetSequenceID()
	usersAssetsData, err := t.userAssetUseCase.GetUsersAssetsData()
	if err != nil {
		return nil, errors.Wrap(err, "get all users assets failed")
	}
	ordersData, err := t.orderUseCase.GetOrdersData()
	if err != nil {
		return nil, errors.Wrap(err, "get all orders failed")
	}
	matchesData, err := t.matchingUseCase.GetMatchesData()
	if err != nil {
		return nil, errors.Wrap(err, "get all matches failed")
	}
	return &domain.TradingSnapshot{
		SequenceID:  sequenceID,
		UsersAssets: usersAssetsData,
		Orders:      ordersData,
		MatchData:   matchesData,
	}, nil
}

func (t *tradingUseCase) GetHistorySnapshot(ctx context.Context) (*domain.TradingSnapshot, error) {
	historySnapshot, err := t.tradingRepo.GetHistorySnapshot(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get history snapshot failed")
	}
	return historySnapshot, nil
}

func (t *tradingUseCase) RecoverBySnapshot(tradingSnapshot *domain.TradingSnapshot) error {
	if err := t.syncTradingUseCase.RecoverBySnapshot(tradingSnapshot); err != nil {
		return errors.Wrap(err, "recover by snapshot failed")
	}
	return nil
}

func (t *tradingUseCase) SaveSnapshot(ctx context.Context, tradingSnapshot *domain.TradingSnapshot) error {
	if err := t.tradingRepo.SaveSnapshot(
		ctx,
		tradingSnapshot.SequenceID,
		tradingSnapshot.UsersAssets,
		tradingSnapshot.Orders,
		tradingSnapshot.MatchData,
	); err != nil {
		return errors.Wrap(err, "save snapshot failed")
	}
	return nil
}

// TODO: error handle when consume failed
func (t *tradingUseCase) Notify(ctx context.Context, userID int, stream endpoint.Stream[domain.TradingNotifyRequest, any]) error {
	consumeKey := strconv.Itoa(userID) + "-" + utilKit.GetSnowflakeIDString()

	notifyOrderBookFn := func(orderBook *domain.OrderBookEntity) error {
		orderBookNotify := tradingOrderBookNotify{
			ProductID: t.currencyUseCase.GetProductID(),
			Sequence:  orderBook.SequenceID,
			Time:      time.Now().UnixMilli(),
			Type:      "snapshot",           // TODO: workaround
			Bids:      make([][]float64, 0), // TODO: client need zero value
			Asks:      make([][]float64, 0), // TODO: client need zero value
		}
		for _, val := range orderBook.Buy {
			orderBookNotify.Bids = append(orderBookNotify.Bids, []float64{val.Price.InexactFloat64(), val.Quantity.InexactFloat64(), 1}) // TODO: 1 is workaround
		}
		for _, val := range orderBook.Sell {
			orderBookNotify.Asks = append(orderBookNotify.Asks, []float64{val.Price.InexactFloat64(), val.Quantity.InexactFloat64(), 1}) // TODO: 1 is workaround
		}
		if err := stream.Send(&orderBookNotify); err != nil {
			return errors.Wrap(err, "send failed")
		}
		return nil
	}

	notifyOrderBookFn(t.matchingUseCase.GetOrderBook(100)) // TODO: max depth

	t.matchingRepo.ConsumeOrderBook(ctx, consumeKey, notifyOrderBookFn)

	var (
		isConsumeTick       bool
		isConsumeMatch      bool
		isConsumeFounds     bool
		isConsumeOrder      bool
		isConsumeCandleMin  atomic.Bool
		isConsumeCandleHour atomic.Bool
		isConsumeCandleDay  atomic.Bool
	)
	for {
		receive, err := stream.Recv()
		if err != nil {
			return errors.Wrap(err, "receive failed")
		}
		for _, channel := range receive.Channels {
			switch domain.ExchangeRequestType(channel) {
			case domain.TickerExchangeRequestType:
				if isConsumeTick {
					continue
				}
				t.quotationRepo.ConsumeTicks(ctx, consumeKey, func(sequenceID int, ticks []*domain.TickEntity) error {
					for _, tick := range ticks {
						if err := stream.Send(&tradingTickNotify{
							// Close24H  : TODO
							// High24H   : TODO
							// LastSize  : TODO
							// Low24H    : TODO
							// Open24H   : TODO
							Price:     tick.Price.String(),
							ProductID: t.currencyUseCase.GetProductID(),
							Sequence:  sequenceID,
							Side:      tick.TakerDirection.String(),
							Time:      tick.CreatedAt,
							// TradeID   : TODO
							Type: string(domain.TickerExchangeRequestType),
							// Volume24H : TODO
							// Volume30D : TODO
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeTick = true
			case domain.FoundsExchangeRequestType:
				if isConsumeFounds {
					continue
				}
				t.userAssetRepo.ConsumeUserAsset(ctx, consumeKey, func(notifyUserID, assetID int, userAsset *domain.UserAsset) error {
					if userID != notifyUserID {
						return nil
					}

					currencyCode, err := t.currencyUseCase.GetCurrencyUpperNameByType(domain.CurrencyType(assetID))
					if err != nil {
						return errors.Wrap(err, "get currency upper name by type failed")
					}

					if err := stream.Send(tradingFoundsNotify{
						Available:    userAsset.Available.String(),
						CurrencyCode: currencyCode,
						Hold:         userAsset.Frozen.String(),
						Type:         string(domain.FoundsExchangeRequestType),
						UserID:       strconv.Itoa(notifyUserID),
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}

					return nil
				})
				isConsumeFounds = true
			case domain.CandlesExchangeRequestType: // TODO: what this?
			case domain.Candles60ExchangeRequestType, domain.Candles3600ExchangeRequestType, domain.Candles86400ExchangeRequestType:
				if isConsumeCandleMin.Load() && isConsumeCandleHour.Load() && isConsumeCandleDay.Load() {
					continue
				}
				// if !(isConsumeCandleMin.Load() || isConsumeCandleHour.Load() || isConsumeCandleDay.Load()) {
				// 	t.candleRepo.ConsumeCandle(ctx, consumeKey, func(candleBar *domain.CandleBar) error {
				// 		if candleBar.Type == domain.CandleTimeTypeMin && isConsumeCandleMin.Load() ||
				// 			candleBar.Type == domain.CandleTimeTypeHour && isConsumeCandleHour.Load() ||
				// 			candleBar.Type == domain.CandleTimeTypeDay && isConsumeCandleDay.Load() {
				// 			if err := stream.Send(candleBar); err != nil {
				// 				return errors.Wrap(err, "send failed")
				// 			}
				// 		}
				// 		return nil
				// 	})
				// }
				if domain.ExchangeRequestType(channel) == domain.Candles60ExchangeRequestType {
					isConsumeCandleMin.Store(true)
				} else if domain.ExchangeRequestType(channel) == domain.Candles3600ExchangeRequestType {
					isConsumeCandleHour.Store(true)
				} else if domain.ExchangeRequestType(channel) == domain.Candles86400ExchangeRequestType {
					isConsumeCandleDay.Store(true)
				}
			case domain.MatchExchangeRequestType:
				if isConsumeMatch {
					continue
				}
				t.matchingRepo.ConsumeMatchOrder(ctx, consumeKey, func(matchOrderDetail *domain.MatchOrderDetail) error {
					if matchOrderDetail.Type != domain.MatchTypeTaker {
						return nil
					}
					if err := stream.Send(&tradingMatchNotify{
						// MakerOrderID : TODO
						Price: matchOrderDetail.Price.String(),
						// ProductID    : TODO
						Sequence:     matchOrderDetail.SequenceID,
						Side:         matchOrderDetail.Direction.String(),
						Size:         matchOrderDetail.Quantity.String(),
						TakerOrderID: strconv.Itoa(matchOrderDetail.UserID),
						Time:         matchOrderDetail.CreatedAt,
						// TradeID      : TODO
						Type: string(domain.MatchExchangeRequestType),
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}
					return nil
				})
				isConsumeMatch = true
			case domain.Level2ExchangeRequestType:
			case domain.OrderExchangeRequestType:
				if isConsumeOrder {
					continue
				}
				t.orderRepo.ConsumeOrder(ctx, consumeKey, func(order *domain.OrderEntity) error {
					if err := stream.Send(&tradingOrderNotify{
						CreatedAt:     order.CreatedAt,
						ExecutedValue: order.Quantity.Sub(order.UnfilledQuantity).Mul(order.Price).String(),
						FillFees:      "0",
						FilledSize:    order.Quantity.Sub(order.UnfilledQuantity).String(),
						Funds:         order.Quantity.Mul(order.Price).String(),
						ID:            strconv.Itoa(order.ID),
						OrderType:     "limit",
						Price:         order.Price.String(),
						ProductID:     t.currencyUseCase.GetProductID(),
						// Settled       : TODO: what this?
						Side: order.Direction.String(),
						Size: order.Quantity.String(),
						Status: func(status string) string {
							if status == "pending" {
								return "open"
							} else if status == "fully-filled" {
								return "filled"
							}
							return status
						}(order.Status.String()),
						Type:   string(domain.OrderExchangeRequestType),
						UserID: strconv.Itoa(order.UserID),
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}
					return nil
				})
				isConsumeOrder = true
			case domain.PingExchangeRequestType:
				if err := stream.Send(&tradingPongNotify{Type: "pong"}); err != nil {
					return errors.Wrap(err, "send failed")
				}
			}
		}
	}
}

func (t *tradingUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingUseCase) Shutdown() error {
	t.cancel()
	<-t.doneCh
	return t.err
}
