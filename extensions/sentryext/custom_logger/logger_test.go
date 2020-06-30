package custom_logger

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_logger(t *testing.T) {
	assert := assert.New(t)
	logger := NewSentryErrorLogger()

	var buf bytes.Buffer
	logger.SetOutput(&buf)

	str1 := "This is a error"
	logger.Print(str1)
	now := time.Now().Format("2006/01/02 15:04:05")
	logedMsg := string(buf.Bytes())
	assert.Containsf(logedMsg, str1, "error message %s", "formatted")
	assert.Containsf(logedMsg, now, "error message %s", "formatted")
}
