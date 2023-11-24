package code

import (
	"net/http"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
)

func TestErrorCode(t *testing.T) {
	errorCodeNotFound := CreateErrorCode(http.StatusNotFound)
	assert.Equal(t, errorCodeNotFound, ParseErrorCode(errorCodeNotFound))

	for _, testCase := range []struct {
		message          string
		errString        string
		isExistCallStack bool
		errorCode        *errorCode
	}{
		{
			message:          "bad request",
			errString:        `{"http_code":400,"code":0,"message":"bad request"}`,
			isExistCallStack: false,
			errorCode:        CreateErrorCode(http.StatusBadRequest),
		},
		{
			message:          "rate limit error. expiry: 3",
			errString:        `{"http_code":429,"code":1,"message":"rate limit error. expiry: 3"}`,
			isExistCallStack: false,
			errorCode:        CreateErrorCode(http.StatusTooManyRequests).AddCode(RateLimit, 3),
		},
		{
			message:          "internal error",
			errString:        `{"http_code":500,"code":0,"message":"internal error"}`,
			isExistCallStack: true,
			errorCode:        ParseErrorCode(errors.New("unknown error")),
		},
	} {
		assert.Equal(t, testCase.message, testCase.errorCode.Message)
		assert.Equal(t, testCase.errString, testCase.errorCode.Error())
		assert.Equal(t, testCase.isExistCallStack, len(testCase.errorCode.CallStack) != 0)
	}
}
