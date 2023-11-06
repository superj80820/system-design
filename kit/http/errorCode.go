package http

import (
	"encoding/json"
	"fmt"
	httpPKG "net/http"
)

type ErrorCode struct {
	HTTPCode int    `json:"http_code"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
}

func (e ErrorCode) Error() string {
	errorStr, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(errorStr)
}

var errorCodes = map[int]map[int]*ErrorCode{
	httpPKG.StatusTooManyRequests: {
		1: {
			HTTPCode: httpPKG.StatusTooManyRequests,
			Code:     1,
			Message:  "rate limit error. last requests: %d. expiry: %d",
		},
	},
	httpPKG.StatusNotFound: {
		0: {
			HTTPCode: httpPKG.StatusNotFound,
			Code:     0,
			Message:  "not found",
		},
	},
	httpPKG.StatusInternalServerError: {
		0: {
			HTTPCode: httpPKG.StatusInternalServerError,
			Code:     0,
			Message:  "internal error",
		},
	},
}

func CreateErrorHTTPCodeWithCode(httpCode, code int, args ...any) *ErrorCode {
	if httpErrorCodes, ok := errorCodes[httpCode]; ok {
		if errorCodes, ok := httpErrorCodes[code]; ok {
			return &ErrorCode{
				HTTPCode: errorCodes.HTTPCode,
				Code:     errorCodes.Code,
				Message:  fmt.Sprintf(errorCodes.Message, args...),
			}
		}
	}
	return errorCodes[httpPKG.StatusInternalServerError][1]
}

func CreateErrorHTTPCode(httpCode int) *ErrorCode {
	return CreateErrorHTTPCodeWithCode(httpCode, 0)
}

func DecodeErrorCode(err error) *ErrorCode {
	var errorCode ErrorCode
	if err := json.Unmarshal([]byte(err.Error()), &errorCode); err != nil {
		return CreateErrorHTTPCode(httpPKG.StatusInternalServerError)
	}
	return &errorCode
}
