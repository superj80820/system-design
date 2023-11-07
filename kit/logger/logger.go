package logger

import (
	"context"
	"os"

	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"
	"go.uber.org/zap"
)

type Logger struct {
	*zap.Logger
}

type Field = zap.Field

func NewLogger(path string) (*Logger, error) {
	os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0666)
	c := zap.NewProductionConfig()
	c.OutputPaths = []string{"stdout", path}
	logger, err := c.Build()
	if err != nil {
		return nil, errors.Wrap(err, "create logger failed")
	}
	return &Logger{logger}, err
}

func (l *Logger) WithMetadata(ctx context.Context) *Logger {
	return &Logger{l.Logger.With(zap.String("trace-id", httpKit.GetTraceID(ctx)))}
}

func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}
