package ticketplus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	utilKit "github.com/superj80820/system-design/kit/util"

	"github.com/superj80820/system-design/domain"

	"github.com/pkg/errors"
	ormKit "github.com/superj80820/system-design/kit/orm"
)

const createTable = `
	CREATE TABLE IF NOT EXISTS "ticket_plus_user_token" (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"country_code" varchar(10) NOT NULL,
		"mobile" varchar(30) NOT NULL,
		"ticket_plus_user_id" varchar(255) NOT NULL,
		"access_token" varchar(255) NOT NULL,
		"refresh_token" varchar(255) NOT NULL,
		"access_token_expires_in" timestamp NULL DEFAULT NULL,
		"created_at" timestamp NULL DEFAULT NULL,
		"updated_at" timestamp NULL DEFAULT NULL,
		UNIQUE ("country_code", "mobile"),
		UNIQUE ("ticket_plus_user_id")
	)
`

type ticketReserveRequestCaptchaStruct struct {
	Key string `json:"key"`
	Ans string `json:"ans"`
}

type ticketReserveRequestStruct struct {
	Products         []*domain.TicketReserveRequestProduct `json:"products"`
	Captcha          *ticketReserveRequestCaptchaStruct    `json:"captcha"`
	ReserveSeats     bool                                  `json:"reserveSeats"`
	ConsecutiveSeats bool                                  `json:"consecutiveSeats"`
	FinalizedSeats   bool                                  `json:"finalizedSeats"`
}

var _ domain.TicketPlusRepo = (*TicketPlus)(nil)

type TicketPlus struct {
	orm *ormKit.DB

	url string
}

func CreateTicketPlus(url string, orm *ormKit.DB) (domain.TicketPlusRepo, error) {
	if orm == nil {
		return nil, errors.New("no set orm")
	}

	if err := orm.Exec(createTable).Error; err != nil {
		return nil, errors.Wrap(err, "create db table failed")
	}

	return &TicketPlus{
		orm: orm,

		url: url,
	}, nil
}

func (t *TicketPlus) Login(countryCode, mobile, password string) (*domain.LoginResponse, error) {
	url := t.url + "/user/api/v1/login?_=" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	method := "POST"

	payload := strings.NewReader(
		`{"mobile":"` + mobile + `","countryCode":"` + countryCode + `","password":"` + password + `"}`)

	client := &http.Client{}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}

	req.Header.Add("authority", "apis.ticketplus.com.tw")
	req.Header.Add("accept", "application/json, text/plain, */*")
	req.Header.Add("accept-language", "en-US,en;q=0.9")
	req.Header.Add("content-type", "application/json")
	req.Header.Add("origin", "https://ticketplus.com.tw")
	req.Header.Add("referer", "https://ticketplus.com.tw/")
	req.Header.Add("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-site")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, errors.Wrap(err, fmt.Sprintf("http code error, http status: %s", res.Status))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	var loginResponse domain.LoginResponse
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return nil, errors.Wrap(err, "unmarshal json failed")
	}

	if err := t.parseErrorCodeWithMetadata(loginResponse.ErrCode, string(body)); err != nil {
		return nil, err
	}

	if loginResponse.UserInfo == nil {
		return nil, errors.New(fmt.Sprintf("get null user information, raw data: %s", string(body)))
	}

	return &loginResponse, nil
}

func (t *TicketPlus) GetTicketInformation(eventID string) (*domain.TicketInformation, error) {
	url := t.url + "/config/api/v1/getS3?path=event%2F" + eventID + "%2Fproducts.json"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("authority", "apis.ticketplus.com.tw")
	req.Header.Add("accept", "application/json, text/plain, */*")
	req.Header.Add("accept-language", "en-US,en;q=0.9,zh-TW;q=0.8,zh;q=0.7,ja;q=0.6")
	req.Header.Add("origin", "https://ticketplus.com.tw")
	req.Header.Add("referer", "https://ticketplus.com.tw/")
	req.Header.Add("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-site")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read request failed")
	}

	var ticketInformation domain.TicketInformation
	if err := json.Unmarshal(body, &ticketInformation); err != nil {
		return nil, errors.Wrap(err, "unmarshal response failed")
	}

	if len(ticketInformation.Products) == 0 {
		return nil, errors.Wrap(err, "products is 0 error, raw data: "+string(body))
	}

	return &ticketInformation, nil
}

func (t *TicketPlus) GetTicketStatus(ticketAreaIDs, productIDs []string) (*domain.TicketStatus, error) {
	return t.getTicketStatus(ticketAreaIDs, productIDs)
}

func (t *TicketPlus) GetTicketStatusOrderByCount(ticketAreaIDs []string, productIDs []string) (ticketStatus *domain.TicketStatus, err error) {
	ticketStatus, err = t.getTicketStatus(ticketAreaIDs, productIDs)
	if err != nil {
		return
	}
	sort.SliceStable(ticketStatus.Result.TicketArea, func(i, j int) bool {
		return ticketStatus.Result.TicketArea[i].Count > ticketStatus.Result.TicketArea[j].Count
	})
	return
}

func (t *TicketPlus) getTicketStatus(ticketAreaIDs, productIDs []string) (*domain.TicketStatus, error) {
	queryTicketAreaIDs := strings.Join(ticketAreaIDs, url.QueryEscape(","))
	queryProductIDs := strings.Join(productIDs, url.QueryEscape(","))
	url := t.url + "/config/api/v1/get?" +
		"ticketAreaId=" + queryTicketAreaIDs +
		"&productId=" + queryProductIDs +
		"&_=" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("authority", "apis.ticketplus.com.tw")
	req.Header.Add("accept", "application/json, text/plain, */*")
	req.Header.Add("accept-language", "en-US,en;q=0.9,zh-TW;q=0.8,zh;q=0.7,ja;q=0.6")
	req.Header.Add("origin", "https://ticketplus.com.tw")
	req.Header.Add("referer", "https://ticketplus.com.tw/")
	req.Header.Add("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-site")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read request failed")
	}

	var ticketStatus domain.TicketStatus
	if err := json.Unmarshal(body, &ticketStatus); err != nil {
		return nil, errors.Wrap(err, "unmarshal json failed")
	}

	if len(ticketStatus.Result.TicketArea) == 0 {
		return nil, errors.New(fmt.Sprintf("ticket area length is 0, raw data: %s", string(body)))
	}

	sort.Slice(ticketStatus.Result.TicketArea, func(i, j int) bool {
		return ticketStatus.Result.TicketArea[i].SortedIndex < ticketStatus.Result.TicketArea[j].SortedIndex
	})

	return &ticketStatus, nil
}

func (t *TicketPlus) GetCaptcha(token, sessionID string, refresh bool) (*domain.TicketCaptcha, error) {
	url := t.url + "/captcha/api/v1/generate?_=" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	method := "POST"

	payloadMap := make(map[string]interface{})
	payloadMap["sessionId"] = sessionID
	payloadMap["refresh"] = refresh
	payloadJsonMarshal, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, errors.Wrap(err, "marshal payload failed")
	}
	payload := bytes.NewReader(payloadJsonMarshal)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("authority", "apis.ticketplus.com.tw")
	req.Header.Add("accept", "application/json, text/plain, */*")
	req.Header.Add("accept-language", "en-US,en;q=0.9,zh-TW;q=0.8,zh;q=0.7,ja;q=0.6")
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("origin", "https://ticketplus.com.tw")
	req.Header.Add("referer", "https://ticketplus.com.tw/")
	req.Header.Add("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-site")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	var ticketCaptcha domain.TicketCaptcha
	if err := json.Unmarshal(body, &ticketCaptcha); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unmarshal json failed, http status: %s, body: %s", res.Status, string(body)))
	}

	return &ticketCaptcha, t.parseErrorCodeWithMetadata(ticketCaptcha.ErrCode, string(body))
}

func (t *TicketPlus) reserve(ctx context.Context, token string, ticketReserveRequest *ticketReserveRequestStruct) (*domain.TicketReserve, error) {
	url := t.url + "/ticket/api/v1/reserve?_=" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	method := "POST"

	payloadJsonMarshal, err := json.Marshal(ticketReserveRequest)
	if err != nil {
		return nil, errors.Wrap(err, "marshal payload failed")
	}
	payload := bytes.NewReader(payloadJsonMarshal)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	req = req.WithContext(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("authority", "apis.ticketplus.com.tw")
	req.Header.Add("accept", "application/json, text/plain, */*")
	req.Header.Add("accept-language", "en-US,en;q=0.9,zh-TW;q=0.8,zh;q=0.7,ja;q=0.6")
	req.Header.Add("authorization", "Bearer "+token)
	req.Header.Add("content-type", "application/json;charset=UTF-8")
	req.Header.Add("origin", "https://ticketplus.com.tw")
	req.Header.Add("referer", "https://ticketplus.com.tw/")
	req.Header.Add("sec-ch-ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-site")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read request failed")
	}

	var ticketReserve domain.TicketReserve
	if err := json.Unmarshal(body, &ticketReserve); err != nil {
		return nil, errors.Wrap(err, "unmarshal json failed")
	}

	if err := t.parseErrorCodeWithMetadata(ticketReserve.ErrCode, string(body)); err != nil {
		return nil, err
	}

	if ticketReserve.OrderID == 0 {
		return nil, errors.New(fmt.Sprintf("get null products, raw data: %s", string(body)))
	}

	return &ticketReserve, nil
}

func (t *TicketPlus) parseErrorCodeWithMetadata(errCode string, metadata string) error {
	switch errCode {
	case "137":
		return domain.ErrCodePending
	case "00":
		return nil
	case "111":
		return domain.ReserveErrCodeReserved
	case "135":
		return domain.ReserveErrCaptchaFailed
	case "136":
		return domain.ReserveErrCaptchaNotFound
	case "121":
		return domain.ReserveErrSoldOut
	case "103":
		return domain.ErrCodeTokenExpired
	case "131":
		return domain.ErrCodeLocked
	default:
		return errors.Wrap(domain.ErrCodeUnknown, "metadata: "+metadata)
	}
}

func (t *TicketPlus) Reserve(ctx context.Context, token string, productIDs []*domain.TicketReserveRequestProduct, captchaKey, captchaAns string) (*domain.TicketReserve, error) {
	return t.reserve(ctx, token, &ticketReserveRequestStruct{
		Products: productIDs,
		Captcha: &ticketReserveRequestCaptchaStruct{
			Key: captchaKey,
			Ans: captchaAns,
		},
		ReserveSeats:     true,
		ConsecutiveSeats: true,
		FinalizedSeats:   true,
	})
}

func (t *TicketPlus) GetToken(countryCode, mobile string) (*domain.TicketPlusDBUserToken, error) {
	var userToken ticketPlusDBUserToken
	err := t.orm.Where("country_code = ? AND mobile = ?", countryCode, mobile).First(&userToken).Error
	if err != nil {
		return nil, errors.Wrap(err, "get db user failed")
	}
	return &userToken.TicketPlusDBUserToken, nil
}

func (t *TicketPlus) SaveOrUpdateToken(countryCode, mobile, ticketPlusUserID string, accessToken, refreshToken string, accessTokenExpiresIn time.Time) (int64, error) {
	var userToken ticketPlusDBUserToken
	err := t.orm.Where("country_code = ? AND mobile = ?", countryCode, mobile).First(&userToken).Error
	if errors.Is(err, ormKit.ErrRecordNotFound) {
		userToken := ticketPlusDBUserToken{
			domain.TicketPlusDBUserToken{
				ID:                   utilKit.GetSnowflakeIDInt64(),
				CountryCode:          countryCode,
				Mobile:               mobile,
				TicketPlusUserID:     ticketPlusUserID,
				AccessTokenExpiresIn: accessTokenExpiresIn,
				AccessToken:          accessToken,
				RefreshToken:         refreshToken,
			},
		}
		if err := t.orm.Where("country_code = ? AND mobile = ?", countryCode, mobile).Save(&userToken).Error; err != nil {
			return 0, errors.Wrap(err, "save user token failed")
		}
		return userToken.ID, nil
	}

	userToken.AccessToken = accessToken
	userToken.RefreshToken = refreshToken
	userToken.AccessTokenExpiresIn = accessTokenExpiresIn
	if err := t.orm.Where("country_code = ? AND mobile = ?", countryCode, mobile).Save(&userToken).Error; err != nil {
		return 0, errors.Wrap(err, "save user token failed")
	}
	return userToken.ID, nil
}
