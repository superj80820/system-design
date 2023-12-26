package ticketplus

import "github.com/superj80820/system-design/domain"

type ticketPlusDBUserToken struct {
	domain.TicketPlusDBUserToken
}

func (t *ticketPlusDBUserToken) TableName() string {
	return "ticket_plus_user_token"
}
