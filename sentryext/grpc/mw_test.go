package sentrygrpcmw

import (
	"context"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/sentryext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"testing"
)

var (
	goodPing         = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
	panicPing        = &pb_testproto.PingRequest{Value: "panic", SleepTimeMs: 9999}
	sentry_extension sentryext.SentryExt
)

type recoveryAssertService struct {
	pb_testproto.TestServiceServer
}

func init() {
	sentry_extension = sentryext.SentryExt{NS: "sentry"}
	app, err := gobay.CreateApp(
		"../../testdata",
		"testing",
		map[gobay.Key]gobay.Extension{
			"sentry": &sentry_extension,
		},
	)
	if err != nil {
		log.Panic(err)
	}
	if err := app.Init(); err != nil {
		log.Panic(err)
	}
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
	option := GetOption(&sentry_extension)
	s := &GRPCSuite{
		&grpc_testing.InterceptorTestSuite{
			TestService: &recoveryAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}},
			ServerOpts: []grpc.ServerOption{
				grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
					grpc_recovery.UnaryServerInterceptor(option),
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
	assert.Equal(s.T(), codes.Internal, status.Code(err), "must error with internal")
	assert.Equal(s.T(), "rpc error: code = Internal desc = msg: very bad thing happened", err.Error(), "must error with message")
}
