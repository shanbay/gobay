package sensorext

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"../encoderext"
	"github.com/shanbay/gobay"
	"github.com/stretchr/testify/assert"
)

func TestSensorsData(t *testing.T) {

	filePath := fmt.Sprintf("../testdata/beat.log-%s.%s", os.Getenv("HOSTNAME"), time.Now().Format("2006-01-02"))

	assert := assert.New(t)
	sensorsData := &SensorsData{}
	encoder := &encoderext.Encoder{}

	exts := map[gobay.Key]gobay.Extension{
		"sensors": sensorsData,
		"encoder": encoder,
	}
	app, err := gobay.CreateApp("../testdata", "testing", exts)
	assert.NotNil(app)
	assert.Nil(err)

	sensorsData.encoder = encoder

	// test log when Track
	dataModel := &DataModel{
		name:   "xxxUserSomeEvent",
		fields: []string{"age", "gender", "service_id", "tags"},
	}
	userID := uint64(123)
	userIDStr := sensorsData.encoder.Pk2str(userID)
	properties := map[string]interface{}{
		"age":        18,
		"gender":     "male",
		"service_id": uint64(9),
		"tags":       []uint64{1, 2, 3},
	}
	err = dataModel.Track(*sensorsData, userID, properties)
	assert.Nil(err)

	// test log when ProfileSet
	profile := map[string]interface{}{"profile1": "test"}
	err = sensorsData.ProfileSet(userID, profile)
	assert.Nil(err)

	// test log when ProfileSetOnce
	err = sensorsData.ProfileSetOnce(userID, profile)
	assert.Nil(err)

	// test log when ProfileIncrement
	profileIncre := map[string]interface{}{"login_times": 1}
	err = sensorsData.ProfileIncrement(userID, profileIncre)
	assert.Nil(err)

	// test log when ProfileAppend
	err = sensorsData.ProfileAppend(userID, profile)
	assert.Nil(err)

	// test log when ProfileUnset
	err = sensorsData.ProfileUnset(userID, profile)
	assert.Nil(err)

	// test log when ProfileDelete
	err = sensorsData.ProfileDelete(userID)
	assert.Nil(err)

	fileN, err := os.Open(filePath)
	if err != nil {
		t.Errorf("Open file error")
	}
	defer fileN.Close()

	lines := [][]byte{}
	scanner := bufio.NewScanner(fileN)
	for scanner.Scan() {
		lines = append(lines, []byte(scanner.Text()))
	}

	for _, line := range lines {
		m := make(map[string]interface{})
		err = json.Unmarshal(line, &m)
		if err != nil {
			fmt.Println(line)
			t.Errorf("Unmarshal failed")
		}
		assert.Equal(m["distinct_id"], userIDStr)

		propertiesRes := m["properties"].(map[string]interface{})
		assert.Equal(propertiesRes["$is_login_id"], true)

		switch m["type"] {
		case "track":
			assert.Equal(m["event"], dataModel.name)
			assert.Equal(propertiesRes["service_id"], sensorsData.encoder.Pk2str(properties["service_id"].(uint64)))
		case "profile_set", "profile_set_once", "profile_append", "profile_unset":
			for k, v := range profile {
				assert.Equal(v, propertiesRes[k])
			}
		case "profile_increment":
			for k := range profileIncre {
				assert.NotNil(propertiesRes[k])
			}
		case "profile_delete":
			fmt.Println("pass")
		default:
			t.Errorf("Unknown log")
		}
	}
}
