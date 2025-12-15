package view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atterpac/temportui/internal/temporal"
	"github.com/atterpac/temportui/internal/ui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowDetail displays detailed information about a workflow with events.
type WorkflowDetail struct {
	*tview.Flex
	app             *App
	workflowID      string
	runID           string
	workflow        *temporal.Workflow
	events          []temporal.HistoryEvent
	leftFlex        *tview.Flex
	workflowPanel   *ui.Panel
	eventDetailPanel *ui.Panel
	eventsPanel     *ui.Panel
	workflowView    *tview.TextView
	eventDetailView *tview.TextView
	eventTable      *ui.Table
	loading         bool
}

// NewWorkflowDetail creates a new workflow detail view.
func NewWorkflowDetail(app *App, workflowID, runID string) *WorkflowDetail {
	wd := &WorkflowDetail{
		Flex:       tview.NewFlex().SetDirection(tview.FlexColumn),
		app:        app,
		workflowID: workflowID,
		runID:      runID,
		eventTable: ui.NewTable(),
	}
	wd.setup()
	return wd
}

func (wd *WorkflowDetail) setup() {
	wd.SetBackgroundColor(ui.ColorBg)

	// Combined workflow info view
	wd.workflowView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	wd.workflowView.SetBackgroundColor(ui.ColorBg)

	// Event detail view
	wd.eventDetailView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	wd.eventDetailView.SetBackgroundColor(ui.ColorBg)

	// Event table
	wd.eventTable.SetHeaders("ID", "TIME", "TYPE")
	wd.eventTable.SetBorder(false)
	wd.eventTable.SetBackgroundColor(ui.ColorBg)

	// Create panels
	wd.workflowPanel = ui.NewPanel("Workflow")
	wd.workflowPanel.SetContent(wd.workflowView)

	wd.eventDetailPanel = ui.NewPanel("Event Detail")
	wd.eventDetailPanel.SetContent(wd.eventDetailView)

	wd.eventsPanel = ui.NewPanel("Events")
	wd.eventsPanel.SetContent(wd.eventTable)

	// Left side: workflow info + event detail stacked
	wd.leftFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	wd.leftFlex.SetBackgroundColor(ui.ColorBg)
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
	wd.workflowView.SetText(fmt.Sprintf("\n [%s]Loading...[-]", ui.TagFgDim))
}

func (wd *WorkflowDetail) setLoading(loading bool) {
	wd.loading = loading
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

		wd.app.UI().QueueUpdateDraw(func() {
			wd.setLoading(false)
			if err != nil {
				wd.showError(err)
				return
			}
			wd.workflow = workflow
			wd.render()
		})
	}()

	// Load events in parallel
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		events, err := provider.GetWorkflowHistory(ctx, wd.app.CurrentNamespace(), wd.workflowID, wd.runID)

		wd.app.UI().QueueUpdateDraw(func() {
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
	wd.events = []temporal.HistoryEvent{
		{ID: 1, Type: "WorkflowExecutionStarted", Time: now.Add(-5 * time.Minute), Details: "WorkflowType: MockWorkflow, TaskQueue: mock-tasks"},
		{ID: 2, Type: "WorkflowTaskScheduled", Time: now.Add(-5 * time.Minute), Details: "TaskQueue: mock-tasks"},
		{ID: 3, Type: "WorkflowTaskStarted", Time: now.Add(-5 * time.Minute), Details: "Identity: worker-1@host"},
		{ID: 4, Type: "WorkflowTaskCompleted", Time: now.Add(-5 * time.Minute), Details: "ScheduledEventId: 2"},
		{ID: 5, Type: "ActivityTaskScheduled", Time: now.Add(-4 * time.Minute), Details: "ActivityType: MockActivity, TaskQueue: mock-tasks"},
		{ID: 6, Type: "ActivityTaskStarted", Time: now.Add(-4 * time.Minute), Details: "Identity: worker-1@host, Attempt: 1"},
		{ID: 7, Type: "ActivityTaskCompleted", Time: now.Add(-3 * time.Minute), Details: "ScheduledEventId: 5, Result: {success: true}"},
	}
	wd.render()
	wd.populateEventTable()
}

func (wd *WorkflowDetail) showError(err error) {
	wd.workflowView.SetText(fmt.Sprintf("\n [%s]Error: %s[-]", ui.TagFailed, err.Error()))
	wd.eventDetailView.SetText("")
}

func (wd *WorkflowDetail) render() {
	if wd.workflow == nil {
		wd.workflowView.SetText(fmt.Sprintf(" [%s]Workflow not found[-]", ui.TagFailed))
		return
	}

	w := wd.workflow
	now := time.Now()
	statusColor := ui.StatusColorTag(w.Status)
	statusIcon := ui.StatusIcon(w.Status)

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
		ui.TagFgDim, ui.TagFg, w.ID,
		ui.TagFgDim, ui.TagFg, w.Type,
		ui.TagFgDim, statusColor, statusIcon, w.Status,
		ui.TagFgDim, ui.TagFg, formatRelativeTime(now, w.StartTime),
		ui.TagFgDim, ui.TagFg, durationStr,
		ui.TagFgDim, ui.TagFg, w.TaskQueue,
		ui.TagFgDim, ui.TagFgDim, truncateStr(w.RunID, 25),
	)
	wd.workflowView.SetText(workflowText)
}

func (wd *WorkflowDetail) updateEventDetail(ev temporal.HistoryEvent) {
	icon := eventIcon(ev.Type)
	colorTag := eventColorTag(ev.Type)

	// Parse and format the details string
	formattedDetails := formatEventDetails(ev.Details)

	detailText := fmt.Sprintf(`
[%s::b]Event ID[-:-:-]     [%s]%d[-]
[%s::b]Type[-:-:-]         [%s]%s %s[-]
[%s::b]Time[-:-:-]         [%s]%s[-]

%s`,
		ui.TagFgDim, ui.TagFg, ev.ID,
		ui.TagFgDim, colorTag, icon, ev.Type,
		ui.TagFgDim, ui.TagFg, ev.Time.Format("2006-01-02 15:04:05.000"),
		formattedDetails,
	)
	wd.eventDetailView.SetText(detailText)
}

// formatEventDetails parses comma-separated key:value pairs and formats them nicely
func formatEventDetails(details string) string {
	if details == "" {
		return fmt.Sprintf("[%s]No details[-]", ui.TagFgDim)
	}

	var sb strings.Builder

	// Split on ", " to get individual fields
	parts := strings.Split(details, ", ")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Try to split on ": " for key-value pairs
		if idx := strings.Index(part, ": "); idx > 0 {
			key := part[:idx]
			value := part[idx+2:]
			sb.WriteString(fmt.Sprintf("[%s::b]%s:[-:-:-] [%s]%s[-]\n", ui.TagFgDim, key, ui.TagFg, value))
		} else {
			// No colon found, just display as-is
			sb.WriteString(fmt.Sprintf("[%s]%s[-]\n", ui.TagFg, part))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func (wd *WorkflowDetail) populateEventTable() {
	wd.eventTable.ClearRows()
	wd.eventTable.SetHeaders("ID", "TIME", "TYPE")

	for _, ev := range wd.events {
		icon := eventIcon(ev.Type)
		color := eventColor(ev.Type)
		wd.eventTable.AddColoredRow(color,
			fmt.Sprintf("%d", ev.ID),
			ev.Time.Format("15:04:05"),
			icon+" "+truncateStr(ev.Type, 30),
		)
	}

	if wd.eventTable.RowCount() > 0 {
		wd.eventTable.SelectRow(0)
		if len(wd.events) > 0 {
			wd.updateEventDetail(wd.events[0])
		}
	}
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
func (wd *WorkflowDetail) Hints() []ui.KeyHint {
	return []ui.KeyHint{
		{Key: "r", Description: "Refresh"},
		{Key: "j/k", Description: "Navigate"},
		{Key: "esc", Description: "Back"},
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
