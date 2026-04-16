package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Attribute represents a key-value pair attached to a span.
type Attribute struct {
	Key   string
	Value any
}

// StringAttr creates a string attribute.
func StringAttr(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

// IntAttr creates an integer attribute.
func IntAttr(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int64Attr creates an int64 attribute.
func Int64Attr(key string, value int64) Attribute {
	return Attribute{Key: key, Value: value}
}

// BoolAttr creates a boolean attribute.
func BoolAttr(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// SpanContext holds the state of an active span.
type SpanContext struct {
	tracer    *Tracer
	traceID   string
	spanID    string
	name      string
	startTime time.Time
	attrs     map[string]any
}

// SetAttribute adds or updates an attribute on the span.
func (s *SpanContext) SetAttribute(key string, value any) {
	s.attrs[key] = value
}

// End completes the span, recording error status if err is non-nil.
func (s *SpanContext) End(err error) {
	status := "ok"
	if err != nil {
		status = "error"
		s.attrs["error.type"] = "error"
		s.attrs["error.message"] = err.Error()
	}

	span := &Span{
		TraceID:     s.traceID,
		SpanID:      s.spanID,
		Name:        s.name,
		ServiceName: s.tracer.serviceName,
		StartTime:   s.startTime.Format(time.RFC3339Nano),
		EndTime:     time.Now().UTC().Format(time.RFC3339Nano),
		Status:      status,
		Attributes:  s.attrs,
	}

	s.tracer.exporter.Export(span)
}

// Tracer creates and exports trace spans as JSONL.
type Tracer struct {
	exporter    *FileExporter
	serviceName string
}

// NewTracer creates a tracer that writes JSONL spans to the given file path.
func NewTracer(tracePath, serviceName string) (*Tracer, error) {
	exp, err := NewFileExporter(tracePath)
	if err != nil {
		return nil, err
	}
	return &Tracer{exporter: exp, serviceName: serviceName}, nil
}

// StartSpan begins a new span with the given name and attributes.
func (t *Tracer) StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, *SpanContext) {
	span := &SpanContext{
		tracer:    t,
		traceID:   generateID(16),
		spanID:    generateID(8),
		name:      name,
		startTime: time.Now().UTC(),
		attrs:     make(map[string]any),
	}
	for _, attr := range attrs {
		span.attrs[attr.Key] = attr.Value
	}
	return ctx, span
}

// Close shuts down the tracer and flushes the exporter.
func (t *Tracer) Close() error {
	return t.exporter.Close()
}

func generateID(byteLen int) string {
	b := make([]byte, byteLen)
	rand.Read(b)
	return hex.EncodeToString(b)
}
