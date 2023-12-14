package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/superj80820/system-design/domain"
)

type AuthClient struct {
	url string
}

type authVerifyRequest struct {
	AccessToken string `json:"access_token"`
}

type authVerifyResponse struct {
	UserID int64 `json:"user_id"`
}

func CreateAuthClient(url string) domain.AuthServiceRepository {
	return &AuthClient{
		url: url,
	}
}

func (a *AuthClient) Verify(accessToken string) (int64, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&authVerifyRequest{
		AccessToken: accessToken,
	}); err != nil { //TODO
		return 0, err // TODO
	}
	res, err := http.Post(a.url+"/api/v1/auth/verify", "application/json; charset=utf-8", &buf)
	if err != nil {
		return 0, err // TODO
	}
	if res.StatusCode != http.StatusOK {
		return 0, errors.New("not ok") // TODO
	}
	defer res.Body.Close()
	var verifyRes authVerifyResponse
	if err := json.NewDecoder(res.Body).Decode(&verifyRes); err != nil {
		return 0, err // TODO
	}
	return verifyRes.UserID, nil
}
