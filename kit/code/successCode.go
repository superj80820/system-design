package code

import httpPKG "net/http"

type SuccessCode struct {
	HTTPCode int
}

func ParseResponseSuccessCode(res interface{}) *SuccessCode {
	switch successCode := res.(type) {
	case SuccessCode:
		return &successCode
	case nil:
		return &SuccessCode{HTTPCode: httpPKG.StatusNoContent}
	}
	return &SuccessCode{HTTPCode: httpPKG.StatusOK}
}
