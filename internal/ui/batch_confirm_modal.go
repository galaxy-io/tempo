package ui

import (
	"fmt"
	"strings"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BatchOperation represents the type of batch operation.
type BatchOperation int

const (
	BatchCancel BatchOperation = iota
	BatchTerminate
	BatchDelete
)

func (b BatchOperation) String() string {
	switch b {
	case BatchCancel:
		return "Cancel"
	case BatchTerminate:
		return "Terminate"
	case BatchDelete:
		return "Delete"
	default:
		return "Unknown"
	}
}

// BatchItem represents a single item in a batch operation.
type BatchItem struct {
	ID     string
	RunID  string
	Status string // "pending", "in_progress", "completed", "failed"
	Error  string
}

// BatchConfirmModal displays a confirmation dialog for batch operations with progress tracking.
type BatchConfirmModal struct {
	*Modal
	textView   *tview.TextView
	nav        *TextViewNavigator
	operation  BatchOperation
	items      []BatchItem
	onConfirm  func()
	onCancel   func()
	inProgress bool
	completed  int
	failed     int
}

// NewBatchConfirmModal creates a new batch confirmation modal.
func NewBatchConfirmModal(op BatchOperation, items []BatchItem) *BatchConfirmModal {
	bm := &BatchConfirmModal{
		Modal: NewModal(ModalConfig{
			Title:     fmt.Sprintf("%s %d Workflows", op.String(), len(items)),
			Width:     60,
			Height:    20,
			MinHeight: 12,
			MaxHeight: 25,
			Backdrop:  true,
		}),
		textView:  tview.NewTextView(),
		operation: op,
		items:     items,
	}
	bm.setup()
	return bm
}

// SetOnConfirm sets the callback when user confirms the operation.
func (bm *BatchConfirmModal) SetOnConfirm(fn func()) *BatchConfirmModal {
	bm.onConfirm = fn
	return bm
}

// SetOnCancel sets the callback when user cancels.
func (bm *BatchConfirmModal) SetOnCancel(fn func()) *BatchConfirmModal {
	bm.onCancel = fn
	bm.Modal.SetOnClose(fn)
	return bm
}

func (bm *BatchConfirmModal) setup() {
	bm.textView.SetDynamicColors(true)
	bm.textView.SetBackgroundColor(ColorBg())
	bm.textView.SetScrollable(true)

	bm.nav = NewTextViewNavigator(bm.textView)

	bm.rebuildContent()

	bm.SetContent(bm.textView)
	bm.updateHints()

	// Register for theme changes
	OnThemeChange(func(_ *config.ParsedTheme) {
		bm.textView.SetBackgroundColor(ColorBg())
		bm.rebuildContent()
	})
}

func (bm *BatchConfirmModal) updateHints() {
	if bm.inProgress {
		bm.SetHints([]KeyHint{
			{Key: "j/k", Description: "Scroll"},
		})
	} else {
		bm.SetHints([]KeyHint{
			{Key: "Enter", Description: "Confirm"},
			{Key: "j/k", Description: "Scroll"},
			{Key: "Esc", Description: "Cancel"},
		})
	}
}

func (bm *BatchConfirmModal) rebuildContent() {
	var sb strings.Builder

	sb.WriteString("\n")

	if bm.inProgress {
		// Show progress view
		total := len(bm.items)
		processed := bm.completed + bm.failed

		// Progress bar
		progressWidth := 30
		filledWidth := 0
		if total > 0 {
			filledWidth = (processed * progressWidth) / total
		}
		progressBar := strings.Repeat(IconProgressFilled, filledWidth) +
			strings.Repeat(IconProgressEmpty, progressWidth-filledWidth)

		sb.WriteString(fmt.Sprintf("[%s::b]%s %d workflows...[-:-:-]\n\n",
			TagAccent(), bm.operation.String()+"ing", total))
		sb.WriteString(fmt.Sprintf("[%s]%s[-] %d/%d\n\n",
			TagAccent(), progressBar, processed, total))

		// Show individual items with status
		for _, item := range bm.items {
			icon := IconPending
			colorTag := TagFgDim()
			switch item.Status {
			case "in_progress":
				icon = IconRunning
				colorTag = TagAccent()
			case "completed":
				icon = IconCompleted
				colorTag = TagCompleted()
			case "failed":
				icon = IconFailed
				colorTag = TagFailed()
			}

			sb.WriteString(fmt.Sprintf("[%s]%s %s[-]", colorTag, icon, truncateStringForDisplay(item.ID, 40)))
			if item.Error != "" {
				sb.WriteString(fmt.Sprintf(" [%s](%s)[-]", TagFailed(), truncateStringForDisplay(item.Error, 20)))
			}
			sb.WriteString("\n")
		}

		// Summary if complete
		if processed == total {
			sb.WriteString(fmt.Sprintf("\n[%s::b]Complete:[-:-:-] ", TagPanelTitle()))
			sb.WriteString(fmt.Sprintf("[%s]%d succeeded[-]", TagCompleted(), bm.completed))
			if bm.failed > 0 {
				sb.WriteString(fmt.Sprintf(", [%s]%d failed[-]", TagFailed(), bm.failed))
			}
			sb.WriteString("\n")
		}
	} else {
		// Show confirmation view
		warningMsg := ""
		switch bm.operation {
		case BatchCancel:
			warningMsg = "This will request graceful cancellation. Workflows can handle the cancellation."
		case BatchTerminate:
			warningMsg = "This will forcefully terminate workflows immediately. No cleanup code will run."
		case BatchDelete:
			warningMsg = "This will permanently delete workflow executions and history. This cannot be undone."
		}

		sb.WriteString(fmt.Sprintf("[%s::b]%s %d workflows?[-:-:-]\n\n",
			TagAccent(), bm.operation.String(), len(bm.items)))

		if warningMsg != "" {
			sb.WriteString(fmt.Sprintf("[%s]%s %s[-]\n\n", TagFailed(), IconWarning, warningMsg))
		}

		sb.WriteString(fmt.Sprintf("[%s]Workflows:[-]\n", TagFgDim()))
		for i, item := range bm.items {
			if i >= 10 && len(bm.items) > 12 {
				sb.WriteString(fmt.Sprintf("[%s]  ... and %d more[-]\n", TagFgDim(), len(bm.items)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("[%s]  %s %s[-]\n", TagFg(), IconArrowRight, truncateStringForDisplay(item.ID, 45)))
		}
	}

	bm.textView.SetText(sb.String())

	// Adjust height based on content
	lineCount := strings.Count(sb.String(), "\n")
	height := min(lineCount+4, 22)
	if height < 12 {
		height = 12
	}
	bm.SetSize(60, height)
}

// StartProgress switches the modal to progress mode.
func (bm *BatchConfirmModal) StartProgress() {
	bm.inProgress = true
	bm.completed = 0
	bm.failed = 0
	for i := range bm.items {
		bm.items[i].Status = "pending"
		bm.items[i].Error = ""
	}
	bm.updateHints()
	bm.rebuildContent()
}

// UpdateItemProgress updates the progress of a specific item.
func (bm *BatchConfirmModal) UpdateItemProgress(index int, status string, err string) {
	if index < 0 || index >= len(bm.items) {
		return
	}
	bm.items[index].Status = status
	bm.items[index].Error = err

	if status == "completed" {
		bm.completed++
	} else if status == "failed" {
		bm.failed++
	}

	bm.rebuildContent()
}

// MarkItemInProgress marks an item as in progress.
func (bm *BatchConfirmModal) MarkItemInProgress(index int) {
	bm.UpdateItemProgress(index, "in_progress", "")
}

// MarkItemCompleted marks an item as completed.
func (bm *BatchConfirmModal) MarkItemCompleted(index int) {
	bm.UpdateItemProgress(index, "completed", "")
}

// MarkItemFailed marks an item as failed.
func (bm *BatchConfirmModal) MarkItemFailed(index int, err string) {
	bm.UpdateItemProgress(index, "failed", err)
}

// IsComplete returns true if all items have been processed.
func (bm *BatchConfirmModal) IsComplete() bool {
	return bm.completed+bm.failed >= len(bm.items)
}

// GetResults returns the count of completed and failed operations.
func (bm *BatchConfirmModal) GetResults() (completed, failed int) {
	return bm.completed, bm.failed
}

// InputHandler handles keyboard input.
func (bm *BatchConfirmModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return bm.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if !bm.inProgress && bm.onConfirm != nil {
				bm.onConfirm()
			} else if bm.IsComplete() && bm.onCancel != nil {
				bm.onCancel()
			}
		case tcell.KeyEscape:
			if !bm.inProgress && bm.onCancel != nil {
				bm.onCancel()
			} else if bm.IsComplete() && bm.onCancel != nil {
				bm.onCancel()
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				bm.nav.MoveDown()
			case 'k':
				bm.nav.MoveUp()
			case 'q':
				if bm.IsComplete() && bm.onCancel != nil {
					bm.onCancel()
				}
			}
		case tcell.KeyDown:
			bm.nav.MoveDown()
		case tcell.KeyUp:
			bm.nav.MoveUp()
		}
	})
}

// Focus delegates focus to the text view.
func (bm *BatchConfirmModal) Focus(delegate func(p tview.Primitive)) {
	delegate(bm.textView)
}

// truncateStringForDisplay truncates a string for display.
func truncateStringForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Progress bar icons (additional icons not in styles.go)
const (
	IconProgressFilled = "\u2588" // Full block
	IconProgressEmpty  = "\u2591" // Light shade
	IconWarning        = "\u26A0" // Warning sign
)
