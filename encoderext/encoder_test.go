package encoderext

import (
	"testing"
	"reflect"
	"github.com/shanbay/gobay"
	"github.com/stretchr/testify/assert"
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
	for k, v:= range(testSingleElem){
		value := uint64(v)
		kRes := encoder.Pk2str(value)
		vRes := encoder.Str2pk(k)
		if k != kRes{
			t.Errorf("Pk2str encode error")
		}
		if value != vRes{
			t.Errorf("Str2pk decode error")
		}
	}

	testMap := map[string]interface{}{
		"id": uint64(20),
		"user_id": uint64(399),
		"age": uint64(18),
		"gender": "male",
		"tags": []uint64{1, 2, 3},
	}

	testMapRes := map[string]interface{}{
		"id": "6xqx7",
		"user_id": "z2vr7",
		"age": uint64(18),
		"gender": "male",
		"tags": []string{"867nv", "25t52", "ghpzy"},
	}

	encodedMap := encoder.EncodeMap(testMap, []string{"user_id"})
	for v := range(encodedMap){
		if v != "user_id" && !reflect.DeepEqual(encodedMap[v], testMapRes[v]){
			t.Errorf("Encode map with exclueded fields error")
		}
		if v == "user_id" && encodedMap[v] != testMap[v]{
			t.Errorf("Encode map error")
		}
	}

	encodedMapN := encoder.EncodeMap(testMap, []string{})
	for v:= range(encodedMapN){
		if !reflect.DeepEqual(encodedMapN[v], testMapRes[v]){
			t.Errorf("Encode map error")
		}
	}

	decodedMap := encoder.DecodeMap(testMapRes)
	for v:= range(decodedMap){
		if !reflect.DeepEqual(decodedMap[v], testMap[v]){
			t.Errorf("Decode map error")
		}
	}

	testSlice := []uint64{100, 99, 98}
	testSliceRes := []string{"6d7gw", "wnzat", "2fqd5"}
	
		encodedSlice := encoder.EncodeSlice(testSlice)
		if !reflect.DeepEqual(encodedSlice, testSliceRes){
			t.Errorf("Encode Slice error")
		}
	
		decodedSlice := encoder.DecodeSlice(testSliceRes)
		if !reflect.DeepEqual(decodedSlice, testSlice){
			t.Errorf("Decode Slice error")
		}
}