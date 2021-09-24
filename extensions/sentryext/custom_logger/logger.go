package custom_logger

import (
	"log"
	"os"

	"github.com/RichardKnop/logging"
	"github.com/getsentry/sentry-go"
)

var _ logging.LoggerInterface = (*sentryErrorLogger)(nil)

const (
	flag = log.Ldate | log.Ltime
)

type sentryErrorLogger struct {
	*log.Logger
}

func NewSentryErrorLogger() *sentryErrorLogger {
	return &sentryErrorLogger{
		log.New(os.Stderr, "ERROR: ", flag),
	}
}

func (s *sentryErrorLogger) captureOriginException(v ...interface{}) {
	for _, vv := range v {
		if err, ok := vv.(error); ok {
			sentry.CaptureException(err)
		}
	}
}

func (s *sentryErrorLogger) Print(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Print(v...)
}

func (s *sentryErrorLogger) Printf(format string, v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Printf(format, v...)
}
func (s *sentryErrorLogger) Println(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Println(v...)
}

func (s *sentryErrorLogger) Fatal(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Fatal(v...)
}
func (s *sentryErrorLogger) Fatalf(format string, v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Fatalf(format, v...)
}
func (s *sentryErrorLogger) Fatalln(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Fatalln(v...)
}

func (s *sentryErrorLogger) Panic(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Panic(v...)
}
func (s *sentryErrorLogger) Panicf(format string, v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Panicf(format, v...)
}
func (s *sentryErrorLogger) Panicln(v ...interface{}) {
	s.captureOriginException(v...)
	s.Logger.Panicln(v...)
}
