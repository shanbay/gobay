package asynctask

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

func Test_logger(t *testing.T) {
	assert := assert.New(t)
	projectName := "this is the Project Name"
	logger := NewSentryErrorLogger(projectName)

	var buf bytes.Buffer
	commonLogger, _ := logger.logger.(*log.Logger)
	commonLogger.SetOutput(&buf)

	str1 := "This is a error"
	logger.Print(str1)
	now := time.Now().Format("2006/01/02 15:04:05")
	logedMsg := string(buf.Bytes())
	assert.Containsf(logedMsg, str1, "error message %s", "formatted")
	assert.Containsf(logedMsg, now, "error message %s", "formatted")
}
