package stubext

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/shanbay/gobay"
	protos_go "github.com/shanbay/gobay/testdata/health_pb"
	mock_protos_go "github.com/shanbay/gobay/testdata/health_pb_mock"
)

var (
	server  *grpc.Server
	stubext StubExt
)

// mock Check function return Error body no healthy upstream
func mockCheckRPC(t *testing.T) (*mock_protos_go.MockHealthClient, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockedClient := mock_protos_go.NewMockHealthClient(ctrl)
	stubclient := mockedClient
	mockedClient.EXPECT().Check(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		nil, errors.New("no healthy upstream"),
	).AnyTimes()
	return stubclient, ctrl
}

func tearDownMmockCheckRPC(ctrl *gomock.Controller) {
	ctrl.Finish()
}

func setupStub(env string) {
	stubext = StubExt{
		NS:          "stub_health_",
		DialOptions: []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()},
		NewClientFuncs: map[string]NewClientFunc{
			"health": func(conn *grpc.ClientConn) interface{} {
				return protos_go.NewHealthClient(conn)
			},
		},
	}

	app, err := gobay.CreateApp(
		"../../testdata",
		env,
		map[gobay.Key]gobay.Extension{
			"stubext": &stubext,
		},
	)
	if err != nil {
		panic(err)
	}

	if err := app.Init(); err != nil {
		log.Println(err)
	}
	if stubext.ConnTimeout != 1*time.Second {
		panic("ConnTimeout should be 1s, got " + strconv.Itoa(int(stubext.ConnTimeout)))
	}
	if stubext.CallTimeout != 300*time.Millisecond {
		panic(stubext.CallTimeout)
	}
}

type healthServer struct{}

func (h *healthServer) Check(ctx context.Context, req *protos_go.HealthCheckRequest) (*protos_go.HealthCheckResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic(md)
	}
	log.Printf("md got on server: %v", md)
	if md[":authority"][0] != "health-rpc.xyz" {
		panic("authority should be \"health-rpc.xyz\"")
	}
	if md["svc_auth_token"][0] != "abcdefg" {
		panic("authority should be \"abcdefg\"")
	}
	return &protos_go.HealthCheckResponse{Status: protos_go.HealthCheckResponse_SERVING}, nil
}

func setupServer() {
	server = grpc.NewServer()
	protos_go.RegisterHealthServer(server, &healthServer{})
	lis, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "5555"))
	if err != nil {
		panic(err)
	}
	go func() {
		err = server.Serve(lis)
		if err != nil {
			panic(err)
		}
	}()
}

func tearDownServer() {
	server.GracefulStop()
}

func TestStubExt(t *testing.T) {
	// run 3 times
	for i := 0; i < 3; i++ {
		// setup
		setupServer()
		setupStub("testing")

		// init client
		client := stubext.Clients["health"]
		stubclient := client.(protos_go.HealthClient)

		// set ctx
		ctx := stubext.GetCtx(context.Background())
		md, ok := metadata.FromOutgoingContext(ctx)
		assert.True(t, ok)
		t.Logf("md: %v", md)

		// call
		res, err := stubclient.Check(ctx, &protos_go.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, res.Status, protos_go.HealthCheckResponse_SERVING)
		// tearDown
		tearDownServer()
	}
}

func TestStubExtServerStop(t *testing.T) {
	// setup
	setupServer()
	setupStub("testing")

	// init client
	client := stubext.Clients["health"]
	stubclient := client.(protos_go.HealthClient)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	_, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	assert.NotNil(t, err)
	diff := time.Since(start)
	assert.True(t, diff < 200*time.Millisecond)
	t.Logf("shorter duration: %v", diff)
}

func TestStubExtServerStopRetryLonger(t *testing.T) {
	// setup
	setupServer()
	setupStub("grpclong")

	// init client
	client := stubext.Clients["health"]
	stubclient := client.(protos_go.HealthClient)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	_, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	assert.NotNil(t, err)

	diff := time.Since(start)
	assert.True(t, diff > 200*time.Millisecond)
	t.Logf("longer duration: %v", diff)
}

func TestStubExtServerStopRetryLongerUh(t *testing.T) {
	// setup
	setupServer()
	setupStub("grpclong")
	// stop server
	server.GracefulStop()

	start := time.Now()
	stubclient, ctrl := mockCheckRPC(t)
	defer tearDownMmockCheckRPC(ctrl)
	_, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	assert.NotNil(t, err)
	diff := time.Since(start)
	assert.True(t, diff < 1*time.Millisecond)
	t.Logf("longer duration: %v", diff)
}

func TestStubExtServerStopNoRetry(t *testing.T) {
	// setup
	setupServer()
	setupStub("grpcnoretry")

	// init client
	client := stubext.Clients["health"]
	stubclient := client.(protos_go.HealthClient)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	_, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	assert.NotNil(t, err)
	diff := time.Since(start)
	assert.True(t, diff < 1*time.Millisecond)
	t.Logf("no retry duration: %v", diff)
}

func TestStubExtServerStopMocked(t *testing.T) {
	// setup without server
	setupStub("grpcmocked")

	// init client
	client := stubext.Clients["health"]
	assert.Nil(t, client)
}
