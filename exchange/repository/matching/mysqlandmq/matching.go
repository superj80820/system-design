package mysqlandmq

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"gorm.io/gorm/clause"
)

type matchOrderDetailDB struct {
	*domain.MatchOrderDetail
}

func (*matchOrderDetailDB) TableName() string {
	return "match_details"
}

type matchingRepo struct {
	orm             *ormKit.DB
	matchingMQTopic mq.MQTopic

	sequenceID  int
	marketPrice decimal.Decimal
}

func CreateMatchingRepo(orm *ormKit.DB, matchingMQTopic mq.MQTopic) domain.MatchingRepo {
	return &matchingRepo{
		orm:             orm,
		matchingMQTopic: matchingMQTopic,
		marketPrice:     decimal.Zero,
	}
}

func (m *matchingRepo) GetSequenceID() int {
	return m.sequenceID
}

func (m *matchingRepo) GetMarketPrice() decimal.Decimal {
	return m.marketPrice
}

func (m *matchingRepo) SetMarketPrice(price decimal.Decimal) {
	m.marketPrice = price
}

func (m *matchingRepo) SetSequenceID(sequenceID int) {
	m.sequenceID = sequenceID
}

func (m *matchingRepo) GetMatchingDetails(orderID int) ([]*domain.MatchOrderDetail, error) {
	var matchOrderDetails []*domain.MatchOrderDetail
	if err := m.orm.Model(&matchOrderDetailDB{}).Where("order_id = ?", orderID).Order("id DESC").Find(&matchOrderDetails).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return matchOrderDetails, nil
}

func (m *matchingRepo) SaveMatchingDetailsWithIgnore(ctx context.Context, matchOrderDetails []*domain.MatchOrderDetail) error {
	matchOrderDetailsDB := make([]*matchOrderDetailDB, len(matchOrderDetails))
	for idx, matchOrderDetail := range matchOrderDetails { // TODO: need for loop to assign?
		matchOrderDetailsDB[idx] = &matchOrderDetailDB{
			MatchOrderDetail: matchOrderDetail,
		}
	}
	if err := m.orm.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(matchOrderDetailsDB).Error; err != nil {
		return errors.Wrap(err, "save match order details failed")
	}
	return nil
}

type mqMessage struct {
	*domain.MatchOrderDetail
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.MatchOrderDetail.SequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (m *matchingRepo) ProduceMatchOrderMQByTradingResults(ctx context.Context, tradingResults []*domain.TradingResult) error {
	var mqMessages []mq.Message

	for _, tradingResult := range tradingResults {
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			break
		}

		for _, matchDetail := range tradingResult.MatchResult.MatchDetails {
			takerOrderDetail := &domain.MatchOrderDetail{
				SequenceID:     tradingResult.SequenceID,
				OrderID:        matchDetail.TakerOrder.ID,
				CounterOrderID: matchDetail.MakerOrder.ID,
				UserID:         matchDetail.TakerOrder.UserID,
				CounterUserID:  matchDetail.MakerOrder.UserID,
				Direction:      matchDetail.TakerOrder.Direction,
				Price:          matchDetail.Price,
				Quantity:       matchDetail.Quantity,
				Type:           domain.MatchTypeTaker,
				CreatedAt:      tradingResult.MatchResult.CreatedAt,
			}
			makerOrderDetail := &domain.MatchOrderDetail{
				SequenceID:     tradingResult.SequenceID,
				OrderID:        matchDetail.MakerOrder.ID,
				CounterOrderID: matchDetail.TakerOrder.ID,
				UserID:         matchDetail.MakerOrder.UserID,
				CounterUserID:  matchDetail.TakerOrder.UserID,
				Direction:      matchDetail.MakerOrder.Direction,
				Price:          matchDetail.Price,
				Quantity:       matchDetail.Quantity,
				Type:           domain.MatchTypeMaker,
				CreatedAt:      tradingResult.MatchResult.CreatedAt,
			}

			mqMessages = append(mqMessages,
				&mqMessage{
					MatchOrderDetail: takerOrderDetail,
				},
				&mqMessage{
					MatchOrderDetail: makerOrderDetail,
				},
			)
		}
	}

	if err := m.matchingMQTopic.ProduceBatch(ctx, mqMessages); err != nil {
		return errors.Wrap(err, "produce failed")
	}

	return nil
}

func (m *matchingRepo) ConsumeMatchOrderMQBatch(ctx context.Context, key string, notify func([]*domain.MatchOrderDetail) error) {
	m.matchingMQTopic.SubscribeBatch(key, func(messages [][]byte) error {
		details := make([]*domain.MatchOrderDetail, len(messages))
		for idx, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			details[idx] = mqMessage.MatchOrderDetail
		}

		if err := notify(details); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (m *matchingRepo) ConsumeMatchOrderMQBatchWithCommit(ctx context.Context, key string, notify func(matchOrderDetails []*domain.MatchOrderDetail, commitFn func() error) error) {
	m.matchingMQTopic.SubscribeBatchWithManualCommit(key, func(messages [][]byte, commitFn func() error) error {
		details := make([]*domain.MatchOrderDetail, len(messages))
		for idx, message := range messages {
			var mqMessage mqMessage
			err := json.Unmarshal(message, &mqMessage)
			if err != nil {
				return errors.Wrap(err, "unmarshal failed")
			}
			details[idx] = mqMessage.MatchOrderDetail
		}

		if err := notify(details, commitFn); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (m *matchingRepo) GetMatchingHistory(maxResults int) ([]*domain.MatchOrderDetail, error) {
	var matchOrderDetails []*domain.MatchOrderDetail
	if err := m.orm.Model(&matchOrderDetailDB{}).Order("id DESC").Limit(maxResults).Find(&matchOrderDetails).Error; err != nil {
		return nil, errors.Wrap(err, "query failed")
	}
	return matchOrderDetails, nil
}
