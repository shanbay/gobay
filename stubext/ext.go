package stubext

import (
	"context"
	"errors"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/shanbay/gobay"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"strconv"
	"time"
)

var (
	defaultConnTimeout  = 1 * time.Second
	defaultCallTimeout  = 500 * time.Millisecond
	defaultRetryBackoff = 100 * time.Millisecond
	defaultRetryTimes   = 0 // disable retry
	defaultRetryCodes   = []codes.Code{
		codes.Unavailable,
	}
)

type StubExt struct {
	NS  string
	app *gobay.Application

	Host         string
	Port         uint16
	ConnTimeout  time.Duration
	CallTimeout  time.Duration
	RetryBackoff time.Duration
	RetryTimes   uint
	Metadata     map[string]string

	retryCodes []codes.Code
	conn       *grpc.ClientConn
}

func (d *StubExt) Application() *gobay.Application { return d.app }

func (d *StubExt) Object() interface{} { return d }

func (d *StubExt) Close() error { return d.conn.Close() }

func (d *StubExt) Init(app *gobay.Application) error {
	// init from default
	d.ConnTimeout = defaultConnTimeout
	d.CallTimeout = defaultCallTimeout
	d.RetryBackoff = defaultRetryBackoff
	d.RetryTimes = uint(defaultRetryTimes)
	d.retryCodes = defaultRetryCodes

	// init from config
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
		config.SetEnvPrefix(d.NS)
	}
	config.AutomaticEnv()
	err := config.Unmarshal(d)
	if d.Port == 0 || d.Host == "" {
		log.Printf("host: %v, port: %v", d.Host, d.Port)
		return errors.New("lack of port or host")
	}
	return err
}

func (d *StubExt) SetRetryCodes(retryCodes []codes.Code) { d.retryCodes = retryCodes }

func (d *StubExt) getCallOpts() []grpc_retry.CallOption {
	// per call timeout
	var callOpts []grpc_retry.CallOption
	if d.CallTimeout > 0 {
		callOpts = append(callOpts, grpc_retry.WithPerRetryTimeout(d.CallTimeout))
	}
	// backoff
	if d.RetryBackoff > 0 {
		callOpts = append(callOpts, grpc_retry.WithBackoff(
			grpc_retry.BackoffExponential(d.RetryBackoff),
		))
	}
	// max retry
	if d.RetryTimes > 0 {
		callOpts = append(callOpts, grpc_retry.WithMax(d.RetryTimes))
	}
	return callOpts

}

func (d *StubExt) GetConn(opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if d.conn == nil {
		// opts: per call opts
		callOpts := d.getCallOpts()
		opts = append(
			opts,
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(callOpts...)),
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(callOpts...)),
		)
		// opts: connect opts
		ctxDefault := context.Background()
		if d.ConnTimeout > 0 {
			ctx, cancelFunc := context.WithTimeout(ctxDefault, d.ConnTimeout)
			ctxDefault = ctx
			defer cancelFunc()
		}
		address := net.JoinHostPort(d.Host, strconv.Itoa(int(d.Port)))
		// connect
		conn, err := grpc.DialContext(ctxDefault, address, opts...)
		if err != nil {
			return nil, err
		}
		d.conn = conn
	}
	return d.conn, nil
}

func (d *StubExt) GetClient(newFunc func(*grpc.ClientConn) interface{}, opts ...grpc.DialOption) (interface{}, error) {
	conn, err := d.GetConn(opts...)
	if err != nil {
		return nil, err
	}
	client := newFunc(conn)
	return client, nil
}

func (d *StubExt) GetCtx(ctx context.Context) context.Context {
	var md metadata.MD
	for k, v := range d.Metadata {
		md = metadata.Join(md, metadata.Pairs(k, v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
