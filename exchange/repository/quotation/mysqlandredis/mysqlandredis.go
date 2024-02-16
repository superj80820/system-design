package mysqlandredis

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
	ormKit "github.com/superj80820/system-design/kit/orm"
	redisKit "github.com/superj80820/system-design/kit/redis"
)

const redisTicksKey = `_ticks_`

// TODO: refactor
const updateLuaScript = `
--[[

根据sequenceId判断是否需要发送tick通知

KEYS:
  1: 最新Ticks的Key

ARGV:
  1: sequenceId
  2: JSON字符串表示的tick数组："[{...},{...},...]"
  3: JSON字符串表示的tick数组："["{...}","{...}",...]"
--]]

local KEY_LAST_SEQ = '_TickSeq_' -- 上次更新的SequenceID
local LIST_RECENT_TICKS = KEYS[1] -- 最新Ticks的Key

local seqId = ARGV[1] -- 输入的SequenceID
local jsonData = ARGV[2] -- 输入的JSON字符串表示的tick数组："[{...},{...},...]"
local strData = ARGV[3] -- 输入的JSON字符串表示的tick数组："["{...}","{...}",...]"

-- 获取上次更新的sequenceId:
local lastSeqId = redis.call('GET', KEY_LAST_SEQ)
local ticks, len;

if not lastSeqId or tonumber(seqId) > tonumber(lastSeqId) then
    -- 广播:
    redis.call('PUBLISH', 'notification', '{"type":"tick","sequenceId":' .. seqId .. ',"data":' .. jsonData .. '}')
    -- 保存当前sequence id:
    redis.call('SET', KEY_LAST_SEQ, seqId)
    -- 更新最新tick列表:
    ticks = cjson.decode(strData)
    len = redis.call('RPUSH', LIST_RECENT_TICKS, unpack(ticks))
    if len > 100 then
        -- 裁剪LIST以保存最新的100个Tick:
        redis.call('LTRIM', LIST_RECENT_TICKS, len-100, len-1)
    end
    return true
end

-- 无更新返回false
return false
`

type tickDBEntity struct {
	*domain.TickEntity
}

func (t *tickDBEntity) String() string { // TODO: need notice direction in frontend
	return "[" + strconv.FormatInt(t.CreatedAt.UnixMilli(), 10) + "," + strconv.Itoa(int(t.TakerDirection)) + "," + t.Price.String() + "," + t.Quantity.String() + "]"
}

func (*tickDBEntity) TableName() string {
	return "ticks"
}

type quotationRepo struct {
	orm             *ormKit.DB
	redisCache      *redisKit.Cache
	tickMQTopic     mq.MQTopic
	tickSaveMQTopic mq.MQTopic
}

func CreateQuotationRepo(orm *ormKit.DB, redisCache *redisKit.Cache, tickMQTopic, tickSaveMQTopic mq.MQTopic) domain.QuotationRepo {
	return &quotationRepo{
		orm:             orm,
		redisCache:      redisCache,
		tickMQTopic:     tickMQTopic,
		tickSaveMQTopic: tickSaveMQTopic,
	}
}

func (q *quotationRepo) GetTickStrings(ctx context.Context, start int64, stop int64) ([]string, error) {
	cmd := q.redisCache.LRange(ctx, redisTicksKey, start, stop)
	if err := cmd.Err(); err != nil {
		return nil, errors.Wrap(err, "get LRANGE failed")
	}
	tickStrings := cmd.Val()
	ticks := make([]string, len(tickStrings))
	for idx, val := range tickStrings {
		ticks[idx] = val
	}
	return ticks, nil
}

func (q *quotationRepo) SaveTickStrings(ctx context.Context, sequenceID int, ticks []*domain.TickEntity) error {
	if len(ticks) == 0 {
		return domain.ErrNoop
	}

	ticksJoiner := make([]string, len(ticks))
	ticksStrJoiner := make([]string, len(ticks))
	for idx, tick := range ticks {
		tickDB := tickDBEntity{TickEntity: tick}
		ticksJoiner[idx] = tickDB.String()
		ticksStrJoiner[idx] = "\"" + tickDB.String() + "\""
	}
	cmd := q.redisCache.RunLua(
		ctx,
		updateLuaScript,
		[]string{redisTicksKey},
		[]string{
			strconv.Itoa(sequenceID),
			"[" + strings.Join(ticksJoiner, ",") + "]",
			"[" + strings.Join(ticksStrJoiner, ",") + "]",
		})
	if err := cmd.Err(); err != nil {
		return errors.Wrap(err, "execute lua failed")
	}
	result, err := cmd.Result()
	if err != nil {
		return errors.Wrap(err, "get lua result failed")
	}
	isLuaRunOK, ok := result.(int64)
	if !ok {
		return errors.New("except lua return value type")
	}
	if isLuaRunOK != 1 {
		return errors.New("run lua failed")
	}

	tickDBs := make([]*tickDBEntity, len(ticks))
	for idx, tick := range ticks { // TODO: maybe no need?
		tickDBs[idx] = &tickDBEntity{
			TickEntity: tick,
		}
	}
	if err := q.orm.Create(tickDBs).Error; err != nil {
		return errors.Wrap(err, "create ticks failed")
	}
	return nil
}

type mqMessage struct {
	sequenceID int
	ticks      []*domain.TickEntity
}

var _ mq.Message = (*mqMessage)(nil)

func (m *mqMessage) GetKey() string {
	return strconv.Itoa(m.sequenceID)
}

func (m *mqMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}

func (q *quotationRepo) ProduceTicksSaveMQByMatchResult(ctx context.Context, matchResult *domain.MatchResult) error {
	ticks := make([]*domain.TickEntity, len(matchResult.MatchDetails))
	for _, matchDetail := range matchResult.MatchDetails {
		ticks = append(ticks, &domain.TickEntity{
			SequenceID:     matchResult.SequenceID,
			TakerOrderID:   matchDetail.TakerOrder.ID,
			MakerOrderID:   matchDetail.MakerOrder.ID,
			Price:          matchDetail.Price,
			Quantity:       matchDetail.Quantity,
			TakerDirection: matchDetail.TakerOrder.Direction,
			CreatedAt:      matchResult.CreatedAt,
		})
	}
	if err := q.tickSaveMQTopic.Produce(ctx, &mqMessage{
		sequenceID: matchResult.SequenceID,
		ticks:      ticks,
	}); err != nil {
		return errors.Wrap(err, "produce failed")
	}
	return nil
}

func (q *quotationRepo) ConsumeTicksSaveMQ(ctx context.Context, key string, notify func(sequenceID int, ticks []*domain.TickEntity) error) {
	q.tickSaveMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.sequenceID, mqMessage.ticks); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}

func (q *quotationRepo) ProduceTicks(ctx context.Context, sequenceID int, ticks []*domain.TickEntity) error {
	if err := q.tickMQTopic.Produce(ctx, &mqMessage{
		sequenceID: sequenceID,
		ticks:      ticks,
	}); err != nil {
		return errors.Wrap(err, "produce tick failed")
	}
	return nil
}

func (q *quotationRepo) ConsumeTicks(ctx context.Context, key string, notify func(sequenceID int, ticks []*domain.TickEntity) error) {
	q.tickMQTopic.Subscribe(key, func(message []byte) error {
		var mqMessage mqMessage
		err := json.Unmarshal(message, &mqMessage)
		if err != nil {
			return errors.Wrap(err, "unmarshal failed")
		}
		if err := notify(mqMessage.sequenceID, mqMessage.ticks); err != nil {
			return errors.Wrap(err, "notify failed")
		}
		return nil
	})
}
