package readevent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
)

type mockEventRepo struct{}

func (m *mockEventRepo) AddEvent(event *domain.TicketPlusEvent) {}
func (m *mockEventRepo) ReadEvent(key string) (chan *domain.TicketPlusEvent, error) {
	ch := make(chan *domain.TicketPlusEvent)
	go func() {
		for i := 0; i < 10000000; i++ {
			ch <- &domain.TicketPlusEvent{
				Status:      domain.TicketPlusResponseStatusOK,
				SortedIndex: 1,
			}
		}
	}()
	return ch, nil
}

func TestGetTicketPlusStatus(t *testing.T) {
	mockEventRepo := new(mockEventRepo)
	readEvent := CreateReadEvent(mockEventRepo)

	ticketPlusStatus, err := readEvent.GetTicketPlusStatus("key", "eventKey")
	assert.Nil(t, err)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		for _, val := range ticketPlusStatus() {
			if val.Count[1] == 10000000 {
				return
			}
			assert.LessOrEqual(t, val.Count[1], 10000000)
		}
	}
}
