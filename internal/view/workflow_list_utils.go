package view

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/atterpac/jig/theme"
)

// ptr returns a pointer to the given value.
func ptr[T any](v T) *T {
	return &v
}

// formatRelativeTime formats a time as a human-readable relative string.
func formatRelativeTime(now time.Time, t time.Time) string {
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}

// truncate truncates a string to maxLen, adding ellipsis if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// truncateIfNeeded only truncates if the string exceeds maxLen.
// If maxLen is 0 or negative, returns the string unchanged.
func truncateIfNeeded(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("clipboard not available: install xclip or xsel")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := pipe.Write([]byte(text)); err != nil {
		return err
	}

	if err := pipe.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// copyWorkflowID copies the selected workflow ID to clipboard.
func (wl *WorkflowList) copyWorkflowID() {
	row := wl.table.SelectedRow()
	if row < 0 || row >= len(wl.workflows) {
		return
	}

	wf := wl.workflows[row]
	if err := copyToClipboard(wf.ID); err != nil {
		wl.preview.SetText(fmt.Sprintf("[%s]%s Failed to copy: %s[-]",
			theme.TagError(), theme.IconError, err.Error()))
		return
	}

	wl.preview.SetText(fmt.Sprintf(`[%s::b]Copied to clipboard[-:-:-]

[%s]%s[-]

[%s]Workflow ID copied![-]`,
		theme.TagPanelTitle(),
		theme.TagAccent(), wf.ID,
		theme.TagSuccess()))

	go func() {
		time.Sleep(1500 * time.Millisecond)
		wl.app.JigApp().QueueUpdateDraw(func() {
			if row < len(wl.workflows) {
				wl.updatePreview(wl.workflows[row])
			}
		})
	}()
}
