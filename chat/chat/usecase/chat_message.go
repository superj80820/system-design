package usecase

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/chat/domain"
)

type ChannelMessage struct {
	*domain.ChannelMessage
}

func (c ChannelMessage) GetKey() string {
	return strconv.Itoa(c.ChannelID)
}

func (c ChannelMessage) Marshal() ([]byte, error) {
	jsonData, err := json.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(err, "marshal failed")
	}
	return jsonData, nil
}

type StatusMessage struct {
	*domain.StatusMessage
}

// GetKey implements mq.Message.
func (*StatusMessage) GetKey() string {
	panic("unimplemented")
}

// Marshal implements mq.Message.
func (*StatusMessage) Marshal() ([]byte, error) {
	panic("unimplemented")
}
