package domain

import "github.com/pkg/errors"

var (
	ErrNoop      = errors.New("noop")
	ErrNoData    = errors.New("no data")
	ErrDuplicate = errors.New("duplicate")
)
