package asynctask

import (
	"encoding/json"
	"fmt"
	"github.com/RichardKnop/logging"
	"github.com/getsentry/sentry-go"
	"github.com/shanbay/gobay/extensions/sentryext/custom_err"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

var _ logging.LoggerInterface = (*sentryErrorLogger)(nil)

const (
	flag  = log.Ldate | log.Ltime
	depth = 3
)

func getCaller() string {
	_, fn, line, ok := runtime.Caller(depth)
	if !ok {
		fn = "???"
		line = 1
	}
	return fmt.Sprintf("%s:%d ", filepath.Base(fn), line)
}

type sentryErrorLogger struct {
	name   string
	logger logging.LoggerInterface
}

func NewSentryErrorLogger(projectName string) *sentryErrorLogger {
	return &sentryErrorLogger{
		logger: log.New(os.Stderr, "ERROR: ", flag),
		name:   fmt.Sprintf("Go AsyncTask: %s has Error ", projectName),
	}
}

func (s *sentryErrorLogger) captureCustomException(err string, extras ...interface{}) {
	extraMap := map[string]string{}
	for i, extra := range extras {
		if val, err := json.Marshal(extra); err == nil {
			extraMap[strconv.Itoa(i)] = string(val)
		}
	}
	sentry.CaptureException(&custom_err.CustomComplexError{Message: s.name + err, MoreData: extraMap})
}

func (s *sentryErrorLogger) Print(v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Print(logIfaces...)
}
func (s *sentryErrorLogger) Printf(format string, v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Printf("%s"+format, logIfaces...)
}
func (s *sentryErrorLogger) Println(v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Println(logIfaces...)
}

func (s *sentryErrorLogger) Fatal(v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Fatal(logIfaces...)
}
func (s *sentryErrorLogger) Fatalf(format string, v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Fatalf("%s"+format, logIfaces...)
}
func (s *sentryErrorLogger) Fatalln(v ...interface{}) {
	errLocation := getCaller()
	s.captureCustomException(errLocation, v...)
	logIfaces := append([]interface{}{errLocation}, v...)
	s.logger.Fatalln(logIfaces...)
}

func (s *sentryErrorLogger) Panic(v ...interface{}) {
	s.Fatal(v...)
}
func (s *sentryErrorLogger) Panicf(format string, v ...interface{}) {
	s.Fatalf(format, v...)
}
func (s *sentryErrorLogger) Panicln(v ...interface{}) {
	s.Fatalln(v...)
}
