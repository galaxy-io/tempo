package ui

import (
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard copies the given text to the system clipboard.
// Returns nil on success, or an error if the clipboard is unavailable.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fallback to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else if _, err := exec.LookPath("wl-copy"); err == nil {
			// Wayland
			cmd = exec.Command("wl-copy")
		} else {
			return &ClipboardError{Message: "no clipboard tool found (install xclip, xsel, or wl-copy)"}
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return &ClipboardError{Message: "unsupported platform: " + runtime.GOOS}
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// ClipboardError represents a clipboard operation error.
type ClipboardError struct {
	Message string
}

func (e *ClipboardError) Error() string {
	return "clipboard: " + e.Message
}
