package domain

import "github.com/pkg/errors"

var (
	ErrNoop        = errors.New("noop")
	ErrNoData      = errors.New("no data")
	ErrAlreadyDone = errors.New("already done")
	ErrForbidden   = errors.New("forbidden")
	ErrInvalidData = errors.New("invalid data")
	ErrDuplicate   = errors.New("duplicate")
	ErrExpired     = errors.New("expired")
	ErrRateLimit   = errors.New("rate limit")

	ErrNormalContinue  = errors.New("normal continue")
	ErrWarningContinue = errors.New("warning continue")
	ErrPanicContinue   = errors.New("panic continue")
)
