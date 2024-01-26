package logger

import (
	"context"
	"os"

	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	WithMetadata(ctx context.Context) Logger
	With(fields ...Field) Logger

	GetLevel() Level
}

type LoggerWrapper struct {
	*zap.Logger
	Level Level

	noStdout bool
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

type Option func(*LoggerWrapper)

func NoStdout(l *LoggerWrapper) {
	l.noStdout = true
}

func NewLogger(path string, level Level, options ...Option) (Logger, error) {
	logger := LoggerWrapper{Level: level}

	for _, option := range options {
		option(&logger)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, errors.Wrap(err, "open file failed")
	}
	loggerConfig := zap.NewProductionEncoderConfig()

	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	var teeCore zapcore.Core
	if logger.noStdout {
		teeCore = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(loggerConfig), zapcore.Lock(zapcore.AddSync(file)), zap.InfoLevel),
		)
	} else {
		teeCore = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(loggerConfig), zapcore.Lock(zapcore.AddSync(file)), zap.InfoLevel),
			zapcore.NewCore(zapcore.NewConsoleEncoder(consoleConfig), zapcore.Lock(os.Stdout), zap.InfoLevel),
		)
	}
	zapLogger := zap.New(teeCore)

	logger.Logger = zapLogger

	return &logger, nil
}

func (l *LoggerWrapper) WithMetadata(ctx context.Context) Logger {
	return &LoggerWrapper{Logger: l.Logger.With(zap.String("trace-id", httpKit.GetTraceID(ctx))), Level: l.Level} // TODO
}

func (l *LoggerWrapper) With(fields ...Field) Logger {
	return &LoggerWrapper{Logger: l.Logger.With(fields...), Level: l.Level}
}

func (l *LoggerWrapper) GetLevel() Level {
	return l.Level
}
