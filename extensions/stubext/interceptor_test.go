package stubext

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
	"os"
	"testing"

	"github.com/shanbay/gobay/testdata/health_pb"
)

func (* healthServer) GetDownStreamName(ctx context.Context, req *health_pb.DownStreamNameReq) (*health_pb.DownStreamNameRes, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no header")
	}
	headers := md[DownStreamHeaderName]
	if len(headers) <= 0 {
		return nil, fmt.Errorf("no header")
	}
	return &health_pb.DownStreamNameRes{DownStreamName: headers[0]}, nil
}

func TestDownSteamNameHeaderInterceptor(t *testing.T) {
	setupServer()
	defer tearDownServer()
	setupStub("testing")

	err := os.Setenv("HOSTNAME", "cms-api-57c8c79744-xxsmb")
	assert.Nil(t, err)
	// init client
	client := stubext.Clients["health"]
	stubclient := client.(health_pb.HealthClient)

	// set ctx
	ctx := stubext.GetCtx(context.Background())
	res, err := stubclient.GetDownStreamName(ctx, &health_pb.DownStreamNameReq{Name: "xxx"})
	assert.Nil(t, err)
	assert.EqualValues(t, res.DownStreamName, "cms-api")

	err = os.Setenv("HOSTNAME", "")
	assert.Nil(t, err)
	ctx = stubext.GetCtx(context.Background())
	res, err = stubclient.GetDownStreamName(ctx, &health_pb.DownStreamNameReq{Name: "xxx"})
	assert.NotNil(t, err)
}
