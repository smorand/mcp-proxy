package proxy_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/proxy"
)

func TestScanner_ReadsLines(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1}
{"jsonrpc":"2.0","id":2}
`
	scanner := proxy.NewScanner(strings.NewReader(input))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, string(scanner.Bytes()))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
}

func TestWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	if err := proxy.WriteMessage(&buf, []byte(`{"id":1}`)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if buf.String() != `{"id":1}`+"\n" {
		t.Errorf("got %q", buf.String())
	}
}
