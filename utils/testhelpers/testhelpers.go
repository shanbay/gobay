package testhelpers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shanbay/gobay"
	"github.com/stretchr/objx"
)

// JSONMustMarshal - direct json.Marshal with ignore errors
func JSONMustMarshal(val interface{}) string {
	strVal, _ := json.Marshal(val)
	return string(strVal)
}

// DeepEqualJSON - check if two json string are equal, with ignoredKeys
func DeepEqualJSON(xJSON, yJSON string, ignoredKeys []string) bool {
	if xJSON == "" && yJSON == "" {
		return true
	}
	if xJSON == "" || yJSON == "" {
		return false
	}

	xVal, err := objx.FromJSON(xJSON)
	if err != nil {
		log.Fatalf("objx FromJSON x failed: %v, x=%v", err.Error(), xJSON)
	}

	yVal, err := objx.FromJSON(yJSON)
	if err != nil {
		log.Fatalf("objx FromJSON y failed: %v, y=%v", err.Error(), yJSON)
	}

	for _, key := range ignoredKeys {
		xVal = xVal.Set(key, nil)
		yVal = yVal.Set(key, nil)
	}

	xJSONStr, _ := xVal.JSON()
	yJSONStr, _ := yVal.JSON()

	return xJSONStr == yJSONStr
}

// TestCase - test case
type TestCase struct {
	App              *gobay.Application
	Ctx              context.Context
	Name             string
	Req              interface{}
	IgnoredFieldKeys []string
	WantJSON         string
	WantErr          bool
	WantStatusCode   int
}

// MakeTestCase - auto-fix test case if
func MakeTestCase(query *TestCase, wantRes interface{}) TestCase {
	if wantRes != nil {
		query.WantJSON = JSONMustMarshal(wantRes)
	}

	return *query
}

// CheckAPITestCaseResult - Check API TestCase's result
func CheckAPITestCaseResult(tt TestCase, res *httptest.ResponseRecorder, t *testing.T) {
	if tt.WantErr {
		if res.Code != tt.WantStatusCode {
			t.Errorf("Test case %v should %v, got %v", tt.Name, tt.WantStatusCode, res.Code)
		}
		return
	}

	if res.Code != 200 {
		t.Errorf("Test case %v should %v, got %v", tt.Name, tt.WantStatusCode, res.Code)
	}

	if !DeepEqualJSON(tt.WantJSON, res.Body.String(), tt.IgnoredFieldKeys) {
		t.Errorf("Test case %v: want %v, got json:%v", tt.Name, tt.WantJSON, res.Body.String())
	}
}

// GetAPIRequestFunc -
type GetAPIRequestFunc func(interface{}) *http.Request

// CheckAPITestCases - Check API test Cases
func CheckAPITestCases(
	tests []TestCase,
	getRequest GetAPIRequestFunc,
	t *testing.T,
	handler http.Handler,
) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			req := getRequest(tt.Req)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			CheckAPITestCaseResult(tt, res, t)
		})
	}
}

// GetGRPCResultFunc -
type GetGRPCResultFunc func(TestCase, *testing.T) (interface{}, error)

// CheckGRPCTestCaseResult - check single GRPC test result
func CheckGRPCTestCaseResult(
	testCase TestCase,
	got interface{},
	err error,
	t *testing.T,
) {
	if testCase.WantErr {
		if err == nil {
			t.Errorf("Test case %v error = %v, wantErr %v", testCase.Name, err, testCase.WantErr)
			return
		}
		return
	}
	gotJSON := JSONMustMarshal(got)
	if !DeepEqualJSON(testCase.WantJSON, gotJSON, testCase.IgnoredFieldKeys) {
		t.Errorf("Test case %v: want %v, got json:%v", testCase.Name, testCase.WantJSON, gotJSON)
	}
}

// CheckGRPCTestCases - check multiple GRPC test cases
func CheckGRPCTestCases(
	testCases []TestCase,
	getGrpcResult GetGRPCResultFunc,
	t *testing.T,
) {
	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := getGrpcResult(tt, t)

			CheckGRPCTestCaseResult(tt, got, err, t)
		})
	}
}
