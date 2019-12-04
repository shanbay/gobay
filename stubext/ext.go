package stubext

import (
	"context"
	"fmt"
	"github.com/shanbay/gobay"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type StubExt struct {
	NS  string
	app *gobay.Application

	host           string
	port           uint16
	authority      string
	svc_auth_token string
	conn           *grpc.ClientConn
}

func (d *StubExt) Application() *gobay.Application { return d.app }

func (d *StubExt) Object() interface{} { return d }

func (d *StubExt) Close() error { return d.conn.Close() }

func (d *StubExt) Init(app *gobay.Application) error {
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
		config.SetEnvPrefix(d.NS)
	}
	config.AutomaticEnv()
	err := config.Unmarshal(d)
	return err
}

func (d *StubExt) GetConn(opts ...grpc.DialOption) *grpc.ClientConn {
	// init conn
	if d.conn == nil {
		address := fmt.Sprintf("%v:%v", d.host, d.port)
		conn, _ := grpc.Dial(address, opts...)
		d.conn = conn
	}
	return d.conn
}

func (d *StubExt) GetCtx() context.Context {
	md := metadata.Pairs("grpc.default_authority", d.authority)
	if d.svc_auth_token != "" {
		md = metadata.Join(md, metadata.Pairs("svc_auth_token", d.svc_auth_token))
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}
