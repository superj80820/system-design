package domain

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type TicketPlusReserveSchedule struct {
	Done                        bool                                       `json:"done"`
	CountryCode                 string                                     `json:"country_code"`
	Mobile                      string                                     `json:"mobile"`
	Password                    string                                     `json:"password"`
	EventID                     string                                     `json:"event_id"`
	SessionID                   string                                     `json:"session_id"`
	Priority                    map[string]int                             `json:"priority"`
	CaptchaDuration             time.Duration                              `json:"captcha_duration"`
	CaptchaCount                int                                        `json:"captcha_count"`
	ReserveExecTime             int64                                      `json:"reserve_exec_time"`
	ReserveGetErrorThenContinue bool                                       `json:"reserve_get_error_then_continue"`
	ReserveFrequency            *TicketPlusReserveScheduleReserveFrequency `json:"reserve_frequency"`
	ReserveCount                int                                        `json:"reserve_count"`
	LineNotifyToken             string                                     `json:"line_notify_token"`
}

type TicketPlusReserveScheduleReserveFrequency struct {
	MaxCount int `json:"max_count"`
	Duration int `json:"duration"`
}

type ProcessReserveStatusResultEnum int

const (
	ProcessReserveStatusUnknown ProcessReserveStatusResultEnum = iota
	ProcessReserveStatusAdd
	ProcessReserveStatusUpdate
	ProcessReserveStatusStopTimeout
)

func (p ProcessReserveStatusResultEnum) String() string {
	switch p {
	case ProcessReserveStatusAdd:
		return "add"
	case ProcessReserveStatusUpdate:
		return "update"
	case ProcessReserveStatusStopTimeout:
		return "stop timeout"
	case ProcessReserveStatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

type TicketPlusService interface {
	Reserve(ctx context.Context, ticketPlusReserveRequest *TicketPlusReserveSchedule) (*TicketReserve, error)
	Reserves(ctx context.Context, ticketPlusReserveRequests []*TicketPlusReserveSchedule) ([]string, error)
	GetReservesSchedule() map[string]*TicketPlusReserveSchedule
	DeleteReservesSchedule(scheduleID string) error
	UpdateGlobalReserveFrequency(maxCount int, duration time.Duration)
}

type TicketPlusRepo interface {
	Login(countryCode, mobile, password string) (*LoginResponse, error)
	GetTicketInformation(eventID string) (*TicketInformation, error)
	GetTicketStatus(ticketAreaIDs, productIDs []string) (*TicketStatus, error)
	GetTicketStatusOrderByCount(ticketAreaIDs, productIDs []string) (*TicketStatus, error)
	GetCaptcha(token, sessionID string, refresh bool) (*TicketCaptcha, error)
	Reserve(ctx context.Context, token string, productIDs []*TicketReserveRequestProduct, captchaKey, captchaAns string, isAddMetadata bool) (*TicketReserve, error)

	SaveOrUpdateToken(countryCode, mobile, ticketPlusUserID, accessToken, refreshToken string, accessTokenExpiresIn time.Time) (id int64, err error)
	GetToken(countryCode, mobile string) (*TicketPlusDBUserToken, error)
}

type LoginResponse struct {
	ErrCode   string                 `json:"errCode"`
	ErrMsg    string                 `json:"errMsg"`
	ErrDetail string                 `json:"errDetail"`
	UserInfo  *LoginResponseUserInfo `json:"userInfo"`
}

type LoginResponseUserInfo struct {
	ID                   string `json:"id"`
	AccessToken          string `json:"access_token"`
	RefreshToken         string `json:"refresh_token"`
	AccessTokenExpiresIn int    `json:"access_token_expires_in"`
	VerifyEmail          bool   `json:"verifyEmail"`
}

type TicketInformation struct {
	Products []struct {
		SessionID    string    `json:"sessionId"`
		TicketAreaID string    `json:"ticketAreaId"`
		Name         string    `json:"name"`
		Price        int       `json:"price"`
		Hidden       bool      `json:"hidden"`
		SortedIndex  int       `json:"sortedIndex"`
		ProductID    string    `json:"productId"`
		ExposeStart  time.Time `json:"exposeStart"`
		ExposeEnd    time.Time `json:"exposeEnd"`
	} `json:"products"`
}

type TicketStatus struct {
	ErrCode   string `json:"errCode"`
	ErrMsg    string `json:"errMsg"`
	ErrDetail string `json:"errDetail"`
	Result    struct {
		Product []struct {
			Unit                 int       `json:"unit"`
			DisabilitiesType     int       `json:"disabilitiesType"`
			SaleEnd              time.Time `json:"saleEnd"`
			Status               string    `json:"status"`
			SortedIndex          int       `json:"sortedIndex"`
			Coupon               []any     `json:"coupon"`
			Uncountable          bool      `json:"uncountable"`
			ProductIbonSaleStart time.Time `json:"productIbonSaleStart"`
			ID                   string    `json:"id"`
			EventID              string    `json:"eventId"`
			SaleStart            time.Time `json:"saleStart"`
			ProductIbon          bool      `json:"productIbon"`
			ProductIbonSaleEnd   time.Time `json:"productIbonSaleEnd"`
			ProductLimit         bool      `json:"productLimit"`
			CcDiscount           []any     `json:"ccDiscount"`
			ExposeStart          time.Time `json:"exposeStart"`
			CreatedAt            time.Time `json:"createdAt"`
			ExposeEnd            time.Time `json:"exposeEnd"`
			Lock                 bool      `json:"lock"`
			SeatAssignment       bool      `json:"seatAssignment"`
			TicketAreaID         string    `json:"ticketAreaId"`
			SessionID            string    `json:"sessionId"`
			UpdatedAt            time.Time `json:"updatedAt"`
			UserLimit            int       `json:"userLimit"`
			Price                int       `json:"price"`
			PurchaseLimit        int       `json:"purchaseLimit"`
			AutoRelease          bool      `json:"autoRelease"`
			Hidden               bool      `json:"hidden"`
			Count                int       `json:"count"`
		} `json:"product"`
		TicketArea []*TicketStatusArea `json:"ticketArea"`
	} `json:"result"`
}

type TicketStatusArea struct {
	SaleStart               time.Time `json:"saleStart"`
	TicketAreaName          string    `json:"ticketAreaName"`
	TicketAreaLimit         bool      `json:"ticketAreaLimit"`
	ExposeStart             time.Time `json:"exposeStart"`
	SaleEnd                 time.Time `json:"saleEnd"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"createdAt"`
	ExposeEnd               time.Time `json:"exposeEnd"`
	Lock                    bool      `json:"lock"`
	SortedIndex             int       `json:"sortedIndex"`
	SeatAssignment          bool      `json:"seatAssignment"`
	TicketAreaIbon          bool      `json:"ticketAreaIbon"`
	UpdatedAt               time.Time `json:"updatedAt"`
	TicketAreaIbonSaleStart time.Time `json:"ticketAreaIbonSaleStart"`
	UserLimit               int       `json:"userLimit"`
	TicketAreaIbonSaleEnd   time.Time `json:"ticketAreaIbonSaleEnd"`
	ID                      string    `json:"id"`
	Price                   int       `json:"price"`
	Hidden                  bool      `json:"hidden"`
	Count                   int       `json:"count"`
}

type TicketCaptcha struct {
	ErrCode   string `json:"errCode"`
	ErrMsg    string `json:"errMsg"`
	ErrDetail string `json:"errDetail"`
	Key       string `json:"key"`
	Data      string `json:"data"`
}

type TicketPlusResponseStatus int

const (
	TicketPlusResponseStatusUnknown TicketPlusResponseStatus = iota
	TicketPlusResponseStatusOK
	TicketPlusResponseStatusPending
	TicketPlusResponseStatusReserved
	TicketPlusResponseStatusCaptchaFailed
	TicketPlusResponseStatusCaptchaNotFound
	TicketPlusResponseStatusTokenExpired
	TicketPlusResponseStatusSoldOut
	TicketPlusResponseStatusUserLocked
)

var (
	ErrCodePending = errors.New(TicketPlusResponseStatusPending.String())
	ErrCodeUnknown = errors.New(TicketPlusResponseStatusUnknown.String())

	ErrCodeTokenExpired = errors.New(TicketPlusResponseStatusTokenExpired.String())
	ErrCodeLocked       = errors.New(TicketPlusResponseStatusUserLocked.String())

	ReserveErrCodeReserved    = errors.New(TicketPlusResponseStatusReserved.String())
	ReserveErrCaptchaFailed   = errors.New(TicketPlusResponseStatusCaptchaFailed.String())
	ReserveErrCaptchaNotFound = errors.New(TicketPlusResponseStatusCaptchaNotFound.String())
	ReserveErrSoldOut         = errors.New(TicketPlusResponseStatusSoldOut.String())
)

func (t TicketPlusResponseStatus) String() string {
	switch t {
	case TicketPlusResponseStatusUnknown:
		return "unknown"
	case TicketPlusResponseStatusOK:
		return "status ok"
	case TicketPlusResponseStatusPending:
		return "pending"
	case TicketPlusResponseStatusReserved:
		return "reserved"
	case TicketPlusResponseStatusCaptchaFailed:
		return "captcha failed"
	case TicketPlusResponseStatusCaptchaNotFound:
		return "captcha not found"
	case TicketPlusResponseStatusTokenExpired:
		return "token expired"
	case TicketPlusResponseStatusSoldOut:
		return "sold out"
	case TicketPlusResponseStatusUserLocked:
		return "user locked"
	default:
		return "unknown"
	}
}

type TicketReserve struct {
	ErrCode   string `json:"errCode"`
	ErrMsg    string `json:"errMsg"`
	ErrDetail string `json:"errDetail"`
	Products  []struct {
		Info struct {
		} `json:"info"`
		Idx               int       `json:"idx"`
		ProductID         string    `json:"productId"`
		Count             int       `json:"count"`
		UserID            string    `json:"userId"`
		UserType          string    `json:"userType"`
		TicketAreaName    string    `json:"ticketAreaName"`
		ExpiryTimestamp   time.Time `json:"expiryTimestamp"`
		Status            string    `json:"status"`
		Hash              string    `json:"hash"`
		OrderID           int       `json:"orderId"`
		SingleTicketPrice int       `json:"singleTicketPrice"`
		CreatedAt         time.Time `json:"createdAt"`
	} `json:"products"`
	OrderID int    `json:"orderId"`
	Total   int    `json:"total"`
	Hash    string `json:"hash"`
}

type TicketReserveRequestProduct struct {
	ProductID string `json:"productId"`
	Count     int    `json:"count"`
}

type TicketPlusDBUserToken struct {
	ID int64

	CountryCode          string
	Mobile               string
	TicketPlusUserID     string
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresIn time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
