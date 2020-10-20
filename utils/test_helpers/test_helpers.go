package test_helpers

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

// CheckTestCase - Check TestCase
func CheckTestCase(tt TestCase, res *httptest.ResponseRecorder, t *testing.T) {
	if tt.WantErr {
		if res.Code != tt.WantStatusCode {
			t.Errorf("Test_adminGetUserAppletAndBuyRecordHandler %v should %v, got %v", tt.Name, tt.WantStatusCode, res.Code)
		}
		return
	}

	if res.Code != 200 {
		t.Errorf("Test_adminGetUserAppletAndBuyRecordHandler %v should %v, got %v", tt.Name, tt.WantStatusCode, res.Code)
	}

	if !DeepEqualJSON(tt.WantJSON, string(res.Body.Bytes()), tt.IgnoredFieldKeys) {
		t.Errorf("Test_adminGetUserAppletAndBuyRecordHandler %v: want %v, got json:%v", tt.Name, tt.WantJSON, string(res.Body.Bytes()))
	}
}

// GetRequestFunc -
type GetRequestFunc func(interface{}) *http.Request

// CheckAPITestCases - Check API test Cases
func CheckAPITestCases(
	tests []TestCase,
	getRequest GetRequestFunc,
	t *testing.T,
	handler http.Handler,
) {
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			req := getRequest(tt.Req)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)

			CheckTestCase(tt, res, t)
		})
	}
}
