package custom_logger

import (
	"errors"
	"fmt"
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

func (s *sentryErrorLogger) Print(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprint(v...)))
	s.Logger.Print(v...)
}

func (s *sentryErrorLogger) Printf(format string, v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintf(format, v...)))
	s.Logger.Printf(format, v...)
}
func (s *sentryErrorLogger) Println(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintln(v...)))
	s.Logger.Println(v...)
}

func (s *sentryErrorLogger) Fatal(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprint(v...)))
	s.Logger.Fatal(v...)
}
func (s *sentryErrorLogger) Fatalf(format string, v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintf(format, v...)))
	s.Logger.Fatalf(format, v...)
}
func (s *sentryErrorLogger) Fatalln(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintln(v...)))
	s.Logger.Fatalln(v...)
}

func (s *sentryErrorLogger) Panic(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprint(v...)))
	s.Logger.Panic(v...)
}
func (s *sentryErrorLogger) Panicf(format string, v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintf(format, v...)))
	s.Logger.Panicf(format, v...)
}
func (s *sentryErrorLogger) Panicln(v ...interface{}) {
	sentry.CaptureException(errors.New(fmt.Sprintln(v...)))
	s.Logger.Panicln(v...)
}
