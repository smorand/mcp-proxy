package proxy_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smorand/mcp-proxy/internal/proxy"
)

func TestHandler_ForwardsJSONResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT-1" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
	}))
	defer server.Close()

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		AccessToken: "AT-1",
		HTTPClient:  server.Client(),
		ServerURL:   server.URL,
		Stdin:       stdin,
		Stdout:      &stdout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(stdout.String(), `"result":"ok"`) {
		t.Errorf("stdout missing result: %s", stdout.String())
	}
}

func TestHandler_HandlesAccepted(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"notify"}` + "\n")
	var stdout bytes.Buffer

	handler := proxy.NewHandler(proxy.HandlerConfig{
		AccessToken: "AT-1",
		HTTPClient:  server.Client(),
		ServerURL:   server.URL,
		Stdin:       stdin,
		Stdout:      &stdout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := handler.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output, got %q", stdout.String())
	}
}

func TestWriteSSEDataTo(t *testing.T) {
	body := strings.NewReader("event: message\ndata: {\"id\":1}\n\nignored after empty\n")
	rc := struct {
		*strings.Reader
	}{body}

	var buf bytes.Buffer
	if err := proxy.WriteSSEDataTo(&buf, &nopCloser{rc.Reader}); err != nil {
		t.Fatalf("WriteSSEDataTo: %v", err)
	}
	if !strings.Contains(buf.String(), `{"id":1}`) {
		t.Errorf("got %q", buf.String())
	}
}

type nopCloser struct{ *strings.Reader }

func (nopCloser) Close() error { return nil }
