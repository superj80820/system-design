package util

import (
	"fmt"
	"math"

	"github.com/pkg/errors"
)

func SafeInt64ToInt(value int64) (int, error) {
	if value < math.MinInt || value > math.MaxInt {
		return 0, errors.New(fmt.Sprintf("value %d is out of int range", value))
	}
	return int(value), nil
}

func SafeUint64ToInt(value uint64) (int, error) {
	if value > uint64(math.MaxInt) {
		return 0, errors.New(fmt.Sprintf("value %d is out of int range", value))
	}
	return int(value), nil
}
