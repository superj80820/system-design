package trace

import (
	"context"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

func CreateTracer(ctx context.Context, serviceName string) (trace.Tracer, error) {
	return CreateTracerWithVersion(ctx, serviceName, "0.0.0")
}

func CreateNoOpTracer() trace.Tracer {
	return trace.NewNoopTracerProvider().Tracer("no-op")
}

func CreateTracerWithVersion(ctx context.Context, serviceName, serviceVersion string) (trace.Tracer, error) {
	client := otlptracegrpc.NewClient() // TODO: why grpc not http?
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "create tracer failed")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource(serviceName, serviceVersion)),
	)

	return tp.Tracer(serviceName), nil
}

func newResource(serviceName, serviceVersion string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
	)
}
