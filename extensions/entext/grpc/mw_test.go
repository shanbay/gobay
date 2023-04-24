package ent_mw

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/go-sql-driver/mysql"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/extensions/entext"
	"github.com/shanbay/gobay/testdata/ent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	goodPing  = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
	panicPing = &pb_testproto.PingRequest{Value: "error", SleepTimeMs: 9999}
	entclient *ent.Client
	ext       *entext.EntExt
)

func setup() func() error {
	exts := map[gobay.Key]gobay.Extension{
		"entext": &entext.EntExt{
			NS: "db_",
			NewClient: func(drvopt interface{}) entext.Client {
				return ent.NewClient(drvopt.(ent.Option))
			},
			Driver: func(drv dialect.Driver) interface{} {
				return ent.Driver(drv)
			},
			IsNotFound:          ent.IsNotFound,
			IsConstraintFailure: ent.IsConstraintError,
			IsNotSingular:       ent.IsNotSingular,
		},
	}
	app, err := gobay.CreateApp("../../../testdata", "testing", exts)
	ext = app.Get("entext").(*entext.EntExt)
	if err != nil {
		panic(err)
	}
	entclient = app.Get("entext").Object().(*ent.Client)
	return app.Close
}

type recoveryAssertService struct {
	pb_testproto.TestServiceServer
}

func (s *recoveryAssertService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	if ping.Value == "error" {
		return nil, &ent.NotFoundError{}
	}
	return s.TestServiceServer.Ping(ctx, ping)
}

func TestEntMw(t *testing.T) {
	defer setup()()
}

type GRPCSuite struct {
	*grpc_testing.InterceptorTestSuite
}

func TestGRPCHandler(t *testing.T) {
	// init sentry
	s := &GRPCSuite{
		&grpc_testing.InterceptorTestSuite{
			TestService: &recoveryAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}},
			ServerOpts: []grpc.ServerOption{
				grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
					GetEntUnaryMw(ext),
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
	assert.Equal(s.T(), codes.NotFound, status.Code(err), "must error with NotFound")
	assert.Equal(s.T(), "rpc error: code = NotFound desc = Not Found", err.Error(), "must error with message")
}
