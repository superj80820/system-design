package candle

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
)

const (
	redisKeySec  = "_sec_bars_"
	redisKeyMin  = "_min_bars_"
	redisKeyHour = "_hour_bars_"
	redisKeyDay  = "_day_bars_"
)

// TODO: performance
// TODO: fix bug(maybe?)
const addScript = `
--[[
    根据sequenceId和已合并的tick更新Bar数据
    
    参数：
    KEYS:
        1. sec-bar的key
        2. min-bar的key
        3. hour-bar的key
        4. day-bar的key
    ARGV:
        1. sequenceId
        2. secTimestamp
        3. minTimestamp
        4. hourTimestamp
        5. dayTimestamp
        6. openPrice
        7. highPrice
        8. lowPrice
        9. closePrice
        10. quantity
    
    Redis存储的Bar数据结构：[timestamp, open, high, low, close, quantity, volume]
    ZScoredSet:
        key: '_day_bars_'
        key: '_hour_bars_'
        key: '_min_bars_'
        key: '_sec_bars_'
    Key: _BarSeq_ 存储上次更新的SequenceId
    --]]
    
    local function merge(existBar, newBar)
        existBar[3] = math.max(existBar[3], newBar[3]) -- 更新High Price
        existBar[4] = math.min(existBar[4], newBar[4]) -- 更新Low Price
        existBar[5] = newBar[5] -- close
        existBar[6] = existBar[6] + newBar[6] -- 更新quantity
    end
    
    local function tryMergeLast(barType, seqId, zsetBars, timestamp, newBar)
        local topic = 'notification'
        local popedScore, popedBar
        -- 查找最后一个Bar:
        local poped = redis.call('ZPOPMAX', zsetBars)
        if #poped == 0 then
            -- ZScoredSet无任何bar, 直接添加:
            redis.call('ZADD', zsetBars, timestamp, cjson.encode(newBar))
            redis.call('PUBLISH', topic, '{"type":"bar","resolution":"' .. barType .. '","sequenceId":' .. seqId .. ',"data":' .. cjson.encode(newBar) .. '}')
        else
            popedBar = cjson.decode(poped[1])
            popedScore = tonumber(poped[2])
            if popedScore == timestamp then
                -- 合并Bar并发送通知:
                merge(popedBar, newBar)
                redis.call('ZADD', zsetBars, popedScore, cjson.encode(popedBar))
                redis.call('PUBLISH', topic, '{"type":"bar","resolution":"' .. barType .. '","sequenceId":' .. seqId .. ',"data":' .. cjson.encode(popedBar) .. '}')
            else
                -- 可持久化最后一个Bar，生成新的Bar:
                if popedScore < timestamp then
                    redis.call('ZADD', zsetBars, popedScore, cjson.encode(popedBar), timestamp, cjson.encode(newBar))
                    redis.call('PUBLISH', topic, '{"type":"bar","resolution":"' .. barType .. '","sequenceId":' .. seqId .. ',"data":' .. cjson.encode(newBar) .. '}')
                    return popedBar
                end
            end
        end
        return nil
    end
    
    local seqId = ARGV[1]
    local KEY_BAR_SEQ = '_BarSeq_'
    
    local zsetBars, topics, barTypeStartTimes
    local openPrice, highPrice, lowPrice, closePrice, quantity
    local persistBars = {}
    
    -- 检查sequence:
    local seq = redis.call('GET', KEY_BAR_SEQ)
    if not seq or tonumber(seqId) > tonumber(seq) then
        zsetBars = { KEYS[1], KEYS[2], KEYS[3], KEYS[4] }
        barTypeStartTimes = { tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4]), tonumber(ARGV[5]) }
        openPrice = tonumber(ARGV[6])
        highPrice = tonumber(ARGV[7])
        lowPrice = tonumber(ARGV[8])
        closePrice = tonumber(ARGV[9])
        quantity = tonumber(ARGV[10])
    
        local i, bar
        local names = { 'SEC', 'MIN', 'HOUR', 'DAY' }
        -- 检查是否可以merge:
        for i = 1, 4 do
            bar = tryMergeLast(names[i], seqId, zsetBars[i], barTypeStartTimes[i], { barTypeStartTimes[i], openPrice, highPrice, lowPrice, closePrice, quantity })
            if bar then
                persistBars[names[i]] = bar
            end
        end
        redis.call('SET', KEY_BAR_SEQ, seqId)
        return cjson.encode(persistBars)
    end
    
    redis.log(redis.LOG_WARNING, 'sequence ignored: exist seq => ' .. seq .. ' >= ' .. seqId .. ' <= new seq')
    
    return '{}'      
`

type secCandleBar struct {
	*domain.CandleBar
}

func (secCandleBar) TableName() string {
	return "sec_bars"
}

type minCandleBar struct {
	*domain.CandleBar
}

func (minCandleBar) TableName() string {
	return "min_bars"
}

type hourCandleBar struct {
	*domain.CandleBar
}

func (hourCandleBar) TableName() string {
	return "hour_bars"
}

type dayCandleBar struct {
	*domain.CandleBar
}

func (dayCandleBar) TableName() string {
	return "day_bars"
}

type candleRepo struct {
	orm                        *ormKit.DB
	redisInstance              *redisKit.Cache
	candleTradingResultMQTopic mq.MQTopic
	candleMQTopic              mq.MQTopic
}

func CreateCandleRepo(orm *ormKit.DB, redisInstance *redisKit.Cache, candleTradingResultMQTopic, candleMQTopic mq.MQTopic) domain.CandleRepo {
	return &candleRepo{
		orm:                        orm,
		redisInstance:              redisInstance,
		candleTradingResultMQTopic: candleTradingResultMQTopic,
		candleMQTopic:              candleMQTopic,
	}
}

func (c *candleRepo) SaveBarByMatchResult(ctx context.Context, matchResult *domain.MatchResult) error {
	openPrice := decimal.NewFromInt(0)
	closePrice := decimal.NewFromInt(0)
	highPrice := decimal.NewFromInt(0)
	lowPrice := decimal.NewFromInt(0)
	quantity := decimal.NewFromInt(0)
	for _, matchDetail := range matchResult.MatchDetails {
		if openPrice.Equal(decimal.Zero) {
			openPrice = matchDetail.Price
			highPrice = matchDetail.Price
			lowPrice = matchDetail.Price
		} else {
			highPrice = decimal.Max(highPrice, matchDetail.Price)
			lowPrice = decimal.Min(lowPrice, matchDetail.Price)
		}
		closePrice = matchDetail.Price
		quantity.Add(matchDetail.Quantity)
	}

	createdAtTimestamp := matchResult.CreatedAt.UnixMilli()
	secStartTime := createdAtTimestamp / 1000 * 1000
	minStartTime := createdAtTimestamp / 1000 / 60 * 1000 * 60
	hourStartTime := createdAtTimestamp / 1000 / 3600 * 1000 * 3600
	dayStartTime := createdAtTimestamp / 1000 / 3600 / 24 * 1000 * 3600 * 24

	// TODO: maybe need consistent
	cmd := c.redisInstance.RunLua(
		ctx,
		addScript,
		[]string{redisKeySec, redisKeyMin, redisKeyHour, redisKeyDay},
		strconv.Itoa(matchResult.SequenceID),
		strconv.FormatInt(secStartTime, 10),
		strconv.FormatInt(minStartTime, 10),
		strconv.FormatInt(hourStartTime, 10),
		strconv.FormatInt(dayStartTime, 10),
		openPrice.String(),
		closePrice.String(),
		highPrice.String(),
		lowPrice.String(),
		quantity.String(),
	)
	if err := cmd.Err(); err != nil {
		return errors.Wrap(err, "run lua failed")
	}
	result, err := cmd.Result()
	if err != nil {
		return errors.Wrap(err, "get result failed")
	}
	resultString, ok := result.(string)
	if !ok {
		return errors.Wrap(err, "cast to string failed")
	}
	resultMap := make(map[string][]decimal.Decimal)
	if err := json.Unmarshal([]byte(resultString), &resultMap); err != nil {
		return errors.Wrap(err, "unmarshal failed")
	}

	saveBarFn := func(bar []decimal.Decimal, barType domain.CandleTimeType) error {
		if len(bar) == 0 {
			return errors.Wrap(domain.ErrNoData, "no data")
		}

		startTime, err := utilKit.SafeInt64ToInt(bar[0].BigInt().Int64())
		if err != nil {
			return errors.Wrap(err, "int64 to int failed")
		}

		candleBar := &domain.CandleBar{
			Type:       barType,
			StartTime:  startTime,
			ClosePrice: bar[1],
			HighPrice:  bar[2],
			LowPrice:   bar[3],
			OpenPrice:  bar[4],
			Quantity:   bar[5],
		}

		switch candleBar.Type {
		case domain.CandleTimeTypeSec:
			if err := c.orm.Create(
				secCandleBar{
					CandleBar: candleBar,
				}).Error; err != nil {
				return errors.Wrap(err, "create bar to db failed")
			}
		case domain.CandleTimeTypeMin:
			if err := c.orm.Create(
				minCandleBar{
					CandleBar: candleBar,
				}).Error; err != nil {
				return errors.Wrap(err, "create bar to db failed")
			}
		case domain.CandleTimeTypeHour:
			if err := c.orm.Create(
				hourCandleBar{
					CandleBar: candleBar,
				}).Error; err != nil {
				return errors.Wrap(err, "create bar to db failed")
			}
		case domain.CandleTimeTypeDay:
			if err := c.orm.Create(
				dayCandleBar{
					CandleBar: candleBar,
				}).Error; err != nil {
				return errors.Wrap(err, "create bar to db failed")
			}
		}

		return nil
	}

	if err := saveBarFn(resultMap["SEC"], domain.CandleTimeTypeSec); !errors.Is(err, domain.ErrNoData) && err != nil { //TODO: 'SEC' to enum
		return errors.Wrap(err, "produce bar failed")
	}
	if err := saveBarFn(resultMap["MIN"], domain.CandleTimeTypeMin); !errors.Is(err, domain.ErrNoData) && err != nil { //TODO: 'MIN' to enum
		return errors.Wrap(err, "produce bar failed")
	}
	if err := saveBarFn(resultMap["HOUR"], domain.CandleTimeTypeHour); !errors.Is(err, domain.ErrNoData) && err != nil { //TODO: 'HOUR' to enum
		return errors.Wrap(err, "produce bar failed")
	}
	if err := saveBarFn(resultMap["DAY"], domain.CandleTimeTypeDay); !errors.Is(err, domain.ErrNoData) && err != nil { //TODO: 'DAY' to enum
		return errors.Wrap(err, "produce bar failed")
	}

	return nil
}

// GetBar response: [timestamp, openPrice, highPrice, lowPrice, closePrice, quantity]
func (c *candleRepo) GetBar(ctx context.Context, timeType domain.CandleTimeType, start, stop string, sortOrderBy domain.SortOrderByEnum) ([]string, error) {
	var redisKey string
	switch timeType {
	case domain.CandleTimeTypeSec:
		redisKey = redisKeySec
	case domain.CandleTimeTypeMin:
		redisKey = redisKeyMin
	case domain.CandleTimeTypeHour:
		redisKey = redisKeyHour
	case domain.CandleTimeTypeDay:
		redisKey = redisKeyDay
	}
	var revArg bool
	if sortOrderBy == domain.DESCSortOrderByEnum {
		revArg = true
	}
	result, err := c.redisInstance.ZRangeArgs(ctx, redisKit.ZRangeArgs{
		Key:     redisKey,
		Start:   start,
		Stop:    stop,
		ByScore: true,
		Rev:     revArg,
	}).Result()
	if err != nil {
		return nil, errors.Wrap(err, "get bar failed")
	}
	return result, nil
}

type mqMessage struct {
	*domain.TradingResult
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.SequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (c *candleRepo) ProduceCandleMQByTradingResults(ctx context.Context, tradingResults []*domain.TradingResult) error {
	mqMessages := make([]mq.Message, len(tradingResults))
	for idx, tradingResult := range tradingResults {
		if tradingResult.TradingResultStatus != domain.TradingResultStatusCreate {
			return nil
		}
		mqMessages[idx] = &mqMessage{
			TradingResult: tradingResult,
		}
	}

	if err := c.candleTradingResultMQTopic.ProduceBatch(ctx, mqMessages); err != nil {
		return errors.Wrap(err, "produce failed")
	}

	return nil
}

func (c *candleRepo) ConsumeCandleMQByTradingResultWithCommit(ctx context.Context, key string, notify func(tradingResult *domain.TradingResult, commitFn func() error) error) {
	c.candleTradingResultMQTopic.SubscribeWithManualCommit(key, func(message []byte, commitFn func() error) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.TradingResult, commitFn); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (c *candleRepo) ProduceCandleMQ(ctx context.Context, candleBar *domain.CandleBar) error {
	if err := c.candleMQTopic.Produce(ctx, &mqCandleMessage{
		CandleBar: candleBar,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (c *candleRepo) ConsumeCandleMQ(ctx context.Context, key string, notify func(candleBar *domain.CandleBar) error) {
	c.candleMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqCandleMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.CandleBar); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}
