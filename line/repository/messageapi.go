package repository

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type lineMessageAPIRepo struct {
	url     string
	dataURL string
	token   string
}

func CreateLineMessageRepo(url, dataURL, token string) domain.LineMessageAPIRepo {
	return &lineMessageAPIRepo{
		url:     url,
		dataURL: dataURL,
		token:   token,
	}
}

func (l *lineMessageAPIRepo) Reply(token string, messages []string) error {
	url := l.url + "/bot/message/reply"
	method := "POST"

	payload := strings.NewReader(fmt.Sprintf(`{
		"replyToken": "%s",
		"messages": %s
	}`, token, messages))

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return errors.Wrap(err, "new request failed")
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+l.token)

	res, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "read body failed")
		}
		return errors.New(fmt.Sprintf("read body failed, body: %s", string(body)))
	}

	return nil
}

func (l *lineMessageAPIRepo) GetImage(imageID string) ([]byte, error) {
	url := l.dataURL + "/bot/message/" + imageID + "/content"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("Authorization", "Bearer "+l.token)

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read body failed")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("read body failed, body: %s", string(body)))
	}

	return body, nil
}

// VerifyLIFF implements domain.LineAPIRepo.
func (l *lineNotifyAPIRepo) VerifyLIFF(liffID, accessToken string) error {
	panic("unimplemented")
}
