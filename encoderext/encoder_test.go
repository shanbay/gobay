package encoderext

import (
	"github.com/shanbay/gobay"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestEncoder(t *testing.T) {
	assert := assert.New(t)

	encoder := &Encoder{}
	exts := map[gobay.Key]gobay.Extension{
		"test": encoder,
	}
	app, err := gobay.CreateApp("../testdata", "testing", exts)
	assert.NotNil(app)
	assert.Nil(err)

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
	encodedMap := encoder.EncodeMap(testMap, []string{"user_id"})
	encodedMapV, ok := encodedMap.(map[string]interface{})
	if !ok {
		t.Errorf("Wrong type")
	}
	for k := range encodedMapV {
		if k != "user_id" && !reflect.DeepEqual(encodedMapV[k], testMapRes[k]) {
			t.Errorf("Encode map with exclueded fields error")
		}
		if k == "user_id" && encodedMapV[k] != testMap[k] {
			t.Errorf("Encode map error")
		}
	}

	// Encode map without excluded_fields
	encodedMapN := encoder.EncodeMap(testMap, []string{})
	encodedMapNV, ok := encodedMapN.(map[string]interface{})
	if !ok {
		t.Errorf("Wrong type")
	}
	for k := range encodedMapNV {
		if !reflect.DeepEqual(encodedMapNV[k], testMapRes[k]) {
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
	decodedMap := encoder.DecodeMap(testMapRes)
	decodedMapV, ok := decodedMap.(map[string]interface{})
	if !ok {
		t.Errorf("Wrong type")
	}
	for k, v := range decodedMapV {
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
				// fmt.Println(vv, )
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
	encodedSlice := encoder.EncodeSlice(testSlice, []string{})
	if !reflect.DeepEqual(encodedSlice, testSliceRes) {
		t.Errorf("Encode Slice error")
	}

	// Encode two-dimension slice
	testSliceM := [][]uint64{{100, 99, 98}}
	testSliceMRes := []interface{}{}
	testSliceMRes = append(testSliceMRes, (interface{})(testSliceRes))

	encodedSliceM := encoder.EncodeSlice(testSliceM, []string{})
	if !reflect.DeepEqual(encodedSliceM, testSliceMRes) {
		t.Errorf("Encode Slice error")
	}

	// Decode slice
	decodedSlice := encoder.DecodeSlice(testSliceRes).([]interface{})
	for k, v := range decodedSlice {
		if v.(uint64) != testSlice[k] {
			t.Errorf("Decode Slice error")
		}
	}

	// Decode two-dimension slice
	decodedSliceM := encoder.DecodeSlice(testSliceMRes).([]interface{})
	for k, v := range decodedSliceM {
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
