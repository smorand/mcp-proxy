package telemetry_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/telemetry"
)

func TestTracer_WritesSpansToFile(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "traces.jsonl")
	tracer, err := telemetry.NewTracer(tracePath, "mcp-proxy")
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	ctx := context.Background()
	_, oauthSpan := tracer.StartSpan(ctx, "oauth.token_check",
		telemetry.StringAttr("oauth.flow.step", "token_validation"),
		telemetry.StringAttr("mcp.server.url", "https://mcp.test.com"),
	)
	oauthSpan.End(nil)

	_, httpSpan := tracer.StartSpan(ctx, "http.forward",
		telemetry.StringAttr("http.method", "POST"),
		telemetry.IntAttr("http.status_code", 200),
	)
	httpSpan.SetAttribute("token.refresh_attempted", false)
	httpSpan.End(nil)

	if err := tracer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(tracePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("perm = %o, want 0600", info.Mode().Perm())
	}

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	for _, want := range []string{"oauth.token_check", "http.forward", "oauth.flow.step", "http.method", "http.status_code"} {
		if !strings.Contains(content, want) {
			t.Errorf("trace output missing %q", want)
		}
	}

	for _, sensitive := range []string{"access_token", "refresh_token", "client_secret", "code_verifier"} {
		if strings.Contains(content, sensitive) {
			t.Errorf("trace output contains sensitive %q", sensitive)
		}
	}
}

func TestTracer_ErrorSpan(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "traces.jsonl")
	tracer, err := telemetry.NewTracer(tracePath, "mcp-proxy")
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test.error", telemetry.StringAttr("test.key", "test.value"))
	span.End(os.ErrPermission)

	if err := tracer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "permission denied") {
		t.Errorf("error message not recorded: %s", content)
	}
}
