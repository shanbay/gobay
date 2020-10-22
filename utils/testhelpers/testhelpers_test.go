package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/shanbay/gobay"

	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
)

type DeepStruct struct {
	MapData    *DeepStruct
	ArrayData  []*DeepStruct
	IntData    int
	StringData string
	BoolData   bool
}

func Test_JSONMustMarshal(t *testing.T) {
	a := DeepStruct{
		IntData:    123,
		StringData: "456",
		BoolData:   true,
		MapData: &DeepStruct{
			IntData: 456,
		},
		ArrayData: []*DeepStruct{
			{
				IntData: 789,
			},
			{
				StringData: "123",
			},
		},
	}

	wantJSONBytes, err := json.Marshal(a)
	if err != nil {
		t.Errorf("json marshal error %v", err)
	}
	wantJSON := string(wantJSONBytes)

	jsonStr := JSONMustMarshal(a)

	if jsonStr != wantJSON {
		t.Errorf("json not match, want: %v, got: %v", wantJSON, jsonStr)
	}
	t.Logf("got json: %v", jsonStr)
}

func Test_deepEqual(t *testing.T) {
	a := DeepStruct{
		IntData:    123,
		StringData: "456",
		BoolData:   true,
		MapData: &DeepStruct{
			IntData: 456,
		},
		ArrayData: []*DeepStruct{
			{
				IntData:    789,
				StringData: "not match 123",
			},
			{
				StringData: "123",
			},
		},
	}
	b := DeepStruct{
		IntData:    123,
		StringData: "456",
		BoolData:   true,
		MapData: &DeepStruct{
			IntData: 456,
		},
		ArrayData: []*DeepStruct{
			{
				IntData:    789,
				StringData: "not match 456",
			},
			{
				StringData: "123",
			},
		},
	}

	aJSON := JSONMustMarshal(a)
	bJSON := JSONMustMarshal(b)

	if DeepEqualJSON(aJSON, bJSON, []string{}) {
		t.Errorf("DeepEqualJSON should fail, %v, %v", aJSON, bJSON)
	}

	if !DeepEqualJSON(aJSON, bJSON, []string{"ArrayData.0.StringData"}) {
		t.Errorf("DeepEqualJSON failed, %v, %v", aJSON, bJSON)
	}
}

func Test_MakeTestCase(t *testing.T) {
	a := DeepStruct{
		IntData:    123,
		StringData: "456",
		BoolData:   true,
		MapData: &DeepStruct{
			IntData: 456,
		},
		ArrayData: []*DeepStruct{
			{
				IntData: 789,
			},
			{
				StringData: "123",
			},
		},
	}

	testCase := TestCase{}

	if testCase.WantJSON != "" {
		t.Errorf("want json should be empty string, got %v", testCase.WantJSON)
	}

	MakeTestCase(&testCase, a)
	aJSON := JSONMustMarshal(a)
	if testCase.WantJSON != aJSON {
		t.Errorf("want json should be empty string, got %v", testCase.WantJSON)
	}
}

func setup() *gobay.Application {
	// init app
	bapp, err := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{},
	)
	if err != nil {
		panic(err)
	}
	if err := bapp.Init(); err != nil {
		log.Panic(err)
	}
	return bapp
}

func tearDown() {}

func getHandler(app *gobay.Application) http.Handler {
	var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("path %v", r.URL.Path)
		if r.URL.Path == "/test/int/12345/string/67890" {
			h := w.Header()
			h.Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(200)
			w.Write([]byte("{\"ArrayData\":null,\"BoolData\":false,\"IntData\":54321,\"MapData\":null,\"StringData\":\"asdf\"}"))
			return
		}
		w.WriteHeader(400)
	})
	return handler
}

func Test_CheckAPITestCaseResult(t *testing.T) {
	bapp := setup()
	defer tearDown()

	ctx := context.Background()
	handler := getHandler(bapp)

	testCases := []TestCase{
		MakeTestCase(
			&TestCase{
				App:  bapp,
				Ctx:  ctx,
				Name: "TestCase1",
				Req: &DeepStruct{
					IntData:    12345,
					StringData: "67890",
				},
				IgnoredFieldKeys: []string{"StringData"},
			},
			DeepStruct{
				IntData:    54321,
				StringData: "DONTMATTER",
			},
		),
		{
			App:  bapp,
			Ctx:  ctx,
			Name: "TestCase2",
			Req: &DeepStruct{
				IntData:    12345,
				StringData: "11111",
			},
			WantErr:        true,
			WantStatusCode: 400,
		},
	}

	getRequestFunc := func(req interface{}) *http.Request {
		reqBody := req.(*DeepStruct)
		result := httptest.NewRequest("GET", "/test/int/"+strconv.Itoa(reqBody.IntData)+"/string/"+reqBody.StringData, nil)
		return result
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			req := getRequestFunc(tt.Req)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			CheckAPITestCaseResult(tt, res, t)
		})
	}
}

func Test_CheckAPITestCases(t *testing.T) {
	bapp := setup()
	defer tearDown()

	ctx := context.Background()
	handler := getHandler(bapp)

	testCases := []TestCase{
		MakeTestCase(
			&TestCase{
				App:  bapp,
				Ctx:  ctx,
				Name: "TestCase1",
				Req: &DeepStruct{
					IntData:    12345,
					StringData: "67890",
				},
				IgnoredFieldKeys: []string{"StringData"},
			},
			DeepStruct{
				IntData:    54321,
				StringData: "DONTMATTER",
			},
		),
		{
			App:  bapp,
			Ctx:  ctx,
			Name: "TestCase2",
			Req: &DeepStruct{
				IntData:    12345,
				StringData: "11111",
			},
			WantErr:        true,
			WantStatusCode: 400,
		},
	}

	getRequestFunc := func(req interface{}) *http.Request {
		reqBody := req.(*DeepStruct)
		result := httptest.NewRequest("GET", "/test/int/"+strconv.Itoa(reqBody.IntData)+"/string/"+reqBody.StringData, nil)
		return result
	}

	CheckAPITestCases(testCases, getRequestFunc, t, handler)
}

type recoveryAssertService struct {
	pb_testproto.TestServiceServer
}

func (s *recoveryAssertService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	if ping.Value == "error" {
		return nil, fmt.Errorf("error")
	}
	return s.TestServiceServer.Ping(ctx, ping)
}

func Test_CheckGRPCTestCaseResult(t *testing.T) {
	bapp := setup()
	defer tearDown()

	ctx := context.Background()

	testCases := []TestCase{
		MakeTestCase(
			&TestCase{
				App:              bapp,
				Ctx:              ctx,
				Name:             "TestCase1",
				Req:              &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999},
				IgnoredFieldKeys: []string{"counter"},
			},
			pb_testproto.PingResponse{
				Value: "something",
			},
		),
		{
			App:     bapp,
			Ctx:     ctx,
			Name:    "TestCase2",
			Req:     &pb_testproto.PingRequest{Value: "error", SleepTimeMs: 9999},
			WantErr: true,
		},
	}

	getRequestFunc := func(test TestCase, t *testing.T) (interface{}, error) {
		s := &recoveryAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}}
		typedReq := test.Req.(*pb_testproto.PingRequest)
		got, err := s.Ping(ctx, typedReq)
		return got, err
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			res, err := getRequestFunc(tt, t)

			CheckGRPCTestCaseResult(tt, res, err, t)
		})
	}
}

func Test_CheckGRPCTestCases(t *testing.T) {
	bapp := setup()
	defer tearDown()

	ctx := context.Background()

	testCases := []TestCase{
		MakeTestCase(
			&TestCase{
				App:              bapp,
				Ctx:              ctx,
				Name:             "TestCase1",
				Req:              &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999},
				IgnoredFieldKeys: []string{"counter"},
			},
			pb_testproto.PingResponse{
				Value: "something",
			},
		),
		{
			App:     bapp,
			Ctx:     ctx,
			Name:    "TestCase2",
			Req:     &pb_testproto.PingRequest{Value: "error", SleepTimeMs: 9999},
			WantErr: true,
		},
	}

	getGRPCResultFunc := func(test TestCase, t *testing.T) (interface{}, error) {
		s := &recoveryAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}}
		typedReq := test.Req.(*pb_testproto.PingRequest)
		got, err := s.Ping(ctx, typedReq)
		return got, err
	}

	CheckGRPCTestCases(testCases, getGRPCResultFunc, t)
}
