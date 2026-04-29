package oauth

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/smorand/mcp-proxy/internal/apperr"
)

// OpenBrowser launches the system default browser at the given URL.
// Supported platforms: macOS (open), Linux (xdg-open), Windows (start).
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204 -- url is the OAuth authorize URL we just built.
	case "linux":
		cmd = exec.Command("xdg-open", url) // #nosec G204 -- url is the OAuth authorize URL we just built.
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url) // #nosec G204 -- url is the OAuth authorize URL we just built.
	default:
		return apperr.NewConfigError(
			fmt.Sprintf("unsupported operating system: %s", runtime.GOOS),
			nil,
		)
	}

	if err := cmd.Start(); err != nil {
		return apperr.NewConfigError(
			fmt.Sprintf("failed to open browser: %v", err),
			err,
		)
	}
	return nil
}
