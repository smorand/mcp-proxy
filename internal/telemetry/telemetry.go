// Package telemetry wraps the OpenTelemetry SDK with a JSONL stdout exporter.
package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const traceFilePerm = 0600

// Tracer wraps the OpenTelemetry tracer provider and writes spans as JSONL.
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	file     *os.File
}

// NewTracer initialises a JSONL trace file and registers the global tracer
// provider. tracePath is the destination file (created with 0600 perms);
// serviceName is recorded as the service.name resource attribute.
func NewTracer(tracePath, serviceName string) (*Tracer, error) {
	// #nosec G304 -- tracePath is derived from the user's cache dir under control of this binary.
	f, err := os.OpenFile(tracePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, traceFilePerm)
	if err != nil {
		return nil, fmt.Errorf("open trace file %s: %w", tracePath, err)
	}
	if err := os.Chmod(tracePath, traceFilePerm); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("set trace file permissions %s: %w", tracePath, err)
	}

	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(f),
		stdouttrace.WithoutTimestamps(),
	)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("create stdout trace exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("build resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	return &Tracer{
		tracer:   provider.Tracer(serviceName),
		provider: provider,
		file:     f,
	}, nil
}

// StartSpan begins a new span with the given name and attributes. The caller
// must call End on the returned SpanContext.
func (t *Tracer) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, *SpanContext) {
	ctx, span := t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, &SpanContext{span: span}
}

// Close flushes spans and closes the trace file.
func (t *Tracer) Close() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := t.provider.Shutdown(ctx); err != nil {
		_ = t.file.Close()
		return fmt.Errorf("shutdown tracer provider: %w", err)
	}
	return t.file.Close()
}

// SpanContext wraps an OpenTelemetry span with helpers used in this codebase.
type SpanContext struct {
	span trace.Span
}

// SetAttribute adds or updates a single attribute on the span. Supported
// value kinds are string, int, int64, and bool; other kinds are stringified.
func (s *SpanContext) SetAttribute(key string, value any) {
	s.span.SetAttributes(toKeyValue(key, value))
}

// End finishes the span. If err is non-nil the span is marked Error and the
// error is recorded.
func (s *SpanContext) End(err error) {
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
		s.span.RecordError(err)
	} else {
		s.span.SetStatus(codes.Ok, "")
	}
	s.span.End()
}

// StringAttr returns a string attribute.
func StringAttr(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

// IntAttr returns an int attribute.
func IntAttr(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

// Int64Attr returns an int64 attribute.
func Int64Attr(key string, value int64) attribute.KeyValue {
	return attribute.Int64(key, value)
}

// BoolAttr returns a bool attribute.
func BoolAttr(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}

func toKeyValue(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case float64:
		return attribute.Float64(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}
