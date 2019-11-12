package	sensorext


import (
	"strings"
	"reflect"
)

const (
	idStr     	= "id"
)

func (encoder *UrlEncoder) Pk2str(value uint64) string {
	return encoder.EncodeURL(value)
}

func (encoder *UrlEncoder) Str2pk(value string) uint64 {
	return encoder.DecodeURL(value)
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

func (encoder *UrlEncoder) EncodeMap(data map[string]interface{}, excludedFields []string) map[string]interface{}{
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
					resData[key] = encoder.EncodeSlice(v)
				}
			case reflect.Uint64:
				if key == idStr || strings.HasSuffix(key, idStr){
					resData[key] = encoder.Pk2str(value.(uint64))
				}
		}
	}
	return resData
}

func (encoder *UrlEncoder) EncodeSlice(arr []uint64) []string {
	res := []string{}
	for _, value := range(arr){
		res = append(res, encoder.Pk2str(value))
	}
	return res
}

func (encoder *UrlEncoder) DecodeMap(data map[string]interface{}) map[string]interface{}{
	resData := mapDeepCopy(data)
	for key, value := range(resData){
		if value == nil {
			continue
		}
		kind := reflect.TypeOf(value).Kind()
		switch kind {
			case reflect.Array, reflect.Slice:
				if v, ok:= value.([]string); ok{
					resData[key] = encoder.DecodeSlice(v)
				}
			case reflect.String:
				if key == idStr || strings.HasSuffix(key, idStr){
					resData[key] = encoder.Str2pk(value.(string))
				}
		}
	}
	return resData
}

func (encoder *UrlEncoder) DecodeSlice(arr []string) []uint64 {
	res := []uint64{}
	for _, value := range(arr){
		res = append(res, encoder.Str2pk(value))
	}
	return res
}