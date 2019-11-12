package	encoderext


import (
	"strings"
	"reflect"
	"github.com/shanbay/gobay"
)

const (
	idStr     	= "id"
	alphabet	= "short_url_alphabet"
)

// Encoder extension
type Encoder struct {
	NS			string
	app 		*gobay.Application
	encoder		*UrlEncoder
}


// Init extension interface
func (e *Encoder) Init(app *gobay.Application) error {
	e.app = app
	config := app.Config()
	if e.NS != "" {
		config = config.Sub(e.NS)
	}
	e.encoder = NewURLEncoder(&Options{Alphabet: config.GetString(alphabet)})
	
	return nil
}

// Close implements Extension interface
func (e *Encoder) Close() error {
	return nil
}

// Object implements Extension interface
func (e *Encoder) Object() interface{} {
	return e
}

// Application implements Extension interface
func (e *Encoder) Application() *gobay.Application {
	return e.app
}

func (e *Encoder) Pk2str(value uint64) string {
	return e.encoder.EncodeURL(value)
}

func (e *Encoder) Str2pk(value string) uint64 {
	return e.encoder.DecodeURL(value)
}

func getFieldsMap(fields []string) map[string]bool {
	fieldsMap := make(map[string]bool)
	for _, field := range fields {
		fieldsMap[field] = true
	}
	return fieldsMap
}

func mapDeepCopy(value map[string]interface{}) map[string]interface{} {
	ncopy := deepCopy(value)
	if nmap, ok := ncopy.(map[string]interface{}); ok {
		return nmap
	}
	return nil
}

func deepCopy(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		newMap := make(map[string]interface{})
		for k, v := range valueMap {
			newMap[k] = deepCopy(v)
		}
		return newMap
	} else if valueSlice, ok := value.([]interface{}); ok {
		newSlice := make([]interface{}, len(valueSlice))
		for k, v := range valueSlice {
			newSlice[k] = deepCopy(v)
		}
		return newSlice
	}
	return value
}

func (e *Encoder) EncodeMap(data map[string]interface{}, excludedFields []string) map[string]interface{}{
	resData := mapDeepCopy(data)
	fieldsMap := getFieldsMap(excludedFields)
	for key, value := range(resData){
		if value == nil || fieldsMap[key] {
			continue
		}
		kind := reflect.TypeOf(value).Kind()
		switch kind {
			case reflect.Array, reflect.Slice:
				if v, ok:= value.([]uint64); ok{
					resData[key] = e.EncodeSlice(v)
				}
			case reflect.Uint64:
				if key == idStr || strings.HasSuffix(key, idStr){
					resData[key] = e.Pk2str(value.(uint64))
				}
		}
	}
	return resData
}

func (e *Encoder) EncodeSlice(arr []uint64) []string {
	res := []string{}
	for _, value := range(arr){
		res = append(res, e.Pk2str(value))
	}
	return res
}

func (e *Encoder) DecodeMap(data map[string]interface{}) map[string]interface{}{
	resData := mapDeepCopy(data)
	for key, value := range(resData){
		if value == nil {
			continue
		}
		kind := reflect.TypeOf(value).Kind()
		switch kind {
			case reflect.Array, reflect.Slice:
				if v, ok:= value.([]string); ok{
					resData[key] = e.DecodeSlice(v)
				}
			case reflect.String:
				if key == idStr || strings.HasSuffix(key, idStr){
					resData[key] = e.Str2pk(value.(string))
				}
		}
	}
	return resData
}

func (e *Encoder) DecodeSlice(arr []string) []uint64 {
	res := []uint64{}
	for _, value := range(arr){
		res = append(res, e.Str2pk(value))
	}
	return res
}