package encoderext

import (
	"reflect"
	"testing"

	"github.com/shanbay/gobay"
)

func TestEncoder(t *testing.T) {

	encoder := &Encoder{}
	exts := map[gobay.Key]gobay.Extension{
		"test": encoder,
	}
	app, err := gobay.CreateApp("../testdata", "testing", exts)
	if app == nil || err != nil {
		t.Errorf("CreateApp error")
	}

	testSingleElem := map[string]int{
		"mmmmm": 0,
		"867nv": 1,
		"25t52": 2,
		"ghpzy": 3,
		"6vyv6": 4,
	}
	for k, v := range testSingleElem {
		value := uint64(v)
		kRes := encoder.Pk2str(value)
		vRes := encoder.Str2pk(k)
		if k != kRes {
			t.Errorf("Pk2str encode error")
		}
		if value != vRes {
			t.Errorf("Str2pk decode error")
		}
	}

	testMap := map[string]interface{}{
		"id":      20,
		"user_id": uint16(399),
		"age":     uint(18),
		"gender":  "male",
		"tags":    []uint64{1, 2, 3},
		"other_id": map[string]interface{}{
			"wordbok_id": 123,
			"hobby_id":   5,
		},
	}

	testMapRes := map[string]interface{}{
		"id":      "6xqx7",
		"user_id": "z2vr7",
		"age":     uint(18),
		"gender":  "male",
		"tags":    []interface{}{"867nv", "25t52", "ghpzy"},
		"other_id": map[string]interface{}{
			"wordbok_id": "9epgb",
			"hobby_id":   "pbq8b",
		},
	}

	// Encode map with excluded_fields
	encodedMap, err := encoder.EncodeMap(testMap, []string{"user_id"})
	if err != nil {
		t.Errorf("Encode map error")
	}
	for k := range encodedMap {
		if k != "user_id" && !reflect.DeepEqual(encodedMap[k], testMapRes[k]) {
			t.Errorf("Encode map with exclueded fields error")
		}
		if k == "user_id" && encodedMap[k] != testMap[k] {
			t.Errorf("Encode map error")
		}
	}

	// Encode map without excluded_fields
	encodedMapN, err := encoder.EncodeMap(testMap, []string{})
	if err != nil {
		t.Errorf("Encode map error")
	}
	for k := range encodedMapN {
		if !reflect.DeepEqual(encodedMapN[k], testMapRes[k]) {
			t.Errorf("Encode map with exclueded fields error")
		}
	}

	testMapDecodeRes := map[string]interface{}{
		"id":      uint64(20),
		"user_id": uint64(399),
		"age":     uint(18),
		"gender":  "male",
		"tags":    []uint64{1, 2, 3},
		"other_id": map[string]uint64{
			"wordbok_id": 123,
			"hobby_id":   5,
		},
	}

	// Decode map
	decodedMap, err := encoder.DecodeMap(testMapRes)
	if err != nil {
		t.Errorf("Decode map error")
	}
	for k, v := range decodedMap {
		switch v.(type) {
		case []interface{}:
			sliceRes := testMapDecodeRes[k].([]uint64)
			for kk, vv := range v.([]interface{}) {
				if vv.(uint64) != sliceRes[kk] {
					t.Errorf("Decode map error")
				}
			}
		case map[string]interface{}:
			mapRes := testMapDecodeRes[k].(map[string]uint64)
			for kk, vv := range v.(map[string]interface{}) {
				if vv.(uint64) != mapRes[kk] {
					t.Errorf("Decode map error")
				}
			}
		default:
			if !reflect.DeepEqual(v, testMapDecodeRes[k]) {
				t.Errorf("Decode map error")
			}
		}
	}

	testSlice := []uint64{100, 99, 98}
	testSliceRes := []interface{}{"6d7gw", "wnzat", "2fqd5"}

	// Encode slice
	encodedSlice, err := encoder.EncodeSlice(testSlice, []string{})
	if err != nil || !reflect.DeepEqual(encodedSlice, testSliceRes) {
		t.Errorf("Encode Slice error")
	}

	// Encode two-dimension slice
	testSliceM := [][]uint64{{100, 99, 98}}
	testSliceMRes := []interface{}{}
	testSliceMRes = append(testSliceMRes, (interface{})(testSliceRes))

	encodedSliceM, err := encoder.EncodeSlice(testSliceM, []string{})
	if err != nil || !reflect.DeepEqual(encodedSliceM, testSliceMRes) {
		t.Errorf("Encode Slice error")
	}

	// Decode slice
	decodedSlice, err := encoder.DecodeSlice(testSliceRes)
	if err != nil {
		t.Errorf("Decode slice error")
	}
	for k, v := range decodedSlice.([]interface{}) {
		if v.(uint64) != testSlice[k] {
			t.Errorf("Decode Slice error")
		}
	}

	// Decode two-dimension slice
	decodedSliceM, err := encoder.DecodeSlice(testSliceMRes)
	if err != nil {
		t.Errorf("Decode slice error")
	}
	for k, v := range decodedSliceM.([]interface{}) {
		for kk, vv := range v.([]interface{}) {
			if vv.(uint64) != testSliceM[k][kk] {
				t.Errorf("Decode Slice error")
			}
		}
	}

	// Test CanDecode method
	canD := encoder.CanDecode("hahaha")
	if !canD {
		t.Errorf("CanDecode analyze error")
	}

	canDD := encoder.CanDecode("23_haah")
	if canDD {
		t.Errorf("CanDecode analyze error")
	}
}
