package oauth

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/smorand/mcp-proxy/internal/errors"
)

// OpenBrowser opens the system default browser to the specified URL
// It uses platform-specific commands:
// - macOS: open
// - Linux: xdg-open
// - Windows: start
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return errors.NewConfigError(
			fmt.Sprintf("unsupported operating system: %s", runtime.GOOS),
			nil,
		)
	}

	if err := cmd.Start(); err != nil {
		return errors.NewConfigError(
			fmt.Sprintf("failed to open browser: %v", err),
			err,
		)
	}

	return nil
}
