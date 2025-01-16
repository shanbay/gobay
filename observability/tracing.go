package observability

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"go.elastic.co/apm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	globalApmTracer  atomic.Value
	setApmTracerOnce sync.Once
)

func Initialize() func(ctx context.Context) error {
	initApm()
	shutdownOtel := initOtel()
	return func(ctx context.Context) error {
		if shutdownOtel != nil {
			return shutdownOtel(ctx)
		}
		if tracer := ApmTracer(); tracer != nil {
			tracer.Close()
		}
		return nil
	}
}

func GetApmEnable() bool {
	return os.Getenv("APM_ENABLE") == "true"
}

func GetOtelEnable() bool {
	return os.Getenv("OTEL_ENABLE") == "true"
}

type tracerHolder struct {
	tracer *apm.Tracer
}

func ApmTracer() *apm.Tracer {
	value := globalApmTracer.Load()
	if value == nil {
		return nil
	}
	return value.(tracerHolder).tracer
}

func initApm() {
	if !GetApmEnable() {
		return
	}

	setApmTracerOnce.Do(func() {
		globalApmTracer.Store(tracerHolder{tracer: apm.DefaultTracer})
	})
}

func initOtel() func(ctx context.Context) error {
	if !GetOtelEnable() {
		return nil
	}

	conn, err := initConn(os.Getenv("OTEL_SERVER_URL"))
	if err != nil {
		log.Fatalf("failed to create gRPC connection: %v\n", err)
	}
	ctx := context.Background()
	res, _ := resource.New(ctx,
		resource.WithAttributes(
			attribute.KeyValue{
				Key:   attribute.Key("service.name"),
				Value: attribute.StringValue(os.Getenv("OTEL_SERVICE_NAME")),
			}))
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v\n", err)
	}
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tracerProvider.Shutdown
}

func initConn(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}
	return conn, err
}
