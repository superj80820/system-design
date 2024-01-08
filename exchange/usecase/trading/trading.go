package trading

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type tradingUseCase struct {
	matchingUseCase domain.MatchingUseCase
}

func CreateOrderUseCase(matchingUseCase domain.MatchingUseCase) domain.TradingUseCase {
	return &tradingUseCase{
		matchingUseCase: matchingUseCase,
	}
}

func (m *tradingUseCase) CancelOrder(ts time.Time, o *domain.Order) error {
	panic("")
}

func (m *tradingUseCase) NewOrder(o *domain.Order) (*domain.MatchResult, error) {
	panic("")
	// matchResult, err := m.matchingUseCase.NewOrder(o)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "process order failed")
	// }
	// return matchResult, nil
}

func (m *tradingUseCase) GetMarketPrice() decimal.Decimal {
	panic("")
}

func (m *tradingUseCase) GetLatestSequenceID() int {
	panic("")
}
