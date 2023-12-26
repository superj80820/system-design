package ocr

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/superj80820/system-design/domain"

	"github.com/pkg/errors"
)

var _ domain.OCRService = (*ocr)(nil)

type ocr struct {
	url string
}

func CreateOCR(url string) domain.OCRService {
	return &ocr{
		url: url,
	}
}

func (o *ocr) OCR(imageRawData string) (*domain.OCRResponse, error) {
	url := o.url + "/ocr"
	method := "POST"

	client := &http.Client{}

	payloadMap := make(map[string]interface{})
	payloadMap["svg"] = imageRawData
	payloadJsonMarshal, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, errors.Wrap(err, "marshal payload failed")
	}
	payload := bytes.NewBuffer(payloadJsonMarshal)
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Add("content-type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	var ocrResponse domain.OCRResponse
	if err := json.Unmarshal(body, &ocrResponse); err != nil {
		return nil, errors.Wrap(err, "unmarshal json failed, raw data: "+imageRawData+", raw data length: "+strconv.Itoa(len(imageRawData))+", response: "+string(body))
	}

	return &ocrResponse, nil
}
