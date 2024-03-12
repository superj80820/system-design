package candle

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/mq"
)

type mqCandleMessage struct {
	*domain.CandleBar
}

var _ mq.Message = (*mqCandleMessage)(nil)

func (m *mqCandleMessage) GetKey() string {
	return strconv.Itoa(m.StartTime)
}

func (m *mqCandleMessage) Marshal() ([]byte, error) {
	marshalData, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return marshalData, nil
}
