package domain

import "github.com/pkg/errors"

var (
	ErrNoop        = errors.New("noop")
	ErrNoData      = errors.New("no data")
	ErrAlreadyDone = errors.New("already done")
	ErrInvalidData = errors.New("invalid data")
	ErrDuplicate   = errors.New("duplicate")
	ErrExpired     = errors.New("expired")
)
