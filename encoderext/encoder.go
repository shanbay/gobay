package encoderext

import (
	"errors"
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

func (e *Encoder) EncodeMap(data map[string]interface{}, excludedFields []string) (map[string]interface{}, error) {
	resData := map[string]interface{}{}
	fieldsMap := getFieldsMap(excludedFields)

	for key, value := range data {
		if value == nil || fieldsMap[key] {
			continue
		}
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Map:
			if vMap, ok := value.(map[string]interface{}); ok {
				enSub, err := e.EncodeMap(vMap, excludedFields)
				if err != nil {
					return nil, err
				}
				resData[key] = enSub
			} else {
				resData[key] = value
			}
		case reflect.Array, reflect.Slice:
			enSub, err := e.EncodeSlice(value, excludedFields)
			if err != nil {
				return nil, err
			}
			resData[key] = enSub
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
	return resData, nil
}

func (e *Encoder) EncodeSlice(arr interface{}, excludedFields []string) (interface{}, error) {
	arrV := reflect.ValueOf(arr)
	if arrV.Kind() != reflect.Array && arrV.Kind() != reflect.Slice {
		err := errors.New("Only accept Array or Slice type!")
		return nil, err
	}
	res := []interface{}{}

	for i := 0; i < arrV.Len(); i++ {
		value := arrV.Index(i).Interface()
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			enSub, err := e.EncodeSlice(value, excludedFields)
			if err != nil {
				return nil, err
			}
			res = append(res, enSub)
		case reflect.Map:
			if vMap, ok := value.(map[string]interface{}); ok {
				enSub, err := e.EncodeMap(vMap, excludedFields)
				if err != nil {
					return nil, err
				}
				res = append(res, enSub)
			} else {
				res = append(res, value)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			res = append(res, e.Pk2str(uint64(v.Int())))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			res = append(res, e.Pk2str(uint64(v.Uint())))
		default:
			res = append(res, value)
		}
	}
	return res, nil
}

func (e *Encoder) CanDecode(value string) bool {
	for i := range value {
		if strings.IndexByte(e.encoder.opt.Alphabet, value[i]) == -1 {
			return false
		}
	}
	return true
}

func (e *Encoder) DecodeMap(data interface{}) (interface{}, error) {
	dataV, ok := data.(map[string]interface{})
	if !ok {
		err := errors.New("Only accept Map with type map[string]interface{} !")
		return nil, err
	}
	resData := map[string]interface{}{}

	for key, value := range dataV {
		if value == nil {
			continue
		}
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Map:
			deSub, err := e.DecodeMap(value)
			if err != nil {
				return nil, err
			}
			resData[key] = deSub
		case reflect.Array, reflect.Slice:
			deSub, err := e.DecodeSlice(value)
			if err != nil {
				return nil, err
			}
			resData[key] = deSub
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
	return resData, nil
}

func (e *Encoder) DecodeSlice(arr interface{}) (interface{}, error) {
	arrV := reflect.ValueOf(arr)
	if arrV.Kind() != reflect.Array && arrV.Kind() != reflect.Slice {
		err := errors.New("Only accept Array or Slice type!")
		return nil, err
	}
	res := []interface{}{}

	for i := 0; i < arrV.Len(); i++ {
		value := arrV.Index(i).Interface()
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			deSub, err := e.DecodeSlice(value)
			if err != nil {
				return nil, err
			}
			res = append(res, deSub)
		case reflect.Map:
			if vMap, ok := value.(map[string]interface{}); ok {
				deSub, err := e.DecodeMap(vMap)
				if err != nil {
					return nil, err
				}
				res = append(res, deSub)
			} else {
				res = append(res, value)
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
	return res, nil
}
