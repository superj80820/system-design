package logger

import (
	"context"
	"io"
	"os"

	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
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

	isUsedRotateLog bool
	maxSize         int
	maxBackups      int
	maxAge          int
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

func WithRotateLog(maxSize, maxBackups, maxAge int) func(l *LoggerWrapper) {
	return func(l *LoggerWrapper) {
		l.isUsedRotateLog = true
		l.maxSize = maxSize
		l.maxBackups = maxBackups
		l.maxAge = maxAge
	}
}

func NewLogger(path string, level Level, options ...Option) (Logger, error) {
	logger := LoggerWrapper{Level: level}

	for _, option := range options {
		option(&logger)
	}

	var (
		file io.Writer
		err  error
	)
	if logger.isUsedRotateLog {
		file = &lumberjack.Logger{
			Filename:   path,
			MaxSize:    logger.maxSize,
			MaxBackups: logger.maxBackups,
			MaxAge:     logger.maxAge,
		}
	} else {
		file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, errors.Wrap(err, "open file failed")
		}
	}

	loggerConfig := zap.NewProductionEncoderConfig()

	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	var teeCore zapcore.Core
	if logger.noStdout {
		teeCore = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(loggerConfig), zapcore.Lock(zapcore.AddSync(file)), level),
		)
	} else {
		teeCore = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(loggerConfig), zapcore.Lock(zapcore.AddSync(file)), level),
			zapcore.NewCore(zapcore.NewConsoleEncoder(consoleConfig), zapcore.Lock(os.Stdout), level),
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
