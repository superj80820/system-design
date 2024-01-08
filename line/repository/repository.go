package repository

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type lineRepo struct {
	url   string
	token string
}

func CreateLineRepo(url, token string) domain.LineRepo {
	return &lineRepo{
		url:   url,
		token: token,
	}
}

func (l *lineRepo) Notify(message string) error {
	return l.notifyWithToken(l.token, message)
}

func (l *lineRepo) NotifyWithToken(token, message string) error {
	return l.notifyWithToken(token, message)
}

func (l *lineRepo) notifyWithToken(token, message string) error {
	url := l.url + "/api/notify"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("message", message)
	err := writer.Close()
	if err != nil {
		return errors.Wrap(err, "write message failed")
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		return errors.Wrap(err, "create request failed")
	}
	req.Header.Add("Authorization", "Bearer "+l.token)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "read request body failed")
	}

	var resStruct domain.LineResponseOK
	if err := json.Unmarshal(body, &resStruct); err != nil {
		return errors.Wrap(err, "unmarshal json failed")
	}

	if res.StatusCode != http.StatusOK ||
		resStruct.Status != domain.LineOKStatus {
		return errors.New("response status not 200, response body: " + string(body))
	}

	return nil
}
