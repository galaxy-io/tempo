package ui

import (
	"fmt"
	"time"

	"github.com/atterpac/loom/internal/config"
	"github.com/atterpac/loom/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ResetPicker provides an enhanced UI for selecting a workflow reset point.
type ResetPicker struct {
	*tview.Box
	resetPoints   []temporal.ResetPoint
	selectedIndex int
	quickReset    bool // If true, auto-select the most recent failure point

	onSelect func(eventID int64, description string)
	onCancel func()
}

// NewResetPicker creates a new reset picker with the given reset points.
func NewResetPicker(resetPoints []temporal.ResetPoint) *ResetPicker {
	rp := &ResetPicker{
		Box:         tview.NewBox(),
		resetPoints: resetPoints,
	}
	rp.SetBackgroundColor(ColorBg())

	OnThemeChange(func(_ *config.ParsedTheme) {
		rp.SetBackgroundColor(ColorBg())
	})

	return rp
}

// SetOnSelect sets the selection callback.
func (rp *ResetPicker) SetOnSelect(fn func(eventID int64, description string)) {
	rp.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (rp *ResetPicker) SetOnCancel(fn func()) {
	rp.onCancel = fn
}

// SetQuickReset enables quick reset mode (auto-selects first failure point).
func (rp *ResetPicker) SetQuickReset(enabled bool) {
	rp.quickReset = enabled
}

// GetHeight returns the preferred height for this component.
func (rp *ResetPicker) GetHeight() int {
	rows := len(rp.resetPoints)
	if rows == 0 {
		rows = 1 // "No reset points" message
	}
	if rows > 10 {
		rows = 10 // Cap at 10 rows
	}
	return rows + 6 // rows + border + title + column headers + hints
}

// Draw renders the reset picker.
func (rp *ResetPicker) Draw(screen tcell.Screen) {
	rp.Box.DrawForSubclass(screen, rp)

	x, y, width, height := rp.GetInnerRect()
	if width <= 0 || height < 5 {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(ColorPanelBorder()).Background(ColorBg())
	titleStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg()).Bold(true)
	headerStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	itemStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())
	selectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())
	descStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	hintStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	failureStyle := tcell.StyleDefault.Foreground(ColorFailed()).Background(ColorBg())

	// Draw border
	screen.SetContent(x, y, '╭', nil, borderStyle)
	screen.SetContent(x+width-1, y, '╮', nil, borderStyle)
	screen.SetContent(x, y+height-1, '╰', nil, borderStyle)
	screen.SetContent(x+width-1, y+height-1, '╯', nil, borderStyle)

	for i := x + 1; i < x+width-1; i++ {
		screen.SetContent(i, y, '─', nil, borderStyle)
		screen.SetContent(i, y+height-1, '─', nil, borderStyle)
	}
	for i := y + 1; i < y+height-1; i++ {
		screen.SetContent(x, i, '│', nil, borderStyle)
		screen.SetContent(x+width-1, i, '│', nil, borderStyle)
	}

	// Title
	title := " Reset Points "
	titleX := x + (width-len(title))/2
	for i, r := range []rune(title) {
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Column headers
	headerY := y + 1
	headers := "  EVENT   TYPE                    TIME        DESCRIPTION"
	for i, r := range []rune(headers) {
		if x+2+i < x+width-1 {
			screen.SetContent(x+2+i, headerY, r, nil, headerStyle)
		}
	}

	// Separator
	sepY := y + 2
	screen.SetContent(x, sepY, '├', nil, borderStyle)
	screen.SetContent(x+width-1, sepY, '┤', nil, borderStyle)
	for i := x + 1; i < x+width-1; i++ {
		screen.SetContent(i, sepY, '─', nil, borderStyle)
	}

	// Draw reset points or empty state
	if len(rp.resetPoints) == 0 {
		emptyMsg := "No valid reset points found for this workflow."
		msgX := x + (width-len(emptyMsg))/2
		msgY := y + height/2
		if msgY >= y+3 && msgY < y+height-1 {
			for i, r := range []rune(emptyMsg) {
				if msgX+i > x && msgX+i < x+width-1 {
					screen.SetContent(msgX+i, msgY, r, nil, descStyle)
				}
			}
		}
	} else {
		maxRows := height - 5
		if maxRows > len(rp.resetPoints) {
			maxRows = len(rp.resetPoints)
		}

		for i := 0; i < maxRows; i++ {
			point := rp.resetPoints[i]
			rowY := y + 3 + i
			if rowY >= y+height-2 {
				break
			}

			style := itemStyle
			dStyle := descStyle
			if i == rp.selectedIndex {
				style = selectedStyle
				dStyle = selectedStyle
			}

			// Check if this is a failure event
			isFailure := isFailureEvent(point.EventType)
			eventStyle := style
			if isFailure && i != rp.selectedIndex {
				eventStyle = failureStyle
			}

			// Clear row
			for cx := x + 1; cx < x+width-1; cx++ {
				screen.SetContent(cx, rowY, ' ', nil, style)
			}

			// Draw marker
			marker := "  "
			if i == rp.selectedIndex {
				marker = IconArrowRight + " "
			}
			for mi, r := range []rune(marker) {
				if x+2+mi < x+width-1 {
					screen.SetContent(x+2+mi, rowY, r, nil, style)
				}
			}

			// Event ID (8 chars)
			eventID := fmt.Sprintf("%-8d", point.EventID)
			for ei, r := range []rune(eventID) {
				if x+4+ei < x+width-1 {
					screen.SetContent(x+4+ei, rowY, r, nil, eventStyle)
				}
			}

			// Event Type (22 chars)
			eventType := truncateStr(point.EventType, 22)
			eventType = fmt.Sprintf("%-22s", eventType)
			for ti, r := range []rune(eventType) {
				if x+12+ti < x+width-1 {
					screen.SetContent(x+12+ti, rowY, r, nil, eventStyle)
				}
			}

			// Time (12 chars)
			timeStr := point.Timestamp.Format("15:04:05")
			timeStr = fmt.Sprintf("%-12s", timeStr)
			for ti, r := range []rune(timeStr) {
				if x+34+ti < x+width-1 {
					screen.SetContent(x+34+ti, rowY, r, nil, style)
				}
			}

			// Description (remaining space)
			desc := truncateStr(point.Description, width-50)
			for di, r := range []rune(desc) {
				if x+46+di < x+width-2 {
					screen.SetContent(x+46+di, rowY, r, nil, dStyle)
				}
			}
		}
	}

	// Draw hints at bottom
	hintY := y + height - 1
	hint := " [↑↓] Navigate  [Enter] Reset to this point  [Esc] Cancel "
	hintX := x + (width-len(hint))/2
	for i, r := range []rune(hint) {
		if hintX+i > x && hintX+i < x+width-1 {
			screen.SetContent(hintX+i, hintY, r, nil, hintStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (rp *ResetPicker) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return rp.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if len(rp.resetPoints) > 0 && rp.selectedIndex >= 0 && rp.selectedIndex < len(rp.resetPoints) {
				point := rp.resetPoints[rp.selectedIndex]
				if rp.onSelect != nil {
					rp.onSelect(point.EventID, point.Description)
				}
			}
		case tcell.KeyEscape:
			if rp.onCancel != nil {
				rp.onCancel()
			}
		case tcell.KeyUp:
			if len(rp.resetPoints) > 0 {
				rp.selectedIndex--
				if rp.selectedIndex < 0 {
					rp.selectedIndex = len(rp.resetPoints) - 1
				}
			}
		case tcell.KeyDown:
			if len(rp.resetPoints) > 0 {
				rp.selectedIndex++
				if rp.selectedIndex >= len(rp.resetPoints) {
					rp.selectedIndex = 0
				}
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				if len(rp.resetPoints) > 0 {
					rp.selectedIndex++
					if rp.selectedIndex >= len(rp.resetPoints) {
						rp.selectedIndex = 0
					}
				}
			case 'k':
				if len(rp.resetPoints) > 0 {
					rp.selectedIndex--
					if rp.selectedIndex < 0 {
						rp.selectedIndex = len(rp.resetPoints) - 1
					}
				}
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				// Quick select by number
				idx := int(event.Rune() - '1')
				if idx >= 0 && idx < len(rp.resetPoints) {
					rp.selectedIndex = idx
					point := rp.resetPoints[rp.selectedIndex]
					if rp.onSelect != nil {
						rp.onSelect(point.EventID, point.Description)
					}
				}
			}
		}
	})
}

// Focus sets focus to this picker.
func (rp *ResetPicker) Focus(delegate func(p tview.Primitive)) {
	rp.Box.Focus(delegate)
}

// GetFirstFailurePoint returns the first failure-related reset point if any.
func (rp *ResetPicker) GetFirstFailurePoint() (temporal.ResetPoint, bool) {
	for _, point := range rp.resetPoints {
		if isFailureEvent(point.EventType) {
			return point, true
		}
	}
	return temporal.ResetPoint{}, false
}

// SelectFirstFailure selects the first failure point.
func (rp *ResetPicker) SelectFirstFailure() bool {
	for i, point := range rp.resetPoints {
		if isFailureEvent(point.EventType) {
			rp.selectedIndex = i
			return true
		}
	}
	return false
}

// isFailureEvent checks if an event type represents a failure.
func isFailureEvent(eventType string) bool {
	failureEvents := []string{
		"ActivityTaskFailed",
		"ActivityTaskTimedOut",
		"WorkflowTaskFailed",
		"WorkflowTaskTimedOut",
		"WorkflowExecutionFailed",
		"WorkflowExecutionTimedOut",
		"ChildWorkflowExecutionFailed",
		"ChildWorkflowExecutionTimedOut",
	}
	for _, fe := range failureEvents {
		if eventType == fe {
			return true
		}
	}
	return false
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// QuickResetModal provides a simple confirmation for quick reset.
type QuickResetModal struct {
	*Modal
	resetPoint  temporal.ResetPoint
	workflowID  string
	onConfirm   func()
	onAdvanced  func() // Open full picker
	onCancel    func()
}

// NewQuickResetModal creates a quick reset confirmation modal.
func NewQuickResetModal(workflowID string, resetPoint temporal.ResetPoint) *QuickResetModal {
	qrm := &QuickResetModal{
		Modal: NewModal(ModalConfig{
			Title:    "Quick Reset",
			Width:    60,
			Height:   12,
			Backdrop: true,
		}),
		resetPoint: resetPoint,
		workflowID: workflowID,
	}
	qrm.setup()
	return qrm
}

func (qrm *QuickResetModal) setup() {
	content := tview.NewTextView().SetDynamicColors(true)
	content.SetBackgroundColor(ColorBg())

	text := fmt.Sprintf(`[%s]Last Failure Detected[-]

[%s]Event:[-] [%s]%d - %s[-]
[%s]Time:[-] [%s]%s[-]
[%s]Description:[-]
[%s]%s[-]

Reset workflow [%s]%s[-] to this point?`,
		TagAccent(),
		TagFgDim(), TagFg(), qrm.resetPoint.EventID, qrm.resetPoint.EventType,
		TagFgDim(), TagFg(), qrm.resetPoint.Timestamp.Format(time.RFC3339),
		TagFgDim(),
		TagFg(), qrm.resetPoint.Description,
		TagAccent(), truncateStr(qrm.workflowID, 30))

	content.SetText(text)
	qrm.SetContent(content)
	qrm.SetHints([]KeyHint{
		{Key: "Enter", Description: "Reset"},
		{Key: "a", Description: "Advanced"},
		{Key: "Esc", Description: "Cancel"},
	})
}

// SetOnConfirm sets the confirm callback.
func (qrm *QuickResetModal) SetOnConfirm(fn func()) {
	qrm.onConfirm = fn
}

// SetOnAdvanced sets the callback to open the full picker.
func (qrm *QuickResetModal) SetOnAdvanced(fn func()) {
	qrm.onAdvanced = fn
}

// SetOnCancel sets the cancel callback.
func (qrm *QuickResetModal) SetOnCancel(fn func()) {
	qrm.onCancel = fn
	qrm.Modal.SetOnClose(fn)
}

// InputHandler handles keyboard input.
func (qrm *QuickResetModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return qrm.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if qrm.onConfirm != nil {
				qrm.onConfirm()
			}
		case tcell.KeyEscape:
			if qrm.onCancel != nil {
				qrm.onCancel()
			}
		case tcell.KeyRune:
			if event.Rune() == 'a' || event.Rune() == 'A' {
				if qrm.onAdvanced != nil {
					qrm.onAdvanced()
				}
			}
		}
	})
}

// Focus focuses the modal.
func (qrm *QuickResetModal) Focus(delegate func(p tview.Primitive)) {
	qrm.Flex.Focus(delegate)
}

// GetResetPoint returns the reset point.
func (qrm *QuickResetModal) GetResetPoint() temporal.ResetPoint {
	return qrm.resetPoint
}
