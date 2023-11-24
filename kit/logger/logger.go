package logger

import (
	"context"
	"os"

	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger
	Level Level
}

type (
	Field = zap.Field
	Level = zapcore.Level
)

const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
)

func NewLogger(path string, level Level) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, errors.Wrap(err, "open file failed")
	}
	loggerConfig := zap.NewProductionEncoderConfig()

	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	teeCore := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(loggerConfig), zapcore.Lock(zapcore.AddSync(file)), zap.InfoLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(consoleConfig), zapcore.Lock(os.Stdout), zap.InfoLevel),
	)
	logger := zap.New(teeCore)
	return &Logger{Logger: logger, Level: level}, nil
}

func (l *Logger) WithMetadata(ctx context.Context) *Logger {
	return &Logger{Logger: l.Logger.With(zap.String("trace-id", httpKit.GetTraceID(ctx))), Level: l.Level} // TODO
}

func (l *Logger) With(fields ...Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...), Level: l.Level}
}
