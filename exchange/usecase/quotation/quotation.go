package quotation

import (
	"container/list"
	"strconv"
	"sync"
	"time"

	"github.com/superj80820/system-design/domain"
)

type quotationUseCase struct {
	tradingRepo        domain.TradingRepo
	tradingResults     []*domain.TradingResult
	tradingResultsLock *sync.Mutex
	tickLock           *sync.Mutex
	tickList           *list.List
	cap                int
}

type tick struct {
	domain.TickEntity
}

func (t *tick) String() string {
	direction := "0" // sell // TODO: why
	if t.TakerDirection == domain.DirectionBuy {
		direction = "1" // buy // TODO: why
	}
	return "[" + strconv.FormatInt(t.CreatedAt.UnixMilli(), 10) + "," + direction + "," + t.Price.String() + "," + t.Quantity.String() + "]"
}

func CreateQuotationUseCase(tradingRepo domain.TradingRepo, cap int) domain.QuotationUseCase {
	q := &quotationUseCase{
		tradingRepo:        tradingRepo,
		tickList:           list.New(),
		cap:                cap,
		tradingResultsLock: new(sync.Mutex),
		tickLock:           new(sync.Mutex),
	}

	go q.collectTickThenSave()

	return q
}

func (q *quotationUseCase) ConsumeTradingResult(key string) {
	q.tradingRepo.SubscribeTradingResult(key, func(tradingResult *domain.TradingResult) {
		q.tradingResultsLock.Lock()
		defer q.tradingResultsLock.Unlock()
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			return
		}
		q.tradingResults = append(q.tradingResults, tradingResult)
	})
}

func (q *quotationUseCase) GetTicks() ([]string, error) {
	q.tickLock.Lock()
	res := make([]string, 0, q.tickList.Len())
	for front := q.tickList.Front(); front != nil; front = front.Next() {
		res = append(res, front.Value.(string))
	}
	q.tickLock.Unlock()

	return res, nil
}

func (q *quotationUseCase) collectTickThenSave() {
	ticker := time.NewTicker(100 * time.Millisecond) // TODO: is best way?
	defer ticker.Stop()

	for range ticker.C {
		q.tradingResultsLock.Lock()
		tradingResultsClone := make([]*domain.TradingResult, len(q.tradingResults))
		copy(tradingResultsClone, q.tradingResults)
		q.tradingResults = nil
		q.tradingResultsLock.Unlock()

		for _, tradingResult := range tradingResultsClone {
			for _, matchDetail := range tradingResult.MatchResult.MatchDetails {
				tick := &tick{
					domain.TickEntity{
						SequenceID:     tradingResult.TradingEvent.SequenceID,
						TakerOrderID:   matchDetail.TakerOrder.ID,
						MakerOrderID:   matchDetail.MakerOrder.ID,
						Price:          matchDetail.Price,
						Quantity:       matchDetail.Quantity,
						TakerDirection: matchDetail.TakerOrder.Direction,
						CreatedAt:      tradingResult.TradingEvent.CreatedAt,
					},
				}

				q.tickLock.Lock()
				if q.tickList.Len() >= q.cap {
					q.tickList.Remove(q.tickList.Front())
				}
				q.tickList.PushBack(tick.String())
				q.tickLock.Unlock()
			}

		}
	}
}
