package stubext

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc/status"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/shanbay/gobay"
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
	uhUpstreamMsg        = "no healthy upstream"
	ErrUnHealthyUpStream = errors.New("grpc response body is no healthy upstream")
)

type StubExt struct {
	NS             string
	DialOptions    []grpc.DialOption
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
}

func (d *StubExt) Application() *gobay.Application { return d.app }

func (d *StubExt) Object() interface{} { return d }

func (d *StubExt) Close() error {
	if d.Mocked {
		return nil
	}
	return d.conn.Close()
}

// no healthy upstream的unary拦截器
func newUHUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		} else {
			e, _ := status.FromError(err)
			if e.Message() == uhUpstreamMsg {
				return ErrUnHealthyUpStream
			}
		}
		return err
	}
}

// no healthy upstream的stream拦截器
func newUHStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err == nil {
			return clientStream, nil
		} else {
			e, _ := status.FromError(err)
			if e.Message() == uhUpstreamMsg {
				return nil, ErrUnHealthyUpStream
			}
		}
		return clientStream, err
	}
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
	if conn, err := d.GetConn(d.DialOptions...); err != nil {
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
		// opts: authority
		if d.Authority != "" {
			opts = append(opts, grpc.WithAuthority(d.Authority))
		}
		// opts: user opts
		opts = append(opts, userOpts...)
		// opts: per call opts
		callOpts := d.getCallOpts()
		opts = append(
			opts,
			grpc.WithChainUnaryInterceptor(grpc_retry.UnaryClientInterceptor(callOpts...)),
			grpc.WithChainUnaryInterceptor(newUHUnaryInterceptor()),
			grpc.WithChainStreamInterceptor(grpc_retry.StreamClientInterceptor(callOpts...)),
			grpc.WithChainStreamInterceptor(newUHStreamInterceptor()),
		)
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
