package observability

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitOtel(config *viper.Viper) func(ctx context.Context) error {
	if !config.GetBool("otel_enable") {
		return nil
	}

	conn, err := initConn(config.GetString("otel_service_address"))
	if err != nil {
		log.Fatalf("failed to create gRPC connection: %v\n", err)
	}
	ctx := context.Background()
	res, _ := resource.New(ctx,
		resource.WithAttributes(
			attribute.KeyValue{
				Key:   attribute.Key("service.name"),
				Value: attribute.StringValue(config.GetString("otel_service_name")),
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
