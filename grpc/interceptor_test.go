package gobay_grpc

import (
	"context"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/shanbay/gobay"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
)

var (
	goodPing = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
)

type interceptorServer struct {
	pb_testproto.TestServiceServer
}

func (s *interceptorServer) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	if GetCurrentAPP(ctx) == nil {
		panic("need gobay_app in ctx")
	}
	return s.TestServiceServer.Ping(ctx, ping)
}

func (s *interceptorServer) PingStream(srv pb_testproto.TestService_PingStreamServer) error {
	if GetCurrentAPP(srv.Context()) == nil {
		panic("need gobay_app in ctx")
	}
	return s.TestServiceServer.PingStream(srv)
}

type CurrentAPPTestSuite struct {
	*grpc_testing.InterceptorTestSuite
}

func TestCurrentAPPTestSuite(t *testing.T) {
	app, err := gobay.CreateApp("../testdata", "testing", nil)
	if err != nil {
		panic(err)
	}

	s := &CurrentAPPTestSuite{
		&grpc_testing.InterceptorTestSuite{
			TestService: &interceptorServer{TestServiceServer: &grpc_testing.TestPingService{T: t}},
			ServerOpts: []grpc.ServerOption{
				grpc.UnaryInterceptor(UnaryCurrentAPPServerInterceptor(app)),
				grpc.StreamInterceptor(StreamCurrentAPPServerInterceptor(app)),
			},
		},
	}
	suite.Run(t, s)
}

func (s *CurrentAPPTestSuite) TestUnary_GetCurrentAPP() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "no error must occur")
}

func (s *CurrentAPPTestSuite) TestStream_GetCurrentAPP() {
	_, err := s.Client.PingStream(s.SimpleCtx())
	require.NoError(s.T(), err, "no error must occur")
}
