package tests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestE2E005_EmptyClientID tests that empty client_id causes clear error
func TestE2E005_EmptyClientID(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Unset GOOGLE_CLIENT_ID
	os.Unsetenv("GOOGLE_CLIENT_ID")

	// Run with URL but no client_id
	cmd := exec.Command("./mcp-proxy-test", "-u", "https://mcp.test.com")
	output, err := cmd.CombinedOutput()

	// Should exit with code 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Error("Expected command to fail with exit code 1")
	}

	// Check error message
	outputStr := string(output)
	expectedMsg := "Error: client_id is required. Set via --client-id flag or GOOGLE_CLIENT_ID environment variable"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message containing %q, got: %s", expectedMsg, outputStr)
	}
}

// TestE2E006_EmptyClientSecret tests that empty client_secret causes clear error
func TestE2E006_EmptyClientSecret(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Set GOOGLE_CLIENT_ID but unset GOOGLE_CLIENT_SECRET
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run with URL and client_id but no client_secret
	cmd := exec.Command("./mcp-proxy-test", "-u", "https://mcp.test.com")
	output, err := cmd.CombinedOutput()

	// Should exit with code 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Error("Expected command to fail with exit code 1")
	}

	// Check error message
	outputStr := string(output)
	expectedMsg := "Error: client_secret is required. Set via --client-secret flag or GOOGLE_CLIENT_SECRET environment variable"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message containing %q, got: %s", expectedMsg, outputStr)
	}
}

// TestE2E007_HTTPURLRejected tests that HTTP URLs are rejected
func TestE2E007_HTTPURLRejected(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Set credentials
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run with HTTP URL (not HTTPS)
	cmd := exec.Command("./mcp-proxy-test", "-u", "http://mcp.test.com")
	output, err := cmd.CombinedOutput()

	// Should exit with code 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Error("Expected command to fail with exit code 1")
	}

	// Check error message
	outputStr := string(output)
	expectedMsg := "Error: MCP server URL must use HTTPS. Insecure HTTP connections are not allowed"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message containing %q, got: %s", expectedMsg, outputStr)
	}
}

// TestE2E014_MalformedURL tests that malformed URLs are rejected
func TestE2E014_MalformedURL(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Set credentials
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test-secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run with malformed URL
	cmd := exec.Command("./mcp-proxy-test", "-u", "not-a-valid-url")
	output, err := cmd.CombinedOutput()

	// Should exit with code 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Error("Expected command to fail with exit code 1")
	}

	// Check error message
	outputStr := string(output)
	expectedMsg := "Error: Invalid MCP server URL format"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message containing %q, got: %s", expectedMsg, outputStr)
	}
}

// TestE2E029_SensitiveDataNeverInErrors tests that sensitive data is never in error messages
func TestE2E029_SensitiveDataNeverInErrors(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "mcp-proxy-test", "../main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("mcp-proxy-test")

	// Set credentials with sensitive values
	os.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "super-secret-value-12345")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Run with invalid URL to trigger error
	cmd := exec.Command("./mcp-proxy-test", "-u", "not-a-valid-url")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected command to fail")
	}

	// Check that sensitive data is NOT in output
	outputStr := string(output)
	sensitiveData := []string{
		"super-secret-value-12345",
		"access_token",
		"refresh_token",
		"client_secret",
	}

	for _, sensitive := range sensitiveData {
		if strings.Contains(outputStr, sensitive) {
			t.Errorf("Error message contains sensitive data: %q", sensitive)
		}
	}
}
