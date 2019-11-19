package sentry_mw

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

var handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	panic("an error should be caught by sentry_mw")
})

func TestOpenAPIWithoutSentry(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("there is a panic")
		} else {
			panic("there should be a panic")
		}
	}()

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

func TestOpenAPIWithSentry(t *testing.T) {
	// init sentry
	config := viper.New()
	config.Set("dsn", "http://username:password@abc.def/80")
	config.Set("environment", "staging")
	config.Set("repanic", false)

	middleware, err := GetOpenAPIMiddleware(config)
	if err != nil {
		log.Fatal("Fail: ", err)
	}

	// handle
	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	new_handler := middleware(handler)
	new_handler.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	// result
	if resp.StatusCode != 200 {
		log.Fatal("Fail: recover error")
	}
	fmt.Println("code: ", resp.StatusCode)
	fmt.Println("content-type: ", resp.Header.Get("Content-Type"))
	fmt.Println("body: ", string(body))

	// init sentry
	config.Set("repanic", true)

	middleware, err = GetOpenAPIMiddleware(config)
	if err != nil {
		log.Fatal("Fail: ", err)
	}

	// handle
	defer func() {
		if err := recover(); err != "an error should be caught by sentry_mw" {
			log.Fatal("Fail: ", err)
		}
	}()
	req = httptest.NewRequest("GET", "/hello", nil)
	w = httptest.NewRecorder()
	new_handler = middleware(handler)
	new_handler.ServeHTTP(w, req)
}

// test gRPC
var (
	goodPing  = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
	panicPing = &pb_testproto.PingRequest{Value: "panic", SleepTimeMs: 9999}
)

type recoveryAssertService struct {
	pb_testproto.TestServiceServer
}

func (s *recoveryAssertService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	if ping.Value == "panic" {
		panic("very bad thing happened")
	}
	return s.TestServiceServer.Ping(ctx, ping)
}

type GRPCSuite struct {
	*grpc_testing.InterceptorTestSuite
}

func TestGRPCHandler(t *testing.T) {
	// init sentry
	config := viper.New()
	config.Set("dsn", "http://username:password@abc.def/80")
	config.Set("environment", "staging")
	sentry_handler, _ := GetGRPCHandler(config)
	s := &GRPCSuite{
		&grpc_testing.InterceptorTestSuite{
			TestService: &recoveryAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}},
			ServerOpts: []grpc.ServerOption{
				grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
					grpc_recovery.UnaryServerInterceptor(sentry_handler),
				)),
			},
		},
	}
	suite.Run(t, s)
}

func (s *GRPCSuite) TestUnary_SuccessfulRequest() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "no error must occur")
}

func (s *GRPCSuite) TestUnary_PanickingRequest() {
	_, err := s.Client.Ping(s.SimpleCtx(), panicPing)
	require.Error(s.T(), err, "there must be an error")
	assert.Equal(s.T(), codes.Internal, grpc.Code(err), "must error with internal")
	assert.Equal(s.T(), "msg: very bad thing happened", grpc.ErrorDesc(err), "must error with message")
}
