package http

import (
	"encoding/json"
	"fmt"
	httpPKG "net/http"

	"context"
	"net/http"
)

type ErrorCode struct {
	HTTPCode  int    `json:"http_code"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	CallStack string `json:"-"`
}

func (e ErrorCode) Error() string {
	errorStr, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(errorStr)
}

const (
	Default   = 0
	RateLimit = 1
)

var errorCodes = map[int]map[int]string{
	httpPKG.StatusTooManyRequests: {
		RateLimit: "rate limit error. expiry: %d",
	},
	httpPKG.StatusNotFound: {
		Default: "not found",
	},
	httpPKG.StatusInternalServerError: {
		Default: "internal error",
	},
}

func CreateErrorHTTPCodeWithCode(httpCode, code int, args ...any) *ErrorCode {
	resHttpCode := httpPKG.StatusInternalServerError
	resCode := Default
	resMessage := errorCodes[httpPKG.StatusInternalServerError][Default]
	if httpErrorCodes, ok := errorCodes[httpCode]; ok {
		if errorCodes, ok := httpErrorCodes[code]; ok {
			resHttpCode = httpCode
			resCode = code
			resMessage = fmt.Sprintf(errorCodes, args...)
		}
	}
	return &ErrorCode{
		HTTPCode: resHttpCode,
		Code:     resCode,
		Message:  resMessage,
	}
}

func CreateErrorHTTPCode(httpCode int) *ErrorCode {
	return CreateErrorHTTPCodeWithCode(httpCode, Default)
}

func DecodeErrorCode(err error) *ErrorCode {
	errorCode := new(ErrorCode)
	if err := json.Unmarshal([]byte(err.Error()), errorCode); err != nil {
		errorCode = CreateErrorHTTPCode(httpPKG.StatusInternalServerError)
	}

	errorCode.CallStack = fmt.Sprintf("%+v", err)

	return errorCode
}

func EncodeErrorResponse() func(ctx context.Context, err error, w http.ResponseWriter) {
	return func(ctx context.Context, err error, w http.ResponseWriter) {
		if err == nil {
			panic("encodeError with nil error")
		}

		ctx = CustomAfterCtx(ctx, w)

		errorCode := DecodeErrorCode(err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(errorCode.HTTPCode)
		json.NewEncoder(w).Encode(errorCode)
	}
}
