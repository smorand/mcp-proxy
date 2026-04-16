package tests

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/telemetry"
)

// TestE2E020_OpenTelemetryTraceFileCreated tests that traces are written to a JSONL file
// with correct permissions and expected attributes.
func TestE2E020_OpenTelemetryTraceFileCreated(t *testing.T) {
	// Create a temp directory for traces
	tmpDir, err := os.MkdirTemp("", "mcp-proxy-traces-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tracePath := tmpDir + "/traces.jsonl"

	tracer, err := telemetry.NewTracer(tracePath, "mcp-proxy")
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	// Write spans with various attributes
	ctx := context.Background()

	// OAuth flow step span
	_, oauthSpan := tracer.StartSpan(ctx, "oauth.token_check",
		telemetry.StringAttr("oauth.flow.step", "token_validation"),
		telemetry.StringAttr("mcp.server.url", "https://mcp.test.com"),
	)
	oauthSpan.End(nil)

	// HTTP request span
	_, httpSpan := tracer.StartSpan(ctx, "http.forward",
		telemetry.StringAttr("http.method", "POST"),
		telemetry.IntAttr("http.status_code", 200),
		telemetry.StringAttr("mcp.server.url", "https://mcp.test.com"),
	)
	httpSpan.End(nil)

	tracer.Close()

	// Verify file was created
	info, err := os.Stat(tracePath)
	if err != nil {
		t.Fatalf("Trace file not created: %v", err)
	}

	// Verify file permissions are 0600
	if info.Mode().Perm() != 0600 {
		t.Errorf("Trace file permissions = %o, want 0600", info.Mode().Perm())
	}

	// Read and verify content
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 trace entries, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var span map[string]any
		if err := json.Unmarshal([]byte(line), &span); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i, err)
			continue
		}

		// Verify required fields exist
		requiredFields := []string{"trace_id", "span_id", "name", "start_time", "end_time", "status", "attributes"}
		for _, field := range requiredFields {
			if _, ok := span[field]; !ok {
				t.Errorf("Line %d missing required field %q", i, field)
			}
		}
	}

	// Verify trace entries include expected attributes (across all entries)
	if !strings.Contains(content, `"oauth.flow.step"`) {
		t.Error("Traces should contain oauth.flow.step attribute")
	}
	if !strings.Contains(content, `"http.method"`) {
		t.Error("Traces should contain http.method attribute")
	}
	if !strings.Contains(content, `"http.status_code"`) {
		t.Error("Traces should contain http.status_code attribute")
	}

	// Verify sensitive data is NOT in traces
	sensitiveValues := []string{"access_token", "refresh_token", "client_secret", "code_verifier"}
	for _, sensitive := range sensitiveValues {
		if strings.Contains(content, sensitive) {
			t.Errorf("Traces should NOT contain sensitive data %q", sensitive)
		}
	}
}

// TestTracer_ErrorSpan tests that error spans record the error correctly.
func TestTracer_ErrorSpan(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "traces-err-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tracer, err := telemetry.NewTracer(tmpFile.Name(), "mcp-proxy")
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test.error",
		telemetry.StringAttr("test.key", "test.value"),
	)
	span.End(os.ErrPermission)
	tracer.Close()

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var spanData map[string]any
	if err := json.Unmarshal(data, &spanData); err != nil {
		t.Fatalf("Failed to parse span JSON: %v", err)
	}

	if spanData["status"] != "error" {
		t.Errorf("status = %q, want error", spanData["status"])
	}

	attrs, ok := spanData["attributes"].(map[string]any)
	if !ok {
		t.Fatal("attributes should be a map")
	}
	if attrs["error.type"] != "error" {
		t.Errorf("error.type = %q, want error", attrs["error.type"])
	}
	if attrs["error.message"] != "permission denied" {
		t.Errorf("error.message = %q, want 'permission denied'", attrs["error.message"])
	}
}

// TestTracer_SpanAttributes tests that span attributes are correctly recorded.
func TestTracer_SpanAttributes(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "traces-attr-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tracer, err := telemetry.NewTracer(tmpFile.Name(), "mcp-proxy")
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test.attrs",
		telemetry.StringAttr("str.key", "str-value"),
		telemetry.IntAttr("int.key", 42),
		telemetry.BoolAttr("bool.key", true),
	)
	span.SetAttribute("added.later", "dynamic-value")
	span.End(nil)
	tracer.Close()

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var spanData map[string]any
	if err := json.Unmarshal(data, &spanData); err != nil {
		t.Fatalf("Failed to parse span JSON: %v", err)
	}

	attrs, ok := spanData["attributes"].(map[string]any)
	if !ok {
		t.Fatal("attributes should be a map")
	}

	if attrs["str.key"] != "str-value" {
		t.Errorf("str.key = %q, want str-value", attrs["str.key"])
	}
	// JSON numbers are float64
	if attrs["int.key"] != float64(42) {
		t.Errorf("int.key = %v, want 42", attrs["int.key"])
	}
	if attrs["bool.key"] != true {
		t.Errorf("bool.key = %v, want true", attrs["bool.key"])
	}
	if attrs["added.later"] != "dynamic-value" {
		t.Errorf("added.later = %q, want dynamic-value", attrs["added.later"])
	}

	if spanData["status"] != "ok" {
		t.Errorf("status = %q, want ok", spanData["status"])
	}
	if spanData["service_name"] != "mcp-proxy" {
		t.Errorf("service_name = %q, want mcp-proxy", spanData["service_name"])
	}
}

// TestTracer_UniqueIDs tests that each span gets unique trace and span IDs.
func TestTracer_UniqueIDs(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "traces-ids-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	tracer, err := telemetry.NewTracer(tmpFile.Name(), "mcp-proxy")
	if err != nil {
		t.Fatalf("Failed to create tracer: %v", err)
	}

	ctx := context.Background()
	_, span1 := tracer.StartSpan(ctx, "span1")
	span1.End(nil)
	_, span2 := tracer.StartSpan(ctx, "span2")
	span2.End(nil)
	tracer.Close()

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	var s1, s2 map[string]any
	json.Unmarshal([]byte(lines[0]), &s1)
	json.Unmarshal([]byte(lines[1]), &s2)

	if s1["trace_id"] == s2["trace_id"] {
		t.Error("trace_ids should be unique across spans")
	}
	if s1["span_id"] == s2["span_id"] {
		t.Error("span_ids should be unique across spans")
	}
}
