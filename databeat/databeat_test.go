package databeat

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Example() {
	// log_models.go
	var UserRegisteredLog = DataModel{
		name:     "bayuser-user_registered",
		group:    "xyz",
		fields:   []string{"user_id", "created_at", "reg_source", "username", "nickname"},
		index_by: Monthly,
	}
	// anywhere
	UserRegisteredLog.Beat(map[string]interface{}{
		"user_id":    82274890,
		"created_at": time.Now(),
		"reg_source": "qq",
		"username":   "kindjeff",
		"nickname":   "善良的杰夫",
	})
}

func TestDatabeat(t *testing.T) {
	UserRegisteredLog := DataModel{
		name:     "bayuser-user_registered",
		group:    "xyz",
		fields:   []string{"user_id", "created_at", "reg_source", "username", "nickname"},
		index_by: Monthly,
	}

	var buf bytes.Buffer
	GetBeatLogger().SetOutput(&buf)
	defer GetBeatLogger().SetOutput(os.Stdout)

	beatTests := []struct {
		in     map[string]interface{}
		errStr string
	}{
		{
			map[string]interface{}{
				"user_id":    82274890,
				"created_at": time.Now(),
				"reg_source": "qq",
				"username":   "kindjeff",
				"nickname":   "善良的杰夫",
			},
			"",
		},
		{
			map[string]interface{}{
				"user_id":    82274890,
				"created_at": time.Now(),
				"reg_source": "qq",
				"username":   "kindjeff",
			},
			"缺少字段: nickname",
		},
		{
			map[string]interface{}{
				"user_id":    82274890,
				"created_at": time.Now(),
				"reg_source": "qq",
				"username":   "kindjeff",
				"nickname":   "善良的杰夫",
				"nonono":     1,
			},
			"传入的内容字段不合法",
		},
	}

	for _, tt := range beatTests {
		actual, err := UserRegisteredLog.Beat(tt.in)
		if tt.errStr != "" && err == nil {
			t.Errorf("Expected error '%s', got none", tt.errStr)
		} else if tt.errStr != "" && err != nil && err.Error() != tt.errStr {
			t.Errorf("Expected %s, got %s", tt.errStr, err.Error())
		} else if actual != "" {
			t.Log(buf.String())
			output := strings.Replace(buf.String(), "[DATABEAT] ", "", 1)
			mapOutput := make(map[string]interface{})
			json.Unmarshal([]byte(output), &mapOutput)
			mapReturn := make(map[string]interface{})
			json.Unmarshal([]byte(actual), &mapReturn)
			if !reflect.DeepEqual(mapOutput, mapReturn) {
				t.Errorf("Expected %s, got %s", mapReturn, mapOutput)
			}
		}
	}
}
