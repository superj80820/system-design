package logger

import (
	"time"

	"go.uber.org/zap"
)

func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

func String(key string, val string) Field {
	return zap.String(key, val)
}

func Error(err error) Field {
	return zap.Error(err)
}

func Int(key string, val int) Field {
	return zap.Int(key, val)
}

func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}

func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}
