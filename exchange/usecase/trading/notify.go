package trading

import (
	"context"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type tradingNotifyUseCase struct {
	logger                      loggerKit.Logger
	userAssetNotifyRepo         domain.UserAssetNotifyRepo
	quotationNotifyRepo         domain.QuotationNotifyRepo
	candleNotifyRepo            domain.CandleNotifyRepo
	matchingOrderBookNotifyRepo domain.MatchingOrderBookNotifyRepo
	matchingUseCase             domain.MatchingUseCase
	currencyUseCase             domain.CurrencyUseCase
	matchingNotifyRepo          domain.MatchingNotifyRepo
	OrderNotifyRepo             domain.OrderNotifyRepo

	orderBookMaxDepth int
	errLock           *sync.Mutex
	doneCh            chan struct{}
	err               error
}

func CreateTradingNotifyUseCase(
	ctx context.Context,
	matchingOrderBookNotifyRepo domain.MatchingOrderBookNotifyRepo,
	candleNotifyRepo domain.CandleNotifyRepo,
	userAssetNotifyRepo domain.UserAssetNotifyRepo,
	quotationNotifyRepo domain.QuotationNotifyRepo,
	OrderNotifyRepo domain.OrderNotifyRepo,
	matchingNotifyRepo domain.MatchingNotifyRepo,
	currencyUseCase domain.CurrencyUseCase,
	matchingUseCase domain.MatchingUseCase,
	orderBookMaxDepth int,
	logger loggerKit.Logger,
) domain.TradingNotifyUseCase {
	return &tradingNotifyUseCase{
		logger:                      logger,
		matchingOrderBookNotifyRepo: matchingOrderBookNotifyRepo,
		userAssetNotifyRepo:         userAssetNotifyRepo,
		quotationNotifyRepo:         quotationNotifyRepo,
		candleNotifyRepo:            candleNotifyRepo,
		matchingUseCase:             matchingUseCase,
		matchingNotifyRepo:          matchingNotifyRepo,
		OrderNotifyRepo:             OrderNotifyRepo,
		currencyUseCase:             currencyUseCase,

		orderBookMaxDepth: orderBookMaxDepth,
		errLock:           new(sync.Mutex),
		doneCh:            make(chan struct{}),
	}
}

// TODO: error handle when consume failed
func (t *tradingNotifyUseCase) NotifyForPublic(ctx context.Context, stream domain.TradingNotifyStream) error {
	consumeKey := utilKit.GetSnowflakeIDString()

	defer t.matchingNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.matchingOrderBookNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.candleNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.quotationNotifyRepo.StopConsume(ctx, consumeKey)

	l2OrderBook, err := t.matchingUseCase.GetHistoryL2OrderBook(ctx, t.orderBookMaxDepth)
	if err != nil && !errors.Is(err, domain.ErrNoData) {
		return errors.Wrap(err, "get history l2 order book failed")
	} else if err == nil {
		stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: l2OrderBook,
		})
	}

	t.matchingOrderBookNotifyRepo.ConsumeL2OrderBookWithRingBuffer(ctx, consumeKey, func(l2OrderBook *domain.OrderBookL2Entity) error {
		if err := stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: l2OrderBook,
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
				t.quotationNotifyRepo.ConsumeTicksMQWithRingBuffer(ctx, consumeKey, func(sequenceID int, ticks []*domain.TickEntity) error {
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
				t.candleNotifyRepo.ConsumeCandleMQ(ctx, consumeKey, func(candleBar *domain.CandleBar) error {
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
				t.matchingNotifyRepo.ConsumeMatchOrderMQBatch(ctx, consumeKey, func(matchOrderDetails []*domain.MatchOrderDetail) error {
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
				t.OrderNotifyRepo.ConsumeOrderMQ(ctx, consumeKey, func(sequenceID int, orders []*domain.OrderEntity) error {
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
func (t *tradingNotifyUseCase) NotifyForUser(ctx context.Context, userID int, stream domain.TradingNotifyStream) error {
	consumeKey := strconv.Itoa(userID) + "-" + utilKit.GetSnowflakeIDString()

	defer t.matchingNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.matchingOrderBookNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.candleNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.userAssetNotifyRepo.StopConsume(ctx, consumeKey)
	defer t.quotationNotifyRepo.StopConsume(ctx, consumeKey)

	l2OrderBook, err := t.matchingUseCase.GetHistoryL2OrderBook(ctx, t.orderBookMaxDepth)
	if err != nil && !errors.Is(err, domain.ErrNoData) {
		return errors.Wrap(err, "get history l2 order book failed")
	} else if err == nil {
		stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: l2OrderBook,
		})
	}

	t.matchingOrderBookNotifyRepo.ConsumeL2OrderBookWithRingBuffer(ctx, consumeKey, func(l2OrderBook *domain.OrderBookL2Entity) error {
		if err := stream.Send(domain.TradingNotifyResponse{
			Type:      domain.OrderBookExchangeResponseType,
			ProductID: t.currencyUseCase.GetProductID(),
			OrderBook: l2OrderBook,
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
				t.quotationNotifyRepo.ConsumeTicksMQWithRingBuffer(ctx, consumeKey, func(sequenceID int, ticks []*domain.TickEntity) error {
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
				t.userAssetNotifyRepo.ConsumeUsersAssetsWithRingBuffer(ctx, consumeKey, func(sequenceID int, usersAssets []*domain.UserAsset) error {
					for _, userAsset := range usersAssets {
						if userID != userAsset.UserID {
							continue
						}

						currencyCode, err := t.currencyUseCase.GetCurrencyUpperNameByType(domain.CurrencyType(userAsset.AssetID))
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
					}
					return nil
				})
				isConsumeFounds = true
			case domain.CandlesExchangeRequestType:
				if isConsumeCandle {
					continue
				}
				t.candleNotifyRepo.ConsumeCandleMQ(ctx, consumeKey, func(candleBar *domain.CandleBar) error {
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
				t.matchingNotifyRepo.ConsumeMatchOrderMQBatch(ctx, consumeKey, func(matchOrderDetails []*domain.MatchOrderDetail) error {
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
				t.OrderNotifyRepo.ConsumeOrderMQ(ctx, consumeKey, func(sequenceID int, orders []*domain.OrderEntity) error {
					for _, order := range orders {
						if order.UserID != userID {
							continue
						}
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

func (t *tradingNotifyUseCase) Done() <-chan struct{} {
	return t.doneCh
}

func (t *tradingNotifyUseCase) Err() error {
	t.errLock.Lock()
	defer t.errLock.Unlock()
	return t.err
}

func (t *tradingNotifyUseCase) Shutdown() error {
	panic("need ctx implement")
	<-t.doneCh
	return t.err
}
