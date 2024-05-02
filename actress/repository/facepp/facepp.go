package facepp

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	netURL "net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type facePlusPlusRepo struct {
	faceppKey    string
	faceppSecret string
	apiURL       string
	proxyURL     string
}
type Option func(*facePlusPlusRepo)

type facePlusPlusErrorResponse struct {
	ErrorMessage string `json:"error_message"`
}

func UseProxyURL(proxyURL string) Option {
	return func(fppr *facePlusPlusRepo) {
		fppr.proxyURL = proxyURL
	}
}

func CreateFacePlusPlusRepo(apiURL, faceppKey, faceppSecret string, options ...Option) domain.FacePlusPlusRepo {
	f := &facePlusPlusRepo{
		faceppKey:    faceppKey,
		faceppSecret: faceppSecret,
		apiURL:       apiURL,
	}
	for _, option := range options {
		option(f)
	}
	return f
}

func (f *facePlusPlusRepo) Search(faceSetID string, image []byte) (*domain.FaceSearch, error) {
	url := f.apiURL + "/facepp/v3/search"
	method := "POST"

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	imagePart, err := writer.CreateFormFile("image_file", "image")
	if err != nil {
		return nil, errors.Wrap(err, "create form file failed")
	}
	_, err = io.Copy(imagePart, bytes.NewReader(image))
	if err != nil {
		return nil, errors.Wrap(err, "io copy failed")
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "close writer failed")
	}

	req, err := http.NewRequest(method, url, &payload)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	q := req.URL.Query()
	q.Add("api_key", f.faceppKey)
	q.Add("api_secret", f.faceppSecret)
	q.Add("faceset_token", faceSetID)
	req.URL.RawQuery = q.Encode()

	client, err := f.createCustomHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "create custom http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	if res.StatusCode == http.StatusForbidden {
		var errResponse facePlusPlusErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		if errResponse.ErrorMessage == "CONCURRENCY_LIMIT_EXCEEDED" {
			return nil, errors.Wrap(domain.ErrRateLimit, "get error response, status code: "+res.Status+", body: "+string(body))
		}
		return nil, errors.Wrap(domain.ErrForbidden, "get error response, status code: "+res.Status+", body: "+string(body))
	} else if res.StatusCode == http.StatusBadRequest {
		var errResponse facePlusPlusErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		if errResponse.ErrorMessage == "EMPTY_FACESET" {
			return nil, errors.Wrap(domain.ErrNoData, "get empty face set, face set token: "+faceSetID)
		}
		return nil, errors.Wrap(domain.ErrInvalidData, "get error response, status code: "+res.Status+", body: "+string(body))
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get error response, body: " + string(body) + ", status code: " + res.Status)
	}

	var faceSearch domain.FaceSearch
	if err := json.Unmarshal(body, &faceSearch); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	return &faceSearch, nil
}

func (f *facePlusPlusRepo) Add(faceSetID string, faceTokens []string) (*domain.FaceAdd, error) {
	url := f.apiURL + "/facepp/v3/faceset/addface"
	method := "POST"

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}

	q := req.URL.Query()
	q.Add("api_key", f.faceppKey)
	q.Add("api_secret", f.faceppSecret)
	q.Add("faceset_token", faceSetID)
	q.Add("face_tokens", strings.Join(faceTokens, ","))
	req.URL.RawQuery = q.Encode()

	client, err := f.createCustomHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "create custom http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	if res.StatusCode == http.StatusForbidden {
		var errResponse facePlusPlusErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		if errResponse.ErrorMessage == "CONCURRENCY_LIMIT_EXCEEDED" {
			return nil, errors.Wrap(domain.ErrRateLimit, "get error response, status code: "+res.Status+", body: "+string(body))
		}
		return nil, errors.Wrap(domain.ErrForbidden, "get error response, status code: "+res.Status+", body: "+string(body))
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get error response, body: " + string(string(body)))
	}

	var faceAdd domain.FaceAdd
	if err := json.Unmarshal(body, &faceAdd); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	if len(faceAdd.FailureDetail) != 0 {
		if faceAdd.FailureDetail[0]["reason"] == "INVALID_FACE_TOKEN" {
			return nil, errors.Wrap(domain.ErrInvalidData, "invalid face token")
		}
		return nil, errors.New("get failure detail")
	}

	if faceAdd.FaceAdded == 0 {
		return nil, errors.Wrap(domain.ErrAlreadyDone, "already added face")
	}

	return &faceAdd, nil
}

func (f *facePlusPlusRepo) Detect(image []byte) (*domain.FaceDetect, error) {
	url := f.apiURL + "/facepp/v3/detect"
	method := "POST"

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	imagePart, err := writer.CreateFormFile("image_file", "image")
	if err != nil {
		return nil, errors.Wrap(err, "create form file failed")
	}
	_, err = io.Copy(imagePart, bytes.NewReader(image))
	if err != nil {
		return nil, errors.Wrap(err, "io copy failed")
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "close writer failed")
	}

	req, err := http.NewRequest(method, url, &payload)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	q := req.URL.Query()
	q.Add("api_key", f.faceppKey)
	q.Add("api_secret", f.faceppSecret)
	req.URL.RawQuery = q.Encode()

	client, err := f.createCustomHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "create custom http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	if res.StatusCode == http.StatusForbidden {
		var errResponse facePlusPlusErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		if errResponse.ErrorMessage == "CONCURRENCY_LIMIT_EXCEEDED" {
			return nil, errors.Wrap(domain.ErrRateLimit, "get error response, status code: "+res.Status+", body: "+string(body))
		}
		return nil, errors.Wrap(domain.ErrForbidden, "get error response, status code: "+res.Status+", body: "+string(body))
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get error response, status code: " + res.Status + ", body: " + string(body))
	}

	var faceDetect domain.FaceDetect
	if err := json.Unmarshal(body, &faceDetect); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	return &faceDetect, nil
}

func (f *facePlusPlusRepo) GetFaceSetDetail(faceSetID string) (*domain.FaceSetDetail, error) {
	url := f.apiURL + "/facepp/v3/faceset/getdetail"
	method := "POST"

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}

	q := req.URL.Query()
	q.Add("api_key", f.faceppKey)
	q.Add("api_secret", f.faceppSecret)
	q.Add("faceset_token", faceSetID)
	req.URL.RawQuery = q.Encode()

	client, err := f.createCustomHTTPClient()
	if err != nil {
		return nil, errors.Wrap(err, "create custom http client")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read response failed")
	}

	if res.StatusCode == http.StatusForbidden {
		var errResponse facePlusPlusErrorResponse
		if err := json.Unmarshal(body, &errResponse); err != nil {
			return nil, errors.Wrap(err, "unmarshal failed")
		}
		if errResponse.ErrorMessage == "CONCURRENCY_LIMIT_EXCEEDED" {
			return nil, errors.Wrap(domain.ErrRateLimit, "get error response, status code: "+res.Status+", body: "+string(body))
		}
		return nil, errors.Wrap(domain.ErrForbidden, "get error response, status code: "+res.Status+", body: "+string(body))
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("get error response, body: " + string(string(body)))
	}

	var faceSetDetail domain.FaceSetDetail
	if err := json.Unmarshal(body, &faceSetDetail); err != nil {
		return nil, errors.Wrap(err, "unmarshal failed")
	}

	return &faceSetDetail, nil
}

func (f *facePlusPlusRepo) IsFaceSetFull(faceSetID string) (bool, error) {
	faceSetDetail, err := f.GetFaceSetDetail(faceSetID)
	if err != nil {
		return false, errors.Wrap(err, "get face set detail failed")
	}
	if faceSetDetail.FaceCount == 10000 {
		return true, nil
	} else if faceSetDetail.FaceCount > 10000 {
		return false, errors.New("unexpected size, size: " + strconv.Itoa(faceSetDetail.FaceCount))
	}
	return false, nil
}

func (f *facePlusPlusRepo) createCustomHTTPClient() (*http.Client, error) {
	client := &http.Client{}
	if f.proxyURL != "" {
		proxyURL, err := netURL.Parse(f.proxyURL)
		if err != nil {
			return nil, errors.Wrap(err, "parse proxy failed")
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}
	return client, nil
}
