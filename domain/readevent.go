package domain

type ReadEventService interface {
	GetTicketPlusStatus(key, eventKey string) (func() []*ReadEventStatus, error)
}

type ReadEventStatus struct {
	Key         string
	Name        string
	Done        bool
	Count       []int
	SortedIndex int
}
