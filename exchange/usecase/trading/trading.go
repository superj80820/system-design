package trading

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type batchEventsStruct struct {
	tradingEvent   []*domain.TradingEvent
	sequencerEvent []*domain.SequencerEvent
	commitFns      []func() error
}

type tradingUseCase struct {
	logger             loggerKit.Logger
	userAssetUseCase   domain.UserAssetUseCase
	userAssetRepo      domain.UserAssetRepo
	tradingRepo        domain.TradingRepo
	sequencerRepo      domain.SequencerRepo[domain.TradingEvent]
	candleRepo         domain.CandleRepo
	matchingUseCase    domain.MatchingUseCase
	syncTradingUseCase domain.SyncTradingUseCase
	orderUseCase       domain.OrderUseCase
	quotationRepo      domain.QuotationRepo
	currencyUseCase    domain.CurrencyUseCase
	matchingRepo       domain.MatchingRepo
	orderRepo          domain.OrderRepo

	batchEventsSize     int
	batchEventsDuration time.Duration
	lastTimestamp       time.Time

	orderBookDepth int
	cancel         context.CancelFunc
	lock           *sync.Mutex
	doneCh         chan struct{}
	err            error
}

func CreateTradingUseCase(
	ctx context.Context,
	tradingRepo domain.TradingRepo,
	matchingRepo domain.MatchingRepo,
	quotationRepo domain.QuotationRepo,
	candleRepo domain.CandleRepo,
	orderRepo domain.OrderRepo,
	userAssetRepo domain.UserAssetRepo,
	sequencerRepo domain.SequencerRepo[domain.TradingEvent],
	orderUseCase domain.OrderUseCase,
	userAssetUseCase domain.UserAssetUseCase,
	syncTradingUseCase domain.SyncTradingUseCase,
	matchingUseCase domain.MatchingUseCase,
	currencyUseCase domain.CurrencyUseCase,
	orderBookDepth int,
	logger loggerKit.Logger,
	batchEventsSize int,
	batchEventsDuration time.Duration,
) domain.TradingUseCase {
	ctx, cancel := context.WithCancel(ctx)

	t := &tradingUseCase{
		logger:             logger,
		tradingRepo:        tradingRepo,
		matchingRepo:       matchingRepo,
		quotationRepo:      quotationRepo,
		candleRepo:         candleRepo,
		orderRepo:          orderRepo,
		sequencerRepo:      sequencerRepo,
		userAssetRepo:      userAssetRepo,
		orderUseCase:       orderUseCase,
		syncTradingUseCase: syncTradingUseCase,
		matchingUseCase:    matchingUseCase,
		userAssetUseCase:   userAssetUseCase,
		currencyUseCase:    currencyUseCase,

		batchEventsSize:     batchEventsSize,
		batchEventsDuration: batchEventsDuration,

		orderBookDepth: orderBookDepth,
		cancel:         cancel,
		lock:           new(sync.Mutex),
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
			} else if errors.Is(err, domain.ErrNoOrder) {
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
			err := syncTradingUseCase.Deposit(ctx, te)
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

func (t *tradingUseCase) EnableBackupSnapshot(ctx context.Context, duration time.Duration) {
	setErrAndDone := func(err error) {
		t.lock.Lock()
		defer t.lock.Unlock()
		t.err = err
		close(t.doneCh)
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		snapshotSequenceID := t.syncTradingUseCase.GetSequenceID()

		for range ticker.C {
			if err := t.sequencerRepo.Pause(); err != nil {
				setErrAndDone(errors.Wrap(err, "pause failed"))
				return
			}
			sequenceID := t.syncTradingUseCase.GetSequenceID()
			if snapshotSequenceID == sequenceID {
				if err := t.sequencerRepo.Continue(); err != nil {
					setErrAndDone(errors.Wrap(err, "continue failed"))
					return
				}
				continue
			}
			snapshot, err := t.GetLatestSnapshot(ctx)

			if errors.Is(err, domain.ErrNoop) {
				continue
			} else if err != nil {
				setErrAndDone(errors.Wrap(err, "get snapshot failed"))
				return
			}
			if err := t.sequencerRepo.Continue(); err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			if err = t.SaveSnapshot(ctx, snapshot); !errors.Is(err, domain.ErrDuplicate) && err != nil {
				setErrAndDone(errors.Wrap(err, "continue failed"))
				return
			}
			snapshotSequenceID = sequenceID
		}
	}()
}

func (t *tradingUseCase) ConsumeTradingEventThenProduce(ctx context.Context) {
	var batchEvents batchEventsStruct
	setErrAndDone := func(err error) {
		t.lock.Lock()
		defer t.lock.Unlock()
		t.err = err
		close(t.doneCh)
	}
	eventsFullCh := make(chan struct{})
	lock := new(sync.Mutex)
	sequenceMessageFn := func(tradingEvent *domain.TradingEvent, commitFn func() error) {
		timeNow := time.Now()
		if timeNow.Before(t.lastTimestamp) {
			setErrAndDone(errors.New("now time is before last timestamp"))
			return
		}
		t.lastTimestamp = timeNow

		// sequence event
		previousID := t.sequencerRepo.GetCurrentSequenceID()
		previousIDInt, err := utilKit.SafeUint64ToInt(previousID)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "uint64 to int overflow"))
			return
		}
		sequenceID := t.sequencerRepo.GenerateNextSequenceID()
		sequenceIDInt, err := utilKit.SafeUint64ToInt(sequenceID)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "uint64 to int overflow"))
			return
		}
		tradingEvent.PreviousID = previousIDInt
		tradingEvent.SequenceID = sequenceIDInt
		tradingEvent.CreatedAt = t.lastTimestamp

		marshalData, err := json.Marshal(*tradingEvent)
		if err != nil {
			setErrAndDone(errors.Wrap(err, "marshal failed"))
			return
		}

		lock.Lock()
		batchEvents.sequencerEvent = append(batchEvents.sequencerEvent, &domain.SequencerEvent{
			ReferenceID: int64(tradingEvent.ReferenceID),
			SequenceID:  int64(tradingEvent.SequenceID),
			PreviousID:  int64(tradingEvent.PreviousID),
			Data:        string(marshalData),
			CreatedAt:   time.Now(),
		})
		batchEvents.tradingEvent = append(batchEvents.tradingEvent, tradingEvent)
		batchEvents.commitFns = append(batchEvents.commitFns, commitFn)
		batchEventsLength := len(batchEvents.sequencerEvent)
		lock.Unlock()

		if batchEventsLength >= t.batchEventsSize {
			eventsFullCh <- struct{}{}
		}
	}

	t.sequencerRepo.SubscribeTradeSequenceMessage(sequenceMessageFn)

	go func() {
		ticker := time.NewTicker(t.batchEventsDuration)
		defer ticker.Stop()

		errContinue := errors.New("continue")
		fn := func() error {
			sequencerEventClone, tradingEventClone, latestCommitFn, err :=
				func() ([]*domain.SequencerEvent, []*domain.TradingEvent, func() error, error) {
					lock.Lock()
					defer lock.Unlock()

					if len(batchEvents.sequencerEvent) == 0 || len(batchEvents.tradingEvent) == 0 {
						return nil, nil, nil, errors.Wrap(errContinue, "event length is zero")
					}

					if len(batchEvents.sequencerEvent) != len(batchEvents.tradingEvent) {
						panic("except trading event and sequencer event length")
					}
					sequencerEventClone := make([]*domain.SequencerEvent, len(batchEvents.sequencerEvent))
					copy(sequencerEventClone, batchEvents.sequencerEvent)
					tradingEventClone := make([]*domain.TradingEvent, len(batchEvents.tradingEvent))
					copy(tradingEventClone, batchEvents.tradingEvent)
					latestCommitFn := batchEvents.commitFns[len(batchEvents.commitFns)-1]
					batchEvents.sequencerEvent = nil // reset
					batchEvents.tradingEvent = nil   // reset
					batchEvents.commitFns = nil

					return sequencerEventClone, tradingEventClone, latestCommitFn, nil
				}()
			if err != nil {
				return errors.Wrap(err, "clone events failed")
			}

			err = t.sequencerRepo.SaveEvents(sequencerEventClone)
			if mysqlErr, ok := ormKit.ConvertMySQLErr(err); ok && errors.Is(mysqlErr, ormKit.ErrDuplicatedKey) {
				// TODO: test
				// if duplicate, filter events then retry
				filterEventsMap, err := t.sequencerRepo.GetFilterEventsMap(sequencerEventClone)
				if err != nil {
					return errors.Wrap(err, "get filter events map failed")
				}
				var filterSequencerEventClone []*domain.SequencerEvent
				for _, val := range sequencerEventClone {
					if filterEventsMap[val.ReferenceID] {
						continue
					}
					filterSequencerEventClone = append(filterSequencerEventClone, val)
				}
				var filterTradingEventClone []*domain.TradingEvent
				for _, val := range tradingEventClone {
					if filterEventsMap[int64(val.ReferenceID)] {
						continue
					}
					filterTradingEventClone = append(filterTradingEventClone, val)
				}

				if len(filterSequencerEventClone) == 0 || len(filterTradingEventClone) == 0 {
					return nil
				}

				if len(batchEvents.sequencerEvent) != len(batchEvents.tradingEvent) {
					panic("except trading event and sequencer event length")
				}

				if err := t.sequencerRepo.SaveEvents(sequencerEventClone); err != nil {
					panic(errors.Wrap(err, "save event failed")) // TODO: use panic?
				}

				if err := latestCommitFn(); err != nil {
					return errors.Wrap(err, "commit latest message failed")
				}

				t.tradingRepo.SendTradeEvent(ctx, tradingEventClone)

				return nil
			} else if err != nil {
				panic(errors.Wrap(err, "save event failed")) // TODO: use panic?
			}

			if err := latestCommitFn(); err != nil {
				return errors.Wrap(err, "commit latest message failed")
			}

			t.tradingRepo.SendTradeEvent(ctx, tradingEventClone)

			return nil
		}
		for {
			select {
			case <-ticker.C:
				if err := fn(); errors.Is(err, errContinue) {
					continue
				} else if err != nil {
					setErrAndDone(errors.Wrap(err, "clone event failed"))
					return
				}
			case <-eventsFullCh:
				if err := fn(); errors.Is(err, errContinue) {
					continue
				} else if err != nil {
					setErrAndDone(errors.Wrap(err, "clone event failed"))
					return
				}
			}
		}
	}()
}

func (t *tradingUseCase) ProduceCancelOrderTradingEvent(ctx context.Context, userID, orderID int) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventCancelOrderType,
		OrderCancelEvent: &domain.OrderCancelEvent{
			UserID:  userID,
			OrderId: orderID,
		},
	}

	if err := t.sequencerRepo.SendTradeSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

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

func (t *tradingUseCase) ProduceDepositOrderTradingEvent(ctx context.Context, userID, assetID int, amount decimal.Decimal) (*domain.TradingEvent, error) {
	referenceID, err := utilKit.SafeInt64ToInt(utilKit.GetSnowflakeIDInt64())
	if err != nil {
		return nil, errors.Wrap(err, "safe int64 to int failed")
	}

	tradingEvent := &domain.TradingEvent{
		ReferenceID: referenceID,
		EventType:   domain.TradingEventDepositType,
		DepositEvent: &domain.DepositEvent{
			ToUserID: userID,
			AssetID:  assetID,
			Amount:   amount,
		},
	}

	if err := t.sequencerRepo.SendTradeSequenceMessages(ctx, tradingEvent); err != nil {
		return nil, errors.Wrap(err, "send trade sequence messages failed")
	}

	return tradingEvent, nil
}

func (t *tradingUseCase) GetHistoryMatchDetails(maxResults int) ([]*domain.MatchOrderDetail, error) {
	details, err := t.matchingRepo.GetMatchingHistory(maxResults)
	if err != nil {
		return nil, errors.Wrap(err, "get matching history failed")
	}
	return details, nil
}

func (t *tradingUseCase) GetUserHistoryMatchDetails(userID, orderID int) ([]*domain.MatchOrderDetail, error) {
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
func (t *tradingUseCase) NotifyForPublic(ctx context.Context, stream domain.TradingNotifyStream) error {
	consumeKey := utilKit.GetSnowflakeIDString()

	stream.Send(domain.TradingNotifyResponse{
		Type:      domain.OrderBookExchangeResponseType,
		ProductID: t.currencyUseCase.GetProductID(),
		OrderBook: t.matchingUseCase.GetOrderBook(100), // TODO: max depth
	})

	t.matchingRepo.ConsumeOrderBook(ctx, consumeKey, func(orderBook *domain.OrderBookEntity) error {
		if err := stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: orderBook,
		}); err != nil {
			return errors.Wrap(err, "send failed")
		}
		return nil
	})

	var (
		isConsumeTick   bool
		isConsumeMatch  bool
		isConsumeOrder  bool
		isConsumeCandle bool
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
				t.quotationRepo.ConsumeTicksMQ(ctx, consumeKey, func(sequenceID int, ticks []*domain.TickEntity) error {
					for _, tick := range ticks {
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:      domain.TickerExchangeResponseType,
							ProductID: t.currencyUseCase.GetProductID(),
							Tick:      tick,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeTick = true
			case domain.CandlesExchangeRequestType:
				if isConsumeCandle {
					continue
				}
				t.candleRepo.ConsumeCandleMQ(ctx, consumeKey, func(candleBar *domain.CandleBar) error {
					if err := stream.Send(domain.TradingNotifyResponse{
						Type:      domain.CandleExchangeResponseType,
						ProductID: t.currencyUseCase.GetProductID(),
						CandleBar: candleBar,
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}
					return nil
				})
				isConsumeCandle = true
			case domain.MatchExchangeRequestType:
				if isConsumeMatch {
					continue
				}
				t.matchingRepo.ConsumeMatchOrderMQBatch(ctx, consumeKey, func(matchOrderDetails []*domain.MatchOrderDetail) error {
					for _, matchOrderDetail := range matchOrderDetails {
						if matchOrderDetail.Type != domain.MatchTypeTaker {
							continue
						}
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:             domain.MatchExchangeResponseType,
							ProductID:        t.currencyUseCase.GetProductID(),
							MatchOrderDetail: matchOrderDetail,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeMatch = true
			case domain.OrderExchangeRequestType:
				if isConsumeOrder {
					continue
				}
				t.orderRepo.ConsumeOrderMQBatch(ctx, consumeKey, func(orders []*domain.OrderEntity) error {
					for _, order := range orders {
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:      domain.OrderExchangeResponseType,
							ProductID: t.currencyUseCase.GetProductID(),
							Order:     order,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeOrder = true
			case domain.PingExchangeRequestType:
				if err := stream.Send(domain.TradingNotifyResponse{
					Type:      domain.PongExchangeResponseType,
					ProductID: t.currencyUseCase.GetProductID(),
				}); err != nil {
					return errors.Wrap(err, "send failed")
				}
			}
		}
	}
}

// TODO: error handle when consume failed
func (t *tradingUseCase) NotifyForUser(ctx context.Context, userID int, stream domain.TradingNotifyStream) error {
	consumeKey := strconv.Itoa(userID) + "-" + utilKit.GetSnowflakeIDString()

	stream.Send(domain.TradingNotifyResponse{
		Type:      domain.OrderBookExchangeResponseType,
		ProductID: t.currencyUseCase.GetProductID(),
		OrderBook: t.matchingUseCase.GetOrderBook(100), // TODO: max depth
	})

	t.matchingRepo.ConsumeOrderBook(ctx, consumeKey, func(orderBook *domain.OrderBookEntity) error {
		if err := stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: orderBook,
		}); err != nil {
			return errors.Wrap(err, "send failed")
		}
		return nil
	})

	var (
		isConsumeTick   bool
		isConsumeMatch  bool
		isConsumeFounds bool
		isConsumeOrder  bool
		isConsumeCandle bool
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
				t.quotationRepo.ConsumeTicksMQ(ctx, consumeKey, func(sequenceID int, ticks []*domain.TickEntity) error {
					for _, tick := range ticks {
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:      domain.TickerExchangeResponseType,
							ProductID: t.currencyUseCase.GetProductID(),
							Tick:      tick,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeTick = true
			case domain.AssetsExchangeRequestType:
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

					if err := stream.Send(domain.TradingNotifyResponse{
						Type:      domain.AssetExchangeResponseType,
						ProductID: t.currencyUseCase.GetProductID(),
						UserAsset: &domain.TradingNotifyAsset{
							UserAsset:    userAsset,
							CurrencyName: currencyCode,
						},
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}

					return nil
				})
				isConsumeFounds = true
			case domain.CandlesExchangeRequestType:
				if isConsumeCandle {
					continue
				}
				t.candleRepo.ConsumeCandleMQ(ctx, consumeKey, func(candleBar *domain.CandleBar) error {
					if err := stream.Send(domain.TradingNotifyResponse{
						Type:      domain.CandleExchangeResponseType,
						ProductID: t.currencyUseCase.GetProductID(),
						CandleBar: candleBar,
					}); err != nil {
						return errors.Wrap(err, "send failed")
					}
					return nil
				})
				isConsumeCandle = true
			case domain.MatchExchangeRequestType:
				if isConsumeMatch {
					continue
				}
				t.matchingRepo.ConsumeMatchOrderMQBatch(ctx, consumeKey, func(matchOrderDetails []*domain.MatchOrderDetail) error {
					for _, matchOrderDetail := range matchOrderDetails {
						if matchOrderDetail.Type != domain.MatchTypeTaker {
							continue
						}
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:             domain.MatchExchangeResponseType,
							ProductID:        t.currencyUseCase.GetProductID(),
							MatchOrderDetail: matchOrderDetail,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeMatch = true
			case domain.OrderExchangeRequestType:
				if isConsumeOrder {
					continue
				}
				t.orderRepo.ConsumeOrderMQBatch(ctx, consumeKey, func(orders []*domain.OrderEntity) error {
					for _, order := range orders {
						if err := stream.Send(domain.TradingNotifyResponse{
							Type:      domain.OrderExchangeResponseType,
							ProductID: t.currencyUseCase.GetProductID(),
							Order:     order,
						}); err != nil {
							return errors.Wrap(err, "send failed")
						}
					}
					return nil
				})
				isConsumeOrder = true
			case domain.PingExchangeRequestType:
				if err := stream.Send(domain.TradingNotifyResponse{
					Type:      domain.PongExchangeResponseType,
					ProductID: t.currencyUseCase.GetProductID(),
				}); err != nil {
					return errors.Wrap(err, "send failed")
				}
			}
		}
	}
}

func (t *tradingUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingUseCase) Err() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.err
}

func (t *tradingUseCase) Shutdown() error {
	t.cancel()
	<-t.doneCh
	return t.err
}
