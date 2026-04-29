package config_test

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/smorand/mcp-proxy/internal/apperr"
	"github.com/smorand/mcp-proxy/internal/config"
)

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(os.NewFile(0, os.DevNull))
}

func TestParse_Success(t *testing.T) {
	resetFlags()
	t.Setenv("GOOGLE_CLIENT_ID", "cid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "csecret")
	os.Args = []string{"test", "--url", "https://mcp.example.com"}

	cfg, err := config.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.ServerURL != "https://mcp.example.com" {
		t.Errorf("ServerURL = %q", cfg.ServerURL)
	}
	if cfg.ClientID != "cid" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "csecret" {
		t.Errorf("ClientSecret = %q", cfg.ClientSecret)
	}
}

func TestParse_ExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envID    string
		envSec   string
		errSub   string
		wantCode int
	}{
		{
			name:     "missing url",
			args:     []string{"test"},
			envID:    "cid",
			envSec:   "cs",
			errSub:   "URL is required",
			wantCode: apperr.ExitConfigError,
		},
		{
			name:     "http rejected",
			args:     []string{"test", "--url", "http://insecure.example.com"},
			envID:    "cid",
			envSec:   "cs",
			errSub:   "HTTPS",
			wantCode: apperr.ExitConfigError,
		},
		{
			name:     "missing client_id",
			args:     []string{"test", "--url", "https://mcp.example.com"},
			envID:    "",
			envSec:   "cs",
			errSub:   "client_id is required",
			wantCode: apperr.ExitConfigError,
		},
		{
			name:     "missing client_secret",
			args:     []string{"test", "--url", "https://mcp.example.com"},
			envID:    "cid",
			envSec:   "",
			errSub:   "client_secret is required",
			wantCode: apperr.ExitConfigError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			t.Setenv("GOOGLE_CLIENT_ID", tt.envID)
			t.Setenv("GOOGLE_CLIENT_SECRET", tt.envSec)
			os.Args = tt.args

			_, err := config.Parse()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSub)
			}
			var appErr *apperr.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("error is not *apperr.AppError: %T", err)
			}
			if appErr.ExitCode != tt.wantCode {
				t.Errorf("ExitCode = %d, want %d", appErr.ExitCode, tt.wantCode)
			}
		})
	}
}
