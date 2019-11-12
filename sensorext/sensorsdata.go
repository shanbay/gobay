package	sensorext

import (
	"os"
	"fmt"
	"errors"
	"path/filepath"
	"../encoderext"
	"github.com/shanbay/gobay"
	sdk "github.com/sensorsdata/sa-sdk-go"
)

const (
	sensorNs     	= "sensor_ns"
	sensorPath      = "sensor_path"
	sensorFilename  = "sensor_filename"
	nsProperty  	= "ns"
	alphabet		= "short_url_alphabet"
	isLoginID 		= true
)


// SensorsData extension
type SensorsData struct {
	NS 			string
	app 		*gobay.Application
	encoder 	*encoderext.Encoder
	trackNs 	string
	sa			sdk.SensorsAnalytics
}


// Init extension interface
func (s *SensorsData) Init(app *gobay.Application) error {
	s.app = app
	config := app.Config()
	if s.NS != "" {
		config = config.Sub(s.NS)
	}

	s.trackNs = config.GetString(sensorNs)
	dirPath := config.GetString(sensorPath)
	filename := fmt.Sprintf("beat.log-%s", os.Getenv("HOSTNAME"))

	filePath := filepath.Join(dirPath, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		os.MkdirAll(filePath, os.ModePerm)
	}
	
	consumer, err := sdk.InitConcurrentLoggingConsumer(filePath, false)
	if err != nil {
		fmt.Println(err)
		return err
	}
	s.sa = sdk.InitSensorsAnalytics(consumer, "", false)	
	return nil
}

// Close implements Extension interface
func (s *SensorsData) Close() error {
	return s.sa.C.Close()
}

// Object implements Extension interface
func (s *SensorsData) Object() interface{} {
	return s
}

// Application implements Extension interface
func (s *SensorsData) Application() *gobay.Application {
	return s.app
}

// Track event
func (s *SensorsData) Track(distinctID uint64, event string, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	// register ns property
	encodedProperties[nsProperty] = s.trackNs
	return s.sa.Track(distinctIDStr, event, encodedProperties, isLoginID)
}

// ProfileSet for user
func (s *SensorsData) ProfileSet(distinctID uint64, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	return s.sa.ProfileSet(distinctIDStr, encodedProperties, isLoginID)
}

// ProfileSetOnce for "first_time" property
func (s *SensorsData) ProfileSetOnce(distinctID uint64, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	return s.sa.ProfileSetOnce(distinctIDStr, encodedProperties, isLoginID)
}

// ProfileIncrement for int type property
func (s *SensorsData) ProfileIncrement(distinctID uint64, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	return s.sa.ProfileIncrement(distinctIDStr, encodedProperties, isLoginID)
}

// ProfileAppend for []string type property
func (s *SensorsData) ProfileAppend(distinctID uint64, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	return s.sa.ProfileAppend(distinctIDStr, encodedProperties, isLoginID)
}

// ProfileUnset unset user property
func (s *SensorsData) ProfileUnset(distinctID uint64, properties map[string]interface{}) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	encodedProperties := s.encoder.EncodeMap(properties, []string{})
	return s.sa.ProfileUnset(distinctIDStr, encodedProperties, isLoginID)
}

// ProfileDelete delete whole user profile
func (s *SensorsData) ProfileDelete(distinctID uint64) error {
	distinctIDStr := s.encoder.Pk2str(distinctID)
	return s.sa.ProfileDelete(distinctIDStr, isLoginID)
}


// DataModel log model
type DataModel struct {
	name 	string
	fields 	[]string
}

// Track event method
func (d *DataModel) Track(s SensorsData, distinctID uint64, properties map[string]interface{}) error {
	if (d.name == "") || (len(d.fields) != len(properties)) {
		return errors.New("Invalid DataModel: lack of name or fields")
	}
	for _, field := range d.fields {
		if properties[field] == nil {
			return errors.New("Invalid DataModel lack of field: " + field)
		}
	}

	return s.Track(distinctID, d.name, properties)
}


