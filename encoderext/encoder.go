package encoderext

import (
	"fmt"
	"github.com/shanbay/gobay"
	"reflect"
	"strings"
)

const (
	idStr    = "id"
	alphabet = "short_url_alphabet"
)

// Encoder extension
type Encoder struct {
	NS      string
	app     *gobay.Application
	encoder *UrlEncoder
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

func (e *Encoder) EncodeMap(data interface{}, excludedFields []string) interface{} {
	dataV, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("Only accept Map with type map[string]interface{} !")
		return data
	}
	resData := map[string]interface{}{}
	fieldsMap := getFieldsMap(excludedFields)

	for key, value := range dataV {
		if value == nil || fieldsMap[key] {
			continue
		}
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Map:
			resData[key] = e.EncodeMap(value, excludedFields)
		case reflect.Array, reflect.Slice:
			resData[key] = e.EncodeSlice(value, excludedFields)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if key == idStr || strings.HasSuffix(key, idStr) {
				resData[key] = e.Pk2str(uint64(v.Int()))
			} else {
				resData[key] = value
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if key == idStr || strings.HasSuffix(key, idStr) {
				resData[key] = e.Pk2str(uint64(v.Uint()))
			} else {
				resData[key] = value
			}
		default:
			resData[key] = value
		}
	}
	return resData
}

func (e *Encoder) EncodeSlice(arr interface{}, excludedFields []string) interface{} {
	arrV := reflect.ValueOf(arr)
	if arrV.Kind() != reflect.Array && arrV.Kind() != reflect.Slice {
		fmt.Println("Only accept Array or Slice type!")
		return arr
	}
	res := []interface{}{}

	for i := 0; i < arrV.Len(); i++ {
		value := arrV.Index(i).Interface()
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			res = append(res, e.EncodeSlice(value, excludedFields))
		case reflect.Map:
			if vMap, ok := value.(map[string]interface{}); ok {
				res = append(res, e.EncodeMap(vMap, excludedFields))
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			res = append(res, e.Pk2str(uint64(v.Int())))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			res = append(res, e.Pk2str(uint64(v.Uint())))
		default:
			res = append(res, value)
		}
	}
	return res
}

func (e *Encoder) CanDecode(value string) bool {
	for i := range value {
		if strings.IndexByte(e.encoder.opt.Alphabet, value[i]) == -1 {
			return false
		}
	}
	return true
}

func (e *Encoder) DecodeMap(data interface{}) interface{} {
	dataV, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("Only accept Map with type map[string]interface{} !")
		return data
	}
	resData := map[string]interface{}{}

	for key, value := range dataV {
		if value == nil {
			continue
		}
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Map:
			resData[key] = e.DecodeMap(value)
		case reflect.Array, reflect.Slice:
			resData[key] = e.DecodeSlice(value)
		case reflect.String:
			if (key == idStr || strings.HasSuffix(key, idStr)) && e.CanDecode(v.String()) {
				resData[key] = e.Str2pk(v.String())
			} else {
				resData[key] = value
			}
		default:
			resData[key] = value
		}
	}
	return resData
}

func (e *Encoder) DecodeSlice(arr interface{}) interface{} {
	arrV := reflect.ValueOf(arr)
	if arrV.Kind() != reflect.Array && arrV.Kind() != reflect.Slice {
		fmt.Println("Only accept Array or Slice type!")
		return arr
	}
	res := []interface{}{}

	for i := 0; i < arrV.Len(); i++ {
		value := arrV.Index(i).Interface()
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			res = append(res, e.DecodeSlice(value))
		case reflect.Map:
			if vMap, ok := value.(map[string]interface{}); ok {
				res = append(res, e.DecodeMap(vMap))
			}
		case reflect.String:
			if e.CanDecode(v.String()) {
				res = append(res, e.Str2pk(v.String()))
			} else {
				res = append(res, value)
			}
		default:
			res = append(res, value)
		}
	}
	return res
}
