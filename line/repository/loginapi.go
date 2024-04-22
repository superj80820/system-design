package repository

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type lineLoginAPIRepo struct {
	clientID     string
	clientSecret string
}

func CreateLineLoginAPIRepo(clientID, clientSecret string) domain.LineLoginAPIRepo {
	return &lineLoginAPIRepo{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (l *lineLoginAPIRepo) IssueAccessToken(code string, redirectURI string) (*domain.LineIssueAccessToken, error) {
	url := "https://api.line.me/oauth2/v2.1/token"
	method := "POST"

	payload := []string{
		"grant_type=authorization_code",
		"code=" + code,
		"client_id=" + l.clientID,
		"client_secret=" + l.clientSecret,
		"redirect_uri=" + redirectURI,
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(strings.Join(payload, "&")))
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "io read body failed")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get response error, http code: " + res.Status + ", body: " + string(body))
	}

	var lineIssueAccessToken domain.LineIssueAccessToken
	if err := json.Unmarshal(body, &lineIssueAccessToken); err != nil {
		return nil, errors.Wrap(err, "json unmarshal failed")
	}

	return &lineIssueAccessToken, nil
}

func (l *lineLoginAPIRepo) VerifyToken(accessToken string) (*domain.LineVerifyToken, error) {
	url := "https://api.line.me/oauth2/v2.1/verify"
	method := "POST"

	payload := []string{
		"id_token=" + accessToken,
		"client_id=" + l.clientID,
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(strings.Join(payload, "&")))
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "json unmarshal failed")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get response error, http code: " + res.Status + ", body: " + string(body))
	}

	var lineVerifyToken domain.LineVerifyToken
	if err := json.Unmarshal(body, &lineVerifyToken); err != nil {
		return nil, errors.Wrap(err, "json unmarshal failed")
	}

	return &lineVerifyToken, nil
}
