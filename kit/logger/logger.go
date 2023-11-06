package logger

import (
	"os"

	log "github.com/go-kit/kit/log/logrus"
	logKit "github.com/go-kit/log"
	"github.com/sirupsen/logrus"
)

type Logger logKit.Logger

func NewLogger(logger logrus.FieldLogger, options ...log.Option) logKit.Logger {
	if logger == nil {
		logrusLogger := logrus.New()
		logrusLogger.Out = os.Stderr
		logrusLogger.Formatter = &logrus.JSONFormatter{}
		return log.NewLogger(logrusLogger, options...)
	}
	return log.NewLogger(logger, options...)
}
