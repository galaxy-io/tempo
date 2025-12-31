package view

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowDetail displays detailed information about a workflow with events.
type WorkflowDetail struct {
	*tview.Flex
	app              *App
	workflowID       string
	runID            string
	workflow         *temporal.Workflow
	events           []temporal.EnhancedHistoryEvent
	leftFlex         *tview.Flex
	workflowPanel    *components.Panel
	eventDetailPanel *components.Panel
	eventsPanel      *components.Panel
	workflowView     *tview.TextView
	eventDetailView  *tview.TextView
	eventTable       *components.Table
	loading          bool
}

// NewWorkflowDetail creates a new workflow detail view.
func NewWorkflowDetail(app *App, workflowID, runID string) *WorkflowDetail {
	wd := &WorkflowDetail{
		Flex:       tview.NewFlex().SetDirection(tview.FlexColumn),
		app:        app,
		workflowID: workflowID,
		runID:      runID,
		eventTable: components.NewTable(),
	}
	wd.setup()
	return wd
}

func (wd *WorkflowDetail) setup() {
	wd.SetBackgroundColor(theme.Bg())

	// Combined workflow info view
	wd.workflowView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	wd.workflowView.SetBackgroundColor(theme.Bg())

	// Event detail view
	wd.eventDetailView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	wd.eventDetailView.SetBackgroundColor(theme.Bg())

	// Event table
	wd.eventTable.SetHeaders("ID", "TIME", "TYPE", "NAME")
	wd.eventTable.SetBorder(false)
	wd.eventTable.SetBackgroundColor(theme.Bg())

	// Create panels with icons (blubber pattern)
	wd.workflowPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Workflow", theme.IconWorkflow))
	wd.workflowPanel.SetContent(wd.workflowView)

	wd.eventDetailPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Event Detail", theme.IconInfo))
	wd.eventDetailPanel.SetContent(wd.eventDetailView)

	wd.eventsPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Events", theme.IconEvent))
	wd.eventsPanel.SetContent(wd.eventTable)

	// Left side: workflow info + event detail stacked
	wd.leftFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	wd.leftFlex.SetBackgroundColor(theme.Bg())
	wd.leftFlex.AddItem(wd.workflowPanel, 0, 1, false)
	wd.leftFlex.AddItem(wd.eventDetailPanel, 0, 1, false)

	// Main layout: left stack + right events
	wd.AddItem(wd.leftFlex, 0, 2, false)
	wd.AddItem(wd.eventsPanel, 0, 3, true)

	// Update event detail when selection changes
	wd.eventTable.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row-1 < len(wd.events) {
			wd.updateEventDetail(wd.events[row-1])
		}
	})

	// Show loading state initially
	wd.workflowView.SetText(fmt.Sprintf("\n [%s]Loading...[-]", theme.TagFgDim()))
}

func (wd *WorkflowDetail) setLoading(loading bool) {
	wd.loading = loading
}

// RefreshTheme updates all component colors after a theme change.
func (wd *WorkflowDetail) RefreshTheme() {
	bg := theme.Bg()
	fg := theme.Fg()

	// Update main container
	wd.SetBackgroundColor(bg)

	// Update text views
	wd.workflowView.SetBackgroundColor(bg)
	wd.workflowView.SetTextColor(fg)
	wd.eventDetailView.SetBackgroundColor(bg)
	wd.eventDetailView.SetTextColor(fg)

	// Update table
	wd.eventTable.SetBackgroundColor(bg)

	// Update flex containers
	wd.leftFlex.SetBackgroundColor(bg)

	// Re-render content with new theme colors
	wd.render()
	wd.populateEventTable()
}

func (wd *WorkflowDetail) loadData() {
	provider := wd.app.Provider()
	if provider == nil {
		wd.loadMockData()
		return
	}

	wd.setLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		workflow, err := provider.GetWorkflow(ctx, wd.app.CurrentNamespace(), wd.workflowID, wd.runID)

		wd.app.JigApp().QueueUpdateDraw(func() {
			wd.setLoading(false)
			if err != nil {
				wd.showError(err)
				return
			}
			wd.workflow = workflow
			wd.render()
			// Update hints now that we have workflow status
			wd.app.JigApp().Menu().SetHints(wd.Hints())
		})
	}()

	// Load events in parallel
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		events, err := provider.GetEnhancedWorkflowHistory(ctx, wd.app.CurrentNamespace(), wd.workflowID, wd.runID)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				return
			}
			wd.events = events
			wd.populateEventTable()
		})
	}()
}

func (wd *WorkflowDetail) loadMockData() {
	now := time.Now()
	wd.workflow = &temporal.Workflow{
		ID:        wd.workflowID,
		RunID:     wd.runID,
		Type:      "MockWorkflow",
		Status:    "Running",
		Namespace: wd.app.CurrentNamespace(),
		TaskQueue: "mock-tasks",
		StartTime: now.Add(-5 * time.Minute),
	}
	wd.events = []temporal.EnhancedHistoryEvent{
		{ID: 1, Type: "WorkflowExecutionStarted", Time: now.Add(-5 * time.Minute), Details: "WorkflowType: MockWorkflow, TaskQueue: mock-tasks"},
		{ID: 2, Type: "WorkflowTaskScheduled", Time: now.Add(-5 * time.Minute), Details: "TaskQueue: mock-tasks"},
		{ID: 3, Type: "WorkflowTaskStarted", Time: now.Add(-5 * time.Minute), Details: "Identity: worker-1@host"},
		{ID: 4, Type: "WorkflowTaskCompleted", Time: now.Add(-5 * time.Minute), Details: "ScheduledEventId: 2"},
		{ID: 5, Type: "ActivityTaskScheduled", Time: now.Add(-4 * time.Minute), Details: "ActivityType: MockActivity, TaskQueue: mock-tasks", ActivityType: "MockActivity"},
		{ID: 6, Type: "ActivityTaskStarted", Time: now.Add(-4 * time.Minute), Details: "Identity: worker-1@host, Attempt: 1", ActivityType: "MockActivity", ScheduledEventID: 5},
		{ID: 7, Type: "ActivityTaskCompleted", Time: now.Add(-3 * time.Minute), Details: "ScheduledEventId: 5, Result: {success: true}", ActivityType: "MockActivity", ScheduledEventID: 5},
	}
	wd.render()
	wd.populateEventTable()
}

func (wd *WorkflowDetail) showError(err error) {
	wd.workflowView.SetText(fmt.Sprintf("\n [%s]Error: %s[-]", theme.TagError(), err.Error()))
	wd.eventDetailView.SetText("")
}

func (wd *WorkflowDetail) render() {
	if wd.workflow == nil {
		wd.workflowView.SetText(fmt.Sprintf(" [%s]Workflow not found[-]", theme.TagError()))
		return
	}

	w := wd.workflow
	now := time.Now()
	statusColor := theme.StatusColorTag(w.Status)
	statusIcon := theme.StatusIcon(w.Status)

	durationStr := "In progress"
	if w.EndTime != nil {
		durationStr = w.EndTime.Sub(w.StartTime).Round(time.Second).String()
	} else if w.Status == "Running" {
		durationStr = time.Since(w.StartTime).Round(time.Second).String()
	}

	// Combined workflow info
	workflowText := fmt.Sprintf(`
[%s::b]ID[-:-:-]           [%s]%s[-]
[%s::b]Type[-:-:-]         [%s]%s[-]
[%s::b]Status[-:-:-]       [%s]%s %s[-]
[%s::b]Started[-:-:-]      [%s]%s[-]
[%s::b]Duration[-:-:-]     [%s]%s[-]
[%s::b]Task Queue[-:-:-]   [%s]%s[-]
[%s::b]Run ID[-:-:-]       [%s]%s[-]`,
		theme.TagFgDim(), theme.TagFg(), w.ID,
		theme.TagFgDim(), theme.TagFg(), w.Type,
		theme.TagFgDim(), statusColor, statusIcon, w.Status,
		theme.TagFgDim(), theme.TagFg(), formatRelativeTime(now, w.StartTime),
		theme.TagFgDim(), theme.TagFg(), durationStr,
		theme.TagFgDim(), theme.TagFg(), w.TaskQueue,
		theme.TagFgDim(), theme.TagFgDim(), truncateStr(w.RunID, 25),
	)
	wd.workflowView.SetText(workflowText)
}

func (wd *WorkflowDetail) updateEventDetail(ev temporal.EnhancedHistoryEvent) {
	icon := eventIcon(ev.Type)
	colorTag := eventColorTag(ev.Type)

	// Parse and format the details string
	formattedDetails := formatEventDetails(ev.Details)

	// Build name line if applicable
	var nameLine string
	name := getEventNameDetail(&ev)
	if name != "" {
		nameLine = fmt.Sprintf("\n[%s::b]Name[-:-:-]         [%s]%s[-]", theme.TagFgDim(), theme.TagFg(), name)
	}

	detailText := fmt.Sprintf(`
[%s::b]Event ID[-:-:-]     [%s]%d[-]
[%s::b]Type[-:-:-]         [%s]%s %s[-]%s
[%s::b]Time[-:-:-]         [%s]%s[-]

%s`,
		theme.TagFgDim(), theme.TagFg(), ev.ID,
		theme.TagFgDim(), colorTag, icon, ev.Type, nameLine,
		theme.TagFgDim(), theme.TagFg(), ev.Time.Format("2006-01-02 15:04:05.000"),
		formattedDetails,
	)
	wd.eventDetailView.SetText(detailText)
}

// formatEventDetails parses event details and formats them with pretty JSON.
func formatEventDetails(details string) string {
	if details == "" {
		return fmt.Sprintf("[%s]No details[-]", theme.TagFgDim())
	}

	// First check if the whole thing is JSON
	trimmed := strings.TrimSpace(details)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		formatted := formatJSONPretty(details)
		return highlightFormattedJSONWorkflow(formatted)
	}

	// Handle key-value format with embedded JSON
	return formatKeyValueDetailsWorkflow(details)
}

// formatKeyValueDetailsWorkflow formats key-value style details with embedded JSON.
func formatKeyValueDetailsWorkflow(details string) string {
	var result strings.Builder

	// Split by commas while preserving JSON objects
	parts := splitPreservingJSONWorkflow(details)

	// First pass: find max key length for alignment
	type kvPair struct {
		key   string
		value string
	}
	var pairs []kvPair
	maxKeyLen := 0

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Find the key-value split point (first colon not inside JSON)
		colonIdx := findKeyColonIndex(part)
		if colonIdx > 0 {
			key := strings.TrimSpace(part[:colonIdx])
			value := strings.TrimSpace(part[colonIdx+1:])
			pairs = append(pairs, kvPair{key, value})
			if len(key) > maxKeyLen {
				maxKeyLen = len(key)
			}
		} else {
			pairs = append(pairs, kvPair{"", part})
		}
	}

	// Second pass: format with aligned keys
	for i, kv := range pairs {
		if i > 0 {
			result.WriteString("\n")
		}

		if kv.key != "" {
			// Pad key for alignment
			paddedKey := kv.key + strings.Repeat(" ", maxKeyLen-len(kv.key))

			// Check if value is JSON
			value := strings.TrimSpace(kv.value)
			if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
				formatted := formatJSONPretty(value)
				if formatted != value {
					// JSON was successfully formatted - put it on next line at left margin
					result.WriteString(fmt.Sprintf("[%s::b]%s[-:-:-]\n", theme.TagFgDim(), paddedKey))
					result.WriteString(highlightFormattedJSONWorkflow(formatted))
				} else {
					result.WriteString(fmt.Sprintf("[%s::b]%s[-:-:-]  ", theme.TagFgDim(), paddedKey))
					result.WriteString(highlightJSONLineWorkflow(value))
				}
			} else {
				result.WriteString(fmt.Sprintf("[%s::b]%s[-:-:-]  ", theme.TagFgDim(), paddedKey))
				result.WriteString(fmt.Sprintf("[%s]%s[-]", theme.TagFg(), highlightValuesWorkflow(value)))
			}
		} else {
			result.WriteString(fmt.Sprintf("[%s]%s[-]", theme.TagFg(), kv.value))
		}
	}

	return result.String()
}

// splitPreservingJSONWorkflow splits a string by commas while preserving JSON objects.
func splitPreservingJSONWorkflow(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '{', '[':
			depth++
			current.WriteRune(ch)
		case '}', ']':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// findKeyColonIndex finds the index of the colon that separates key from value.
// It ignores colons inside JSON objects or strings.
func findKeyColonIndex(s string) int {
	depth := 0
	inString := false
	for i, ch := range s {
		switch ch {
		case '"':
			inString = !inString
		case '{', '[':
			if !inString {
				depth++
			}
		case '}', ']':
			if !inString {
				depth--
			}
		case ':':
			if depth == 0 && !inString {
				return i
			}
		}
	}
	return -1
}

// formatJSONPretty attempts to format a string as pretty JSON.
func formatJSONPretty(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return s
	}

	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return s
	}

	return string(pretty)
}

// highlightFormattedJSONWorkflow applies syntax highlighting to formatted JSON.
func highlightFormattedJSONWorkflow(formatted string) string {
	lines := strings.Split(formatted, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, highlightJSONLineWorkflow(line))
	}
	return strings.Join(result, "\n")
}

// highlightJSONLineWorkflow highlights a single line of JSON content.
func highlightJSONLineWorkflow(line string) string {
	// Check for key: value pattern
	if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
		prefix := line[:colonIdx]
		suffix := line[colonIdx+1:]

		trimmed := strings.TrimSpace(prefix)
		if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
			// JSON key with quotes - use accent color
			return fmt.Sprintf("[%s]%s[-]:[%s]%s[-]", theme.TagAccent(), prefix, theme.TagFg(), highlightValuesWorkflow(suffix))
		}
	}

	return highlightValuesWorkflow(line)
}

// highlightValuesWorkflow highlights JSON values (booleans, null).
func highlightValuesWorkflow(s string) string {
	result := s
	result = strings.ReplaceAll(result, "true", fmt.Sprintf("[%s]true[-]", theme.StatusColorTag("Completed")))
	result = strings.ReplaceAll(result, "false", fmt.Sprintf("[%s]false[-]", theme.StatusColorTag("Failed")))
	result = strings.ReplaceAll(result, "null", fmt.Sprintf("[%s]null[-]", theme.TagFgDim()))
	return result
}

func (wd *WorkflowDetail) populateEventTable() {
	// Preserve current selection
	currentRow := wd.eventTable.SelectedRow()

	wd.eventTable.ClearRows()
	wd.eventTable.SetHeaders("ID", "TIME", "TYPE", "NAME")

	for _, ev := range wd.events {
		icon := eventIcon(ev.Type)
		color := eventColor(ev.Type)
		name := getEventNameDetail(&ev)
		wd.eventTable.AddRowWithColor(color,
			fmt.Sprintf("%d", ev.ID),
			ev.Time.Format("15:04:05"),
			icon+" "+truncateStr(ev.Type, 30),
			name,
		)
	}

	if wd.eventTable.RowCount() > 0 {
		// Restore previous selection if valid, otherwise select first row
		if currentRow >= 0 && currentRow < len(wd.events) {
			wd.eventTable.SelectRow(currentRow)
			wd.updateEventDetail(wd.events[currentRow])
		} else {
			wd.eventTable.SelectRow(0)
			if len(wd.events) > 0 {
				wd.updateEventDetail(wd.events[0])
			}
		}
	}
}

// getEventNameDetail returns the activity type, timer ID, or child workflow type for an event.
func getEventNameDetail(ev *temporal.EnhancedHistoryEvent) string {
	if ev.ActivityType != "" {
		return ev.ActivityType
	}
	if ev.TimerID != "" {
		return "Timer: " + ev.TimerID
	}
	if ev.ChildWorkflowType != "" {
		return ev.ChildWorkflowType
	}
	return ""
}

// Name returns the view name.
func (wd *WorkflowDetail) Name() string {
	return "workflow-detail"
}

// Start is called when the view becomes active.
func (wd *WorkflowDetail) Start() {
	wd.eventTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			wd.loadData()
			return nil
		case 'e':
			// Navigate to event history/graph view
			wd.app.NavigateToEvents(wd.workflowID, wd.runID)
			return nil
		case 'y':
			wd.yankEventData()
			return nil
		case 'd':
			wd.showEventDetailModal()
			return nil
		case 'c':
			wd.showCancelConfirm()
			return nil
		case 'X':
			wd.showTerminateConfirm()
			return nil
		case 's':
			wd.showSignalInput()
			return nil
		case 'D':
			wd.showDeleteConfirm()
			return nil
		case 'R':
			wd.showResetSelector()
			return nil
		case 'Q':
			wd.showQueryInput()
			return nil
		case 'i':
			wd.showIOModal()
			return nil
		}
		return event
	})
	wd.loadData()
}

// Stop is called when the view is deactivated.
func (wd *WorkflowDetail) Stop() {
	wd.eventTable.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (wd *WorkflowDetail) Hints() []KeyHint {
	hints := []KeyHint{
		{Key: "i", Description: "Input/Output"},
		{Key: "e", Description: "Event Graph"},
		{Key: "d", Description: "Detail"},
		{Key: "y", Description: "Yank"},
		{Key: "r", Description: "Refresh"},
		{Key: "j/k", Description: "Navigate"},
	}

	// Only show mutation hints if workflow is running
	if wd.workflow != nil && wd.workflow.Status == "Running" {
		hints = append(hints,
			KeyHint{Key: "c", Description: "Cancel"},
			KeyHint{Key: "X", Description: "Terminate"},
			KeyHint{Key: "s", Description: "Signal"},
			KeyHint{Key: "Q", Description: "Query"},
		)
	}

	// Reset is available for completed/failed workflows
	if wd.workflow != nil && (wd.workflow.Status == "Completed" || wd.workflow.Status == "Failed" || wd.workflow.Status == "Terminated" || wd.workflow.Status == "Canceled") {
		hints = append(hints, KeyHint{Key: "R", Description: "Reset"})
	}

	hints = append(hints,
		KeyHint{Key: "D", Description: "Delete"},
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "esc", Description: "Back"},
	)

	return hints
}

// Focus sets focus to the event table.
func (wd *WorkflowDetail) Focus(delegate func(p tview.Primitive)) {
	delegate(wd.eventTable)
}

// Draw applies theme colors dynamically and draws the view.
func (wd *WorkflowDetail) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	wd.SetBackgroundColor(bg)
	wd.leftFlex.SetBackgroundColor(bg)
	wd.workflowView.SetBackgroundColor(bg)
	wd.eventDetailView.SetBackgroundColor(bg)
	wd.Flex.Draw(screen)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Mutation methods

func (wd *WorkflowDetail) showCancelConfirm() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Cancel Workflow", theme.IconWarning),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("reason", "Reason (optional)", "Cancelled via tempo")
	form.SetOnSubmit(func(values map[string]any) {
		reason := values["reason"].(string)
		wd.closeModal()
		wd.executeCancelWorkflow(reason)
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Confirm"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		reason := values["reason"].(string)
		wd.closeModal()
		wd.executeCancelWorkflow(reason)
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeCancelWorkflow(reason string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.CancelWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
			reason,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showError(err)
				return
			}
			wd.loadData() // Refresh to show updated status
		})
	}()
}

func (wd *WorkflowDetail) showTerminateConfirm() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Terminate Workflow", theme.IconError),
		Width:    65,
		Height:   14,
		Backdrop: true,
	})

	// Create content with warning message
	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	warningText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf("[%s]Warning: Termination is immediate and irreversible.\nNo cleanup code will run in the workflow.[-]", theme.TagError()))

	form := components.NewForm()
	form.AddTextField("reason", "Reason (required)", "Terminated via tempo")
	form.SetOnSubmit(func(values map[string]any) {
		reason := values["reason"].(string)
		if reason == "" {
			return // Require a reason
		}
		wd.closeModal()
		wd.executeTerminateWorkflow(reason)
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	contentFlex.AddItem(warningText, 3, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Terminate"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		reason := values["reason"].(string)
		if reason == "" {
			return
		}
		wd.closeModal()
		wd.executeTerminateWorkflow(reason)
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeTerminateWorkflow(reason string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.TerminateWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
			reason,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showError(err)
				return
			}
			wd.loadData() // Refresh to show updated status
		})
	}()
}

func (wd *WorkflowDetail) showDeleteConfirm() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Delete Workflow", theme.IconError),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	// Create content with warning message
	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	warningText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf(`[%s]Warning: This will permanently delete the workflow and its history.
This action cannot be undone.[-]

[%s]Workflow ID:[-] [%s]%s[-]`,
		theme.TagError(),
		theme.TagFgDim(), theme.TagFg(), wd.workflowID))

	form := components.NewForm()
	form.AddTextField("confirm", "Type workflow ID to confirm", "")
	form.SetOnSubmit(func(values map[string]any) {
		confirm := values["confirm"].(string)
		if confirm != wd.workflowID {
			return // Must match workflow ID
		}
		wd.closeModal()
		wd.executeDeleteWorkflow()
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	contentFlex.AddItem(warningText, 5, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Delete"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		confirm := values["confirm"].(string)
		if confirm != wd.workflowID {
			return
		}
		wd.closeModal()
		wd.executeDeleteWorkflow()
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeDeleteWorkflow() {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.DeleteWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showError(err)
				return
			}
			// Navigate back to workflow list after deletion
			wd.app.JigApp().Pages().Pop()
		})
	}()
}

func (wd *WorkflowDetail) showSignalInput() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Signal Workflow", theme.IconSignal),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("signalName", "Signal Name", "")
	form.AddTextField("input", "Input (JSON, optional)", "")
	form.SetOnSubmit(func(values map[string]any) {
		signalName := values["signalName"].(string)
		if signalName == "" {
			return // Require signal name
		}
		input := values["input"].(string)
		wd.closeModal()
		wd.executeSignalWorkflow(signalName, input)
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Send signal"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		signalName := values["signalName"].(string)
		if signalName == "" {
			return
		}
		input := values["input"].(string)
		wd.closeModal()
		wd.executeSignalWorkflow(signalName, input)
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeSignalWorkflow(signalName, input string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var inputBytes []byte
		if input != "" {
			inputBytes = []byte(input)
		}

		err := provider.SignalWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
			signalName,
			inputBytes,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showError(err)
				return
			}
			wd.loadData() // Refresh to show signal event
		})
	}()
}

func (wd *WorkflowDetail) showResetSelector() {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	// Show loading modal
	loadingModal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Loading Reset Points...", theme.IconInfo),
		Width:    40,
		Height:   5,
		Backdrop: true,
	})
	loadingText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	loadingText.SetBackgroundColor(theme.Bg())
	loadingText.SetText(fmt.Sprintf("[%s]Fetching reset points...[-]", theme.TagFgDim()))
	loadingModal.SetContent(loadingText)
	wd.app.JigApp().Pages().Push(loadingModal)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resetPoints, err := provider.GetResetPoints(ctx, wd.app.CurrentNamespace(), wd.workflowID, wd.runID)

		wd.app.JigApp().QueueUpdateDraw(func() {
			wd.closeModal()

			if err != nil {
				wd.showError(err)
				return
			}

			if len(resetPoints) == 0 {
				wd.showResetError("No valid reset points found for this workflow.")
				return
			}

			// Show the reset picker with all points
			wd.showResetPicker(resetPoints)
		})
	}()
}

func (wd *WorkflowDetail) showQuickResetModal(failurePoint temporal.ResetPoint, allPoints []temporal.ResetPoint) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Quick Reset", theme.IconWarning),
		Width:    70,
		Height:   14,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf(`[%s]Reset to failure point:[-]

[%s]Event ID:[-]    [%s]%d[-]
[%s]Type:[-]        [%s]%s[-]
[%s]Description:[-] [%s]%s[-]`,
		theme.TagAccent(),
		theme.TagFgDim(), theme.TagFg(), failurePoint.EventID,
		theme.TagFgDim(), theme.TagFg(), failurePoint.EventType,
		theme.TagFgDim(), theme.TagFg(), failurePoint.Description))

	form := components.NewForm()
	form.AddTextField("reason", "Reason", "Reset via tempo")
	form.SetOnSubmit(func(values map[string]any) {
		wd.closeModal()
		wd.executeResetWorkflow(failurePoint.EventID, values["reason"].(string))
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	contentFlex.AddItem(infoText, 6, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Reset"},
		{Key: "p", Description: "Pick another"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) showResetPicker(resetPoints []temporal.ResetPoint) {
	modal := components.NewModal(components.ModalConfig{
		Title:     fmt.Sprintf("%s Select Reset Point", theme.IconInfo),
		Width:     90,
		Height:    20,
		MinHeight: 15,
		Backdrop:  true,
	})

	// Create a table for reset points
	table := components.NewTable()
	table.SetHeaders("EVENT ID", "TYPE", "TIME", "DESCRIPTION")
	table.SetBackgroundColor(theme.Bg())

	for _, rp := range resetPoints {
		table.AddRow(
			fmt.Sprintf("%d", rp.EventID),
			truncateStr(rp.EventType, 25),
			rp.Timestamp.Format("15:04:05"),
			truncateStr(rp.Description, 35),
		)
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row := table.SelectedRow()
			if row >= 0 && row < len(resetPoints) {
				wd.closeModal()
				wd.showResetConfirm(resetPoints[row])
			}
			return nil
		case tcell.KeyEscape:
			wd.closeModal()
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				wd.closeModal()
				return nil
			}
		}
		return event
	})

	modal.SetContent(table)
	modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Navigate"},
		{Key: "Enter", Description: "Select"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(table)
}

func (wd *WorkflowDetail) showResetConfirm(resetPoint temporal.ResetPoint) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Confirm Reset", theme.IconWarning),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf(`[%s]Reset workflow to event:[-]

[%s]Event ID:[-]    [%s]%d[-]
[%s]Type:[-]        [%s]%s[-]
[%s]Time:[-]        [%s]%s[-]
[%s]Description:[-] [%s]%s[-]`,
		theme.TagAccent(),
		theme.TagFgDim(), theme.TagFg(), resetPoint.EventID,
		theme.TagFgDim(), theme.TagFg(), resetPoint.EventType,
		theme.TagFgDim(), theme.TagFg(), resetPoint.Timestamp.Format("2006-01-02 15:04:05"),
		theme.TagFgDim(), theme.TagFg(), resetPoint.Description))

	form := components.NewForm()
	form.AddTextField("reason", "Reason", "Reset via tempo")
	form.SetOnSubmit(func(values map[string]any) {
		wd.closeModal()
		wd.executeResetWorkflow(resetPoint.EventID, values["reason"].(string))
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	contentFlex.AddItem(infoText, 7, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Reset"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		wd.closeModal()
		wd.executeResetWorkflow(resetPoint.EventID, values["reason"].(string))
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeResetWorkflow(eventID int64, reason string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		newRunID, err := provider.ResetWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
			eventID,
			reason,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showError(err)
				return
			}
			// Update to the new run ID and reload
			wd.runID = newRunID
			wd.loadData()
		})
	}()
}

func (wd *WorkflowDetail) showResetError(message string) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Reset Error", theme.IconError),
		Width:    50,
		Height:   8,
		Backdrop: true,
	})

	errorText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	errorText.SetBackgroundColor(theme.Bg())
	errorText.SetText(fmt.Sprintf("[%s]%s[-]", theme.TagError(), message))

	modal.SetContent(errorText)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter/Esc", Description: "Close"},
	})
	modal.SetOnSubmit(func() {
		wd.closeModal()
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
}

func (wd *WorkflowDetail) closeModal() {
	wd.app.JigApp().Pages().DismissModal()
}

func (wd *WorkflowDetail) showQueryInput() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query Workflow", theme.IconInfo),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddSelect("queryType", "Query Type", []string{"__stack_trace", "custom"})
	form.AddTextField("customQuery", "Custom Query Name", "")
	form.AddTextField("args", "Arguments (JSON, optional)", "")

	form.SetOnSubmit(func(values map[string]any) {
		queryType := values["queryType"].(string)
		if queryType == "custom" {
			queryType = values["customQuery"].(string)
		}
		if queryType == "" {
			return
		}
		args := values["args"].(string)
		wd.closeModal()
		wd.executeQuery(queryType, args)
	})
	form.SetOnCancel(func() {
		wd.closeModal()
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Execute query"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		queryType := values["queryType"].(string)
		if queryType == "custom" {
			queryType = values["customQuery"].(string)
		}
		if queryType == "" {
			return
		}
		args := values["args"].(string)
		wd.closeModal()
		wd.executeQuery(queryType, args)
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDetail) executeQuery(queryType, args string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var argsBytes []byte
		if args != "" {
			argsBytes = []byte(args)
		}

		result, err := provider.QueryWorkflow(
			ctx,
			wd.app.CurrentNamespace(),
			wd.workflowID,
			wd.runID,
			queryType,
			argsBytes,
		)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				wd.showQueryError(queryType, err.Error())
				return
			}
			wd.showQueryResult(queryType, result.Result)
		})
	}()
}

func (wd *WorkflowDetail) showQueryResult(queryType, result string) {
	modal := components.NewModal(components.ModalConfig{
		Title:     fmt.Sprintf("%s Query Result: %s", theme.IconInfo, queryType),
		Width:     0,
		Height:    0,
		MinWidth:  80,
		MinHeight: 20,
		Backdrop:  true,
	})

	// Create scrollable text view for result
	resultView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	resultView.SetBackgroundColor(theme.Bg())
	resultView.SetTextColor(theme.Fg())

	// Format the result (attempt to pretty-print JSON)
	formatted := formatJSONPretty(result)
	highlighted := highlightFormattedJSONWorkflow(formatted)
	resultView.SetText(highlighted)

	panel := components.NewPanel().SetTitle("Result")
	panel.SetContent(resultView)

	resultView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			wd.closeModal()
			return nil
		case tcell.KeyDown:
			row, col := resultView.GetScrollOffset()
			resultView.ScrollTo(row+1, col)
			return nil
		case tcell.KeyUp:
			row, col := resultView.GetScrollOffset()
			if row > 0 {
				resultView.ScrollTo(row-1, col)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, col := resultView.GetScrollOffset()
				resultView.ScrollTo(row+1, col)
				return nil
			case 'k':
				row, col := resultView.GetScrollOffset()
				if row > 0 {
					resultView.ScrollTo(row-1, col)
				}
				return nil
			case 'g':
				resultView.ScrollTo(0, 0)
				return nil
			case 'G':
				resultView.ScrollToEnd()
				return nil
			case 'y':
				copyToClipboard(result)
				// Show "Copied!" feedback
				panel.SetTitle(fmt.Sprintf("%s Copied!", theme.IconCompleted))
				panel.SetTitleColor(theme.StatusColor("Completed"))
				go func() {
					time.Sleep(1 * time.Second)
					wd.app.JigApp().QueueUpdateDraw(func() {
						panel.SetTitle("Result")
						panel.SetTitleColor(0)
					})
				}()
				return nil
			case 'q':
				wd.closeModal()
				return nil
			}
		}
		return event
	})

	modal.SetContent(panel)
	modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Scroll"},
		{Key: "y", Description: "Copy"},
		{Key: "Esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(resultView)
}

func (wd *WorkflowDetail) showQueryError(queryType, errMsg string) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query Failed: %s", theme.IconError, queryType),
		Width:    60,
		Height:   10,
		Backdrop: true,
	})

	errorText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	errorText.SetBackgroundColor(theme.Bg())
	errorText.SetText(fmt.Sprintf("[%s]Error executing query:[-]\n\n[%s]%s[-]",
		theme.TagError(), theme.TagFg(), errMsg))

	modal.SetContent(errorText)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter/Esc", Description: "Close"},
	})
	modal.SetOnSubmit(func() {
		wd.closeModal()
	})
	modal.SetOnCancel(func() {
		wd.closeModal()
	})

	wd.app.JigApp().Pages().Push(modal)
}

// getSelectedEventDetails returns the details for the currently selected event.
func (wd *WorkflowDetail) getSelectedEventDetails() (string, string) {
	row := wd.eventTable.SelectedRow()
	if row < 0 || row >= len(wd.events) {
		return "", ""
	}
	ev := wd.events[row]
	return ev.Type, prettyPrintJSONDetail(ev.Details)
}

// yankEventData copies the selected event's details to clipboard.
func (wd *WorkflowDetail) yankEventData() {
	eventType, data := wd.getSelectedEventDetails()
	if data == "" {
		return
	}

	if err := copyToClipboard(data); err != nil {
		wd.eventDetailView.SetText(fmt.Sprintf("[%s]%s Failed to copy: %s[-]",
			theme.TagError(), theme.IconError, err.Error()))
		return
	}

	// Show success feedback
	wd.eventDetailView.SetText(fmt.Sprintf(`
[%s::b]Copied to clipboard[-:-:-]

[%s]%s[-]

[%s]%s[-]`,
		theme.TagAccent(),
		theme.TagAccent(), eventType,
		theme.StatusColorTag("Completed"), "Event data copied!"))

	// Restore detail after a brief delay
	go func() {
		time.Sleep(1500 * time.Millisecond)
		wd.app.JigApp().QueueUpdateDraw(func() {
			row := wd.eventTable.SelectedRow()
			if row >= 0 && row < len(wd.events) {
				wd.updateEventDetail(wd.events[row])
			}
		})
	}()
}

// showEventDetailModal shows a full-screen modal with the event details.
func (wd *WorkflowDetail) showEventDetailModal() {
	row := wd.eventTable.SelectedRow()
	if row < 0 || row >= len(wd.events) {
		return
	}

	ev := wd.events[row]

	// Create modal
	modal := components.NewModal(components.ModalConfig{
		Title:     fmt.Sprintf("%s Event: %s", theme.IconEvent, truncateEventTypeStr(ev.Type)),
		Width:     0,
		Height:    0,
		MinWidth:  100,
		MinHeight: 30,
	})

	// Create scrollable text view for details
	detailView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	detailView.SetBackgroundColor(theme.Bg())
	detailView.SetTextColor(theme.Fg())

	// Format the event details
	icon := eventIcon(ev.Type)
	colorTag := eventColorTag(ev.Type)

	headerText := fmt.Sprintf(`[%s::b]Event ID[-:-:-]     [%s]%d[-]
[%s::b]Type[-:-:-]         [%s]%s %s[-]
[%s::b]Time[-:-:-]         [%s]%s[-]

[%s::b]Details[-:-:-]`,
		theme.TagFgDim(), theme.TagFg(), ev.ID,
		theme.TagFgDim(), colorTag, icon, ev.Type,
		theme.TagFgDim(), theme.TagFg(), ev.Time.Format("2006-01-02 15:04:05.000"),
		theme.TagAccent(),
	)

	// Format the details with syntax highlighting
	formattedDetails := formatEventDetails(ev.Details)
	fullText := headerText + "\n" + formattedDetails

	detailView.SetText(fullText)

	// Create panel
	panel := components.NewPanel().SetTitle(fmt.Sprintf("%s Details", theme.IconInfo))
	panel.SetContent(detailView)

	modal.SetContent(panel)
	modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Scroll"},
		{Key: "g/G", Description: "Top/Bottom"},
		{Key: "y", Description: "Copy"},
		{Key: "esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		wd.closeEventDetailModal()
	})

	// Handle input
	detailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			wd.closeEventDetailModal()
			return nil
		case tcell.KeyDown:
			row, col := detailView.GetScrollOffset()
			detailView.ScrollTo(row+1, col)
			return nil
		case tcell.KeyUp:
			row, col := detailView.GetScrollOffset()
			if row > 0 {
				detailView.ScrollTo(row-1, col)
			}
			return nil
		case tcell.KeyPgDn:
			row, col := detailView.GetScrollOffset()
			detailView.ScrollTo(row+10, col)
			return nil
		case tcell.KeyPgUp:
			row, col := detailView.GetScrollOffset()
			if row > 10 {
				detailView.ScrollTo(row-10, col)
			} else {
				detailView.ScrollTo(0, col)
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, col := detailView.GetScrollOffset()
				detailView.ScrollTo(row+1, col)
				return nil
			case 'k':
				row, col := detailView.GetScrollOffset()
				if row > 0 {
					detailView.ScrollTo(row-1, col)
				}
				return nil
			case 'g':
				detailView.ScrollTo(0, 0)
				return nil
			case 'G':
				detailView.ScrollToEnd()
				return nil
			case 'y':
				// Copy the raw details
				if ev.Details != "" {
					copyToClipboard(prettyPrintJSONDetail(ev.Details))
					// Show "Copied!" feedback
					panel.SetTitle(fmt.Sprintf("%s Copied!", theme.IconCompleted))
					panel.SetTitleColor(theme.StatusColor("Completed"))
					go func() {
						time.Sleep(1 * time.Second)
						wd.app.JigApp().QueueUpdateDraw(func() {
							panel.SetTitle(fmt.Sprintf("%s Details", theme.IconInfo))
							panel.SetTitleColor(0)
						})
					}()
				}
				return nil
			case 'q':
				wd.closeEventDetailModal()
				return nil
			}
		}
		return event
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(detailView)
}

// closeEventDetailModal closes the event detail modal.
func (wd *WorkflowDetail) closeEventDetailModal() {
	wd.app.JigApp().Pages().DismissModal()
	wd.app.JigApp().SetFocus(wd.eventTable)
}

// truncateEventTypeStr shortens long event type names for the title.
func truncateEventTypeStr(eventType string) string {
	if len(eventType) > 30 {
		return eventType[:27] + "..."
	}
	return eventType
}

// prettyPrintJSONDetail attempts to format JSON in the details string.
func prettyPrintJSONDetail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Try to parse the whole thing as JSON first
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			pretty, err := json.MarshalIndent(parsed, "", "  ")
			if err == nil {
				return string(pretty)
			}
		}
	}

	// Otherwise, try to find and format JSON embedded in the string
	// Look for patterns like "Result: {...}" or "Input: {...}"
	var result strings.Builder
	parts := strings.Split(s, ", ")
	for i, part := range parts {
		if i > 0 {
			result.WriteString("\n")
		}

		// Check if this part has embedded JSON
		if colonIdx := strings.Index(part, ": "); colonIdx > 0 {
			key := part[:colonIdx]
			value := part[colonIdx+2:]

			// Try to parse and pretty-print the value as JSON
			if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
				var parsed interface{}
				if err := json.Unmarshal([]byte(value), &parsed); err == nil {
					pretty, err := json.MarshalIndent(parsed, "", "  ")
					if err == nil {
						result.WriteString(fmt.Sprintf("%s:\n%s", key, string(pretty)))
						continue
					}
				}
			}
			result.WriteString(fmt.Sprintf("%s: %s", key, value))
		} else {
			result.WriteString(part)
		}
	}

	return result.String()
}

// formatDetailViewWithHighlighting adds color tags for syntax highlighting.
func formatDetailViewWithHighlighting(data string) string {
	lines := strings.Split(data, "\n")
	var result []string

	for _, line := range lines {
		highlighted := highlightDetailLine(line)
		result = append(result, highlighted)
	}

	return strings.Join(result, "\n")
}

// highlightDetailLine adds tview color tags to a single line.
func highlightDetailLine(line string) string {
	// If line contains a colon that looks like a JSON key
	if idx := strings.Index(line, ":"); idx > 0 {
		prefix := line[:idx]
		suffix := line[idx:]

		trimmed := strings.TrimSpace(prefix)
		if strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "'") {
			return fmt.Sprintf("[%s]%s[-]%s", theme.TagAccent(), prefix, highlightDetailValue(suffix))
		} else if !strings.Contains(trimmed, " ") && len(trimmed) > 0 {
			return fmt.Sprintf("[%s::b]%s[-:-:-]%s", theme.TagAccent(), prefix, highlightDetailValue(suffix))
		}
	}

	return highlightDetailValue(line)
}

// highlightDetailValue highlights JSON values.
func highlightDetailValue(s string) string {
	result := s
	result = strings.ReplaceAll(result, "true", fmt.Sprintf("[%s]true[-]", theme.StatusColorTag("Completed")))
	result = strings.ReplaceAll(result, "false", fmt.Sprintf("[%s]false[-]", theme.StatusColorTag("Failed")))
	result = strings.ReplaceAll(result, "null", fmt.Sprintf("[%s]null[-]", theme.TagFgDim()))
	return result
}

// showIOModal displays a modal with workflow input and output side by side.
func (wd *WorkflowDetail) showIOModal() {
	if wd.workflow == nil {
		return
	}

	// Create modal - use percentage-based sizing for larger display
	modal := components.NewModal(components.ModalConfig{
		Title:     fmt.Sprintf("%s Input/Output: %s", theme.IconWorkflow, truncateStr(wd.workflow.Type, 30)),
		Width:     0,  // 0 means use percentage
		Height:    0,
		MinWidth:  120,
		MinHeight: 35,
	})

	// Create two side-by-side text views for input and output
	inputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	inputView.SetBackgroundColor(theme.Bg())
	inputView.SetTextColor(theme.Fg())

	outputView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	outputView.SetBackgroundColor(theme.Bg())
	outputView.SetTextColor(theme.Fg())

	// Format input
	inputText := formatIOContent("Input", wd.workflow.Input)
	inputView.SetText(inputText)

	// Format output
	outputText := formatIOContent("Output", wd.workflow.Output)
	outputView.SetText(outputText)

	// Create panels for each side with visual indicator for focus
	inputPanel := components.NewPanel().SetTitle(fmt.Sprintf("%s Input", theme.IconArrowRight))
	inputPanel.SetContent(inputView)

	outputPanel := components.NewPanel().SetTitle(fmt.Sprintf("%s Output", theme.IconArrowLeft))
	outputPanel.SetContent(outputView)

	// Layout: side by side
	flex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(inputPanel, 0, 1, true).
		AddItem(outputPanel, 0, 1, false)
	flex.SetBackgroundColor(theme.Bg())

	modal.SetContent(flex)
	modal.SetHints([]components.KeyHint{
		{Key: "tab/h/l", Description: "Switch"},
		{Key: "j/k", Description: "Scroll"},
		{Key: "y", Description: "Copy"},
		{Key: "esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		wd.closeIOModal()
	})

	// Track which pane is focused and store references for the handler
	focusedInput := true

	// Update panel titles and colors to show focus
	updatePanelTitles := func() {
		if focusedInput {
			inputPanel.SetTitle(fmt.Sprintf("%s Input (active)", theme.IconArrowRight))
			inputPanel.SetTitleColor(theme.Accent())
			outputPanel.SetTitle(fmt.Sprintf("%s Output", theme.IconArrowLeft))
			outputPanel.SetTitleColor(0) // Use default (PanelTitle color)
		} else {
			inputPanel.SetTitle(fmt.Sprintf("%s Input", theme.IconArrowRight))
			inputPanel.SetTitleColor(0) // Use default
			outputPanel.SetTitle(fmt.Sprintf("%s Output (active)", theme.IconArrowLeft))
			outputPanel.SetTitleColor(theme.Accent())
		}
	}
	updatePanelTitles()

	// Switch focus helper
	switchFocus := func() {
		focusedInput = !focusedInput
		updatePanelTitles()
		if focusedInput {
			wd.app.JigApp().SetFocus(inputView)
		} else {
			wd.app.JigApp().SetFocus(outputView)
		}
	}

	// Scroll helper
	scrollView := func(delta int) {
		var view *tview.TextView
		if focusedInput {
			view = inputView
		} else {
			view = outputView
		}
		row, col := view.GetScrollOffset()
		newRow := row + delta
		if newRow < 0 {
			newRow = 0
		}
		view.ScrollTo(newRow, col)
	}

	// Handle input - shared handler for both views
	inputHandler := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			wd.closeIOModal()
			return nil
		case tcell.KeyTab, tcell.KeyBacktab:
			switchFocus()
			return nil
		case tcell.KeyDown:
			scrollView(1)
			return nil
		case tcell.KeyUp:
			scrollView(-1)
			return nil
		case tcell.KeyPgDn:
			scrollView(10)
			return nil
		case tcell.KeyPgUp:
			scrollView(-10)
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h':
				if !focusedInput {
					switchFocus()
				}
				return nil
			case 'l':
				if focusedInput {
					switchFocus()
				}
				return nil
			case 'j':
				scrollView(1)
				return nil
			case 'k':
				scrollView(-1)
				return nil
			case 'g':
				// Go to top
				if focusedInput {
					inputView.ScrollTo(0, 0)
				} else {
					outputView.ScrollTo(0, 0)
				}
				return nil
			case 'G':
				// Go to bottom - scroll to a large number
				if focusedInput {
					inputView.ScrollToEnd()
				} else {
					outputView.ScrollToEnd()
				}
				return nil
			case 'y':
				// Copy the content of the focused pane
				var content string
				var panel *components.Panel
				if focusedInput {
					content = wd.workflow.Input
					panel = inputPanel
				} else {
					content = wd.workflow.Output
					panel = outputPanel
				}
				if content != "" {
					copyToClipboard(content)
					// Show "Copied!" feedback
					panel.SetTitle(fmt.Sprintf("%s Copied!", theme.IconCompleted))
					panel.SetTitleColor(theme.StatusColor("Completed"))
					go func() {
						time.Sleep(1 * time.Second)
						wd.app.JigApp().QueueUpdateDraw(func() {
							updatePanelTitles()
						})
					}()
				}
				return nil
			case 'q':
				wd.closeIOModal()
				return nil
			}
		}
		return event
	}

	inputView.SetInputCapture(inputHandler)
	outputView.SetInputCapture(inputHandler)

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(inputView)
}

// formatIOContent formats input or output content for display.
func formatIOContent(label, content string) string {
	if content == "" {
		return fmt.Sprintf("[%s]No %s[-]", theme.TagFgDim(), strings.ToLower(label))
	}

	// Pretty print if it's JSON
	formatted := formatJSONPretty(content)
	highlighted := highlightFormattedJSONWorkflow(formatted)

	return highlighted
}

// closeIOModal closes the IO modal.
func (wd *WorkflowDetail) closeIOModal() {
	wd.app.JigApp().Pages().DismissModal()
	wd.app.JigApp().SetFocus(wd.eventTable)
}
