package code

import (
	"encoding/json"
	"fmt"
	httpPKG "net/http"

	"context"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// TODO: gRPC convert

type errorCode struct {
	GeneralCode int    `json:"-"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
	OriginError error  `json:"-"`
	CallStack   string `json:"-"`
}

func CreateHTTPError(err *errorCode) *httpErrorCode {
	return &httpErrorCode{
		HTTPCode:  err.GeneralCode,
		errorCode: err,
	}
}

type httpErrorCode struct {
	HTTPCode int `json:"http_code"`
	*errorCode
}

func CreateWebsocketError(ctx context.Context, err *errorCode) *websocketErrorCode {
	var websocketCode int
	if code, ok := generalCodeToWSCode[err.GeneralCode]; ok {
		websocketCode = code
	} else {
		websocketCode = websocket.CloseInternalServerErr
	}

	return &websocketErrorCode{
		WebsocketCode: websocketCode,
		Metadata:      &websocketErrorCodeMetadata{},
		errorCode:     err,
	}
}

var generalCodeToWSCode = map[int]int{
	httpPKG.StatusTooManyRequests:     websocket.CloseInvalidFramePayloadData,
	httpPKG.StatusNotFound:            websocket.CloseUnsupportedData,
	httpPKG.StatusInternalServerError: websocket.CloseInternalServerErr,
	httpPKG.StatusBadRequest:          websocket.CloseUnsupportedData,
	httpPKG.StatusUnauthorized:        websocket.CloseUnsupportedData,
	httpPKG.StatusForbidden:           websocket.CloseUnsupportedData,
}

type websocketErrorCode struct {
	WebsocketCode int                         `json:"ws_code"`
	Metadata      *websocketErrorCodeMetadata `json:"metadata,omitempty"`
	*errorCode
}

type websocketErrorCodeMetadata struct{}

func (e errorCode) Error() string {
	errorStr, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(errorStr)
}

func (e *errorCode) AddErrorMetaData(err error) *errorCode {
	e.OriginError = err
	e.CallStack = fmt.Sprintf("%+v", err)
	return e
}

func (e *errorCode) AddCode(code int, args ...any) *errorCode {
	if httpErrorCodes, ok := errorCodes[e.GeneralCode]; ok {
		if errorCodes, ok := httpErrorCodes[code]; ok {
			e.Code = code
			e.Message = fmt.Sprintf(errorCodes, args...)
		}
	}
	return e
}

const (
	Default         = 0
	RateLimit       = 1
	InvalidBody     = 2
	Expired         = 3
	Revoke          = 4
	PasswordInvalid = 5
)

var errorCodes = map[int]map[int]string{
	httpPKG.StatusTooManyRequests: {
		Default:   "too many requests",
		RateLimit: "rate limit error. expiry: %d",
	},
	httpPKG.StatusNotFound: {
		Default: "not found",
	},
	httpPKG.StatusInternalServerError: {
		Default: "internal error",
	},
	httpPKG.StatusBadRequest: {
		Default:     "bad request",
		InvalidBody: "invalid body",
	},
	httpPKG.StatusUnauthorized: {
		Default:         "unauthorized",
		Expired:         "expired",
		PasswordInvalid: "password invalid",
	},
	httpPKG.StatusForbidden: {
		Default: "forbidden",
	},
}

type errorCodeOption func(*errorCode)

func CreateErrorCode(code int, options ...errorCodeOption) *errorCode {
	resCode := httpPKG.StatusInternalServerError
	resMessage := errorCodes[httpPKG.StatusInternalServerError][Default]
	if codes, ok := errorCodes[code]; ok {
		resCode = code

		if errorCodes, ok := codes[Default]; ok {
			resMessage = errorCodes
		}
	}

	errorCode := errorCode{
		GeneralCode: resCode,
		Code:        Default,
		Message:     resMessage,
	}

	for _, option := range options {
		option(&errorCode)
	}

	return &errorCode
}

func ParseErrorCode(err error) *errorCode {
	causeErr := errors.Cause(err)
	switch errorCode := causeErr.(type) {
	case *errorCode:
		return errorCode
	}

	errorCode := CreateErrorCode(httpPKG.StatusInternalServerError).AddErrorMetaData(err)

	return errorCode
}
