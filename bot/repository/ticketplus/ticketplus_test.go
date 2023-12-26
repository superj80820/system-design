package ticketplus

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/superj80820/system-design/domain"
	ormKit "github.com/superj80820/system-design/kit/orm"

	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := io.ReadAll(r.Body)
		assert.Nil(t, err)
		reqJson := make(map[string]interface{})
		err = json.Unmarshal(req, &reqJson)
		assert.Nil(t, err)

		assert.Equal(t, "985698738", reqJson["mobile"].(string))
		assert.Equal(t, "abcd", reqJson["password"].(string))
		assert.Equal(t, "886", reqJson["countryCode"].(string))
		assert.Regexp(t, `^/user/api/v1/login\?_=\d{13}$`, r.URL.String())
	}))
	defer server.Close()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus(server.URL, ormDB)
	assert.Nil(t, err)
	ticketPlus.Login("886", "985698738", "abcd")
}

func TestGetTicketInformation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/config/api/v1/getS3?path=event%2Fb0b7808cd4e5ba73763b9c5f583b98f2%2Fproducts.json", r.URL.String())
		w.Write([]byte(`{"products":[{"sessionId":"123"}]}`))
	}))
	defer server.Close()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus(server.URL, ormDB)
	assert.Nil(t, err)
	ticketInformation, err := ticketPlus.GetTicketInformation("b0b7808cd4e5ba73763b9c5f583b98f2")
	assert.Nil(t, err)
	assert.Equal(t, "123", ticketInformation.Products[0].SessionID)
}

func TestGetTicketStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Regexp(t, `^/config/api/v1/get\?ticketAreaId=a000000927%2Ca000000929&productId=b000000927%2Cb000000929&_=\d{13}$`, r.URL.String())
		w.Write([]byte(`{"result":{"product":[{"count":10}],"ticketArea":[{"sortedIndex": 3, "count":30},{"sortedIndex": 2, "count":0},{"sortedIndex": 1, "count":0}]}}`))
	}))
	defer server.Close()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus(server.URL, ormDB)
	assert.Nil(t, err)
	ticketStatus, err := ticketPlus.GetTicketStatusOrderByCount(
		[]string{"a000000927", "a000000929"},
		[]string{"b000000927", "b000000929"},
	)
	assert.Nil(t, err)
	assert.Equal(t, 10, ticketStatus.Result.Product[0].Count)
	assert.Equal(t, 30, ticketStatus.Result.TicketArea[0].Count)
	assert.Equal(t, 3, ticketStatus.Result.TicketArea[0].SortedIndex)
	assert.Equal(t, 0, ticketStatus.Result.TicketArea[1].Count)
	assert.Equal(t, 1, ticketStatus.Result.TicketArea[1].SortedIndex)
	assert.Equal(t, 0, ticketStatus.Result.TicketArea[2].Count)
	assert.Equal(t, 2, ticketStatus.Result.TicketArea[2].SortedIndex)
}

func TestGetCaptcha(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Regexp(t, `^/captcha/api/v1/generate\?_=\d{13}$`, r.URL.String())
		w.Write([]byte(`{"errCode":"00","data":"image"}`))
	}))
	defer server.Close()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus(server.URL, ormDB)
	assert.Nil(t, err)
	ticketCaptcha, err := ticketPlus.GetCaptcha("token", "123", true)
	assert.Nil(t, err)
	assert.Equal(t, "image", ticketCaptcha.Data)
}

func TestReserve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Regexp(t, `^/ticket/api/v1/reserve\?_=\d{13}$`, r.URL.String())
		// TODO: 137
		w.Write([]byte(`
        {
            "errCode": "137",
            "products": [
                {
                    "productId": "p000002754"
                }
            ],
            "orderId": 6343068
        }
        `))
	}))
	defer server.Close()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus(server.URL, ormDB)
	assert.Nil(t, err)
	ticketReserve, err := ticketPlus.Reserve("token", []*domain.TicketReserveRequestProduct{
		{
			ProductID: "123",
			Count:     10,
		},
		{
			ProductID: "567",
			Count:     11,
		},
	},
		"captchaKey",
		"captchaAns",
	)
	assert.ErrorIs(t, err, domain.ErrCodePending)
	assert.Equal(t, "p000002754", ticketReserve.Products[0].ProductID)
	assert.Equal(t, 6343068, ticketReserve.OrderID)
}

func TestGetAndSaveToken(t *testing.T) {
	// clean test db
	err := os.Remove("test.db")
	assert.Nil(t, err)
	defer func() {
		err := os.Remove("test.db")
		assert.Nil(t, err)
	}()

	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("test.db"))
	assert.Nil(t, err)

	ticketPlus, err := CreateTicketPlus("", ormDB)
	assert.Nil(t, err)

	id, err := ticketPlus.SaveOrUpdateToken("123", "123456789", "userID", "accessToken", "refreshToken", time.Now().Add(3600*time.Second))
	assert.Nil(t, err)
	assert.Less(t, int64(0), id)

	timeNow := time.Now()
	id, err = ticketPlus.SaveOrUpdateToken("123", "123456789", "userID", "accessToken2", "refreshToken2", timeNow.Add(3600*time.Second))
	assert.Nil(t, err)
	assert.Less(t, int64(0), id)

	userToken, err := ticketPlus.GetToken("123", "123456789")
	assert.Nil(t, err)
	assert.Equal(t, "accessToken2", userToken.AccessToken)
	assert.Equal(t, "refreshToken2", userToken.RefreshToken)
	assert.Equal(t, timeNow.Add(3600*time.Second).Unix(), userToken.AccessTokenExpiresIn.Unix())

	userToken, err = ticketPlus.GetToken("123", "987654321")
	assert.ErrorIs(t, err, ormKit.ErrRecordNotFound)
}

// func TestLoginE2E(t *testing.T) {
// 	ticketPlus := CreateTicketPlus("985698738", "4b78678bc428e557c05ee431f1da9581", "886", "https://apis.ticketplus.com.tw")
// 	loginRes, err := ticketPlus.Login()
// 	assert.Nil(t, err)
// 	t.Log(loginRes)
// }
