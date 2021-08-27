package stubext

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/shanbay/gobay"
	"go.elastic.co/apm/module/apmgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type NewClientFunc (func(*grpc.ClientConn) interface{})

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
	NS             string
	DailOptions    []grpc.DialOption
	NewClientFuncs map[string]NewClientFunc
	RetryCodes     []codes.Code
	app            *gobay.Application

	Host         string
	Port         uint16
	ConnTimeout  time.Duration
	CallTimeout  time.Duration
	RetryBackoff time.Duration
	RetryTimes   uint
	Authority    string
	Metadata     map[string]string
	Mocked       bool

	Clients map[string]interface{}
	conn    *grpc.ClientConn

	enableApm bool
}

func (d *StubExt) Application() *gobay.Application { return d.app }

func (d *StubExt) Object() interface{} { return d }

func (d *StubExt) Close() error {
	if d.Mocked {
		return nil
	}
	return d.conn.Close()
}

func (d *StubExt) Init(app *gobay.Application) error {
	if d.NS == "" {
		return errors.New("lack of NS")
	}
	// init from default
	d.ConnTimeout = defaultConnTimeout
	d.CallTimeout = defaultCallTimeout
	d.RetryBackoff = defaultRetryBackoff
	d.RetryTimes = uint(defaultRetryTimes)
	d.Clients = make(map[string]interface{})
	if d.RetryCodes == nil {
		d.RetryCodes = defaultRetryCodes
	}

	// init from config
	d.app = app
	config := app.Config()
	d.enableApm = config.GetBool("elastic_apm_enable")
	config = gobay.GetConfigByPrefix(config, d.NS, true)
	if err := config.Unmarshal(d); err != nil {
		return err
	}
	if d.Port == 0 || d.Host == "" {
		log.Printf("host: %v, port: %v", d.Host, d.Port)
		return errors.New("lack of port or host")
	}

	// init connection and client
	if d.Mocked {
		return nil
	}
	if conn, err := d.GetConn(d.DailOptions...); err != nil {
		return err
	} else {
		for k, v := range d.NewClientFuncs {
			d.Clients[k] = v(conn)
		}
	}
	return nil
}

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
	// retry codes
	if len(d.RetryCodes) > 0 {
		callOpts = append(callOpts, grpc_retry.WithCodes(d.RetryCodes...))
	}
	return callOpts
}

func (d *StubExt) GetConn(userOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if d.conn == nil {
		var opts []grpc.DialOption
		// opts: per call opts
		callOpts := d.getCallOpts()
		opts = append(
			opts,
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(callOpts...)),
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(callOpts...)),
		)
		// opts: authority
		if d.Authority != "" {
			opts = append(opts, grpc.WithAuthority(d.Authority))
		}
		// opts: apm
		if d.enableApm {
			opts = append(opts, grpc.WithUnaryInterceptor(apmgrpc.NewUnaryClientInterceptor()))
		}
		// opts: user opts
		opts = append(opts, userOpts...)
		// connect
		ctxDefault := context.Background()
		if d.ConnTimeout > 0 {
			ctx, cancel := context.WithTimeout(ctxDefault, d.ConnTimeout)
			ctxDefault = ctx
			defer cancel()
		}
		address := net.JoinHostPort(d.Host, strconv.Itoa(int(d.Port)))
		conn, err := grpc.DialContext(ctxDefault, address, opts...)
		if err != nil {
			return nil, err
		}
		d.conn = conn
	}
	return d.conn, nil
}

func (d *StubExt) GetCtx(ctx context.Context) context.Context {
	var md metadata.MD
	for k, v := range d.Metadata {
		md = metadata.Join(md, metadata.Pairs(k, v))
	}
	return metadata.NewOutgoingContext(ctx, md)
}
