package stubext

import (
	"context"
	"github.com/shanbay/gobay"
	"github.com/shanbay/gobay/testdata/health_pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"strconv"
	"testing"
	"time"
)

var (
	server  *grpc.Server
	stubext StubExt
)

func setupStub() {
	stubext = StubExt{NS: "stub_health"}

	app, err := gobay.CreateApp(
		"../testdata",
		"testing",
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
	if stubext.RetryBackoff != 50*time.Millisecond {
		panic("ConnTimeout should be 50ms, got " + strconv.Itoa(int(stubext.RetryBackoff)))
	}
	if stubext.RetryTimes != 3 {
		panic(stubext.RetryTimes)
	}
}

type healthServer struct{}

func (h *healthServer) Check(ctx context.Context, req *protos_go.HealthCheckRequest) (*protos_go.HealthCheckResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		panic(md)
	}
	log.Printf("md got on server: %v", md)
	if md["grpc.default_authority"][0] != "health-rpc.xyz" {
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
		setupStub()

		// init client
		conn := stubext.GetConn(grpc.WithInsecure(), grpc.WithBlock())
		stubclient := protos_go.NewHealthClient(conn)

		// set ctx
		ctx := stubext.GetCtx(context.Background())
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			t.Errorf("ctx not ok: %v", ctx)
		}
		t.Logf("md: %v", md)

		// call
		res, err := stubclient.Check(ctx, &protos_go.HealthCheckRequest{})
		if err != nil {
			t.Errorf("Check failed: %v", err)
		}
		if res.Status != protos_go.HealthCheckResponse_SERVING {
			t.Errorf("Status should be SERVING")
		}

		// tearDown
		tearDownServer()
	}
}

func TestStubExtServerStop(t *testing.T) {
	// setup
	setupServer()
	setupStub()

	// init client
	conn := stubext.GetConn(grpc.WithInsecure(), grpc.WithBlock())
	stubclient := protos_go.NewHealthClient(conn)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	res, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	if err == nil {
		t.Errorf("Check not failed: %v", res)
	}
	diff := time.Since(start)
	if diff > 200*time.Millisecond {
		t.Errorf("time shoud less than 200ms")
	}
	t.Logf("shorter duration: %v", diff)
}

func TestStubExtServerStopRetryLonger(t *testing.T) {
	// setup
	setupServer()
	setupStub()
	stubext.RetryBackoff = 300 * time.Millisecond

	// init client
	conn := stubext.GetConn(grpc.WithInsecure(), grpc.WithBlock())
	stubclient := protos_go.NewHealthClient(conn)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	res, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	if err == nil {
		t.Errorf("Check not failed: %v", res)
	}
	diff := time.Since(start)
	if diff < 900*time.Millisecond {
		t.Errorf("time shoud longer than 900ms")
	}
	t.Logf("longer duration: %v", diff)
}

func TestStubExtServerStopNoRetry(t *testing.T) {
	// setup
	setupServer()
	setupStub()
	stubext.RetryTimes = 0

	// init client
	conn := stubext.GetConn(grpc.WithInsecure(), grpc.WithBlock())
	stubclient := protos_go.NewHealthClient(conn)

	// stop server
	server.GracefulStop()

	// call
	start := time.Now()
	res, err := stubclient.Check(
		stubext.GetCtx(context.Background()),
		&protos_go.HealthCheckRequest{},
	)
	if err == nil {
		t.Errorf("Check not failed: %v", res)
	}
	diff := time.Since(start)
	if diff > 1*time.Millisecond {
		t.Errorf("time shoud less than 1ms, got %v", diff)
	}
	t.Logf("no retry duration: %v", diff)
}
