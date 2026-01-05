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

// EventViewMode represents the display mode for event history.
type EventViewMode int

const (
	ViewModeList EventViewMode = iota
	ViewModeTree
	ViewModeTimeline
)

// EventHistory displays workflow event history with multiple view modes.
type EventHistory struct {
	*components.MasterDetailView
	app        *App
	workflowID string
	runID      string

	// View mode
	viewMode EventViewMode

	// List view components (original)
	table *components.Table

	// Tree view components
	treeView  *EventTreeView
	treeNodes []*temporal.EventTreeNode

	// Timeline view components
	timelineView *TimelineView

	// Shared components
	sidePanel *tview.TextView

	// Data
	events         []temporal.HistoryEvent
	enhancedEvents []temporal.EnhancedHistoryEvent
	loading        bool
}

// NewEventHistory creates a new event history view.
func NewEventHistory(app *App, workflowID, runID string) *EventHistory {
	eh := &EventHistory{
		app:          app,
		workflowID:   workflowID,
		runID:        runID,
		viewMode:     ViewModeTree, // Default to tree view
		table:        components.NewTable(),
		treeView:     NewEventTreeView(),
		timelineView: NewTimelineView(),
		sidePanel:    tview.NewTextView(),
	}
	eh.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(eh)

	return eh
}

func (eh *EventHistory) setup() {
	// Configure list view table
	eh.table.SetHeaders("ID", "TIME", "TYPE", "NAME", "DETAILS")
	eh.table.SetBorder(false)
	eh.table.SetBackgroundColor(theme.Bg())

	// Configure side panel
	eh.sidePanel.SetDynamicColors(true)
	eh.sidePanel.SetTextAlign(tview.AlignLeft)
	eh.sidePanel.SetBackgroundColor(theme.Bg())

	// Create MasterDetailView - default to tree view
	eh.MasterDetailView = components.NewMasterDetailView().
		SetMasterTitle(fmt.Sprintf("%s Events (Tree)", theme.IconEvent)).
		SetDetailTitle(fmt.Sprintf("%s Details", theme.IconInfo)).
		SetMasterContent(eh.treeView).
		SetDetailContent(eh.sidePanel).
		SetRatio(0.6).
		ConfigureEmpty(theme.IconInfo, "No Event", "Select an event to view details")

	// List view selection handlers
	eh.table.SetSelectionChangedFunc(func(row, col int) {
		if eh.viewMode == ViewModeList && eh.IsDetailVisible() && row > 0 {
			eh.updateSidePanelFromList(row - 1)
		}
	})

	eh.table.SetSelectedFunc(func(row, col int) {
		if row > 0 {
			eh.toggleSidePanel()
			if eh.IsDetailVisible() {
				eh.updateSidePanelFromList(row - 1)
			}
		}
	})

	// Tree view selection handlers
	eh.treeView.SetOnSelectionChanged(func(node *temporal.EventTreeNode) {
		if eh.viewMode == ViewModeTree && eh.IsDetailVisible() {
			eh.updateSidePanelFromTree(node)
		}
	})

	eh.treeView.SetOnSelect(func(node *temporal.EventTreeNode) {
		// Toggle expand/collapse is handled by tree view itself
		// Optionally toggle side panel on enter
	})

	// Timeline view selection handler (Enter key)
	eh.timelineView.SetOnSelect(func(lane *TimelineLane) {
		if lane != nil && lane.Node != nil {
			eh.updateSidePanelFromTree(lane.Node)
		}
	})

	// Timeline view selection change handler (navigation)
	eh.timelineView.SetOnSelectionChange(func(lane *TimelineLane) {
		if lane != nil && lane.Node != nil {
			eh.updateSidePanelFromTree(lane.Node)
		}
	})
}

func (eh *EventHistory) buildLayout() {
	// Update panel title and content based on view mode
	switch eh.viewMode {
	case ViewModeList:
		eh.SetMasterTitle(fmt.Sprintf("%s Events (List)", theme.IconEvent))
		eh.SetMasterContent(eh.table)
	case ViewModeTree:
		eh.SetMasterTitle(fmt.Sprintf("%s Events (Tree)", theme.IconEvent))
		eh.SetMasterContent(eh.treeView)
	case ViewModeTimeline:
		eh.SetMasterTitle(fmt.Sprintf("%s Events (Timeline)", theme.IconEvent))
		eh.SetMasterContent(eh.timelineView)
	}

	// Set focus to the active view component
	if eh.app != nil && eh.app.JigApp() != nil {
		switch eh.viewMode {
		case ViewModeList:
			eh.app.JigApp().SetFocus(eh.table)
		case ViewModeTree:
			eh.app.JigApp().SetFocus(eh.treeView)
		case ViewModeTimeline:
			eh.app.JigApp().SetFocus(eh.timelineView)
		}
	}
}

func (eh *EventHistory) setViewMode(mode EventViewMode) {
	if eh.viewMode == mode {
		return
	}
	eh.viewMode = mode
	eh.buildLayout()
	eh.setupInputCapture()
	eh.refreshCurrentView()
}

func (eh *EventHistory) cycleViewMode() {
	nextMode := (eh.viewMode + 1) % 3
	eh.setViewMode(nextMode)
}

func (eh *EventHistory) refreshCurrentView() {
	switch eh.viewMode {
	case ViewModeList:
		eh.populateTable()
	case ViewModeTree:
		eh.populateTreeView()
	case ViewModeTimeline:
		eh.populateTimelineView()
	}
}

func (eh *EventHistory) setLoading(loading bool) {
	eh.loading = loading
}

// RefreshTheme updates all component colors after a theme change.
func (eh *EventHistory) RefreshTheme() {
	bg := theme.Bg()
	fg := theme.Fg()

	// Update table (list view)
	eh.table.SetBackgroundColor(bg)

	// Update side panel
	eh.sidePanel.SetBackgroundColor(bg)
	eh.sidePanel.SetTextColor(fg)

	// Re-render current view with new theme colors
	eh.refreshCurrentView()
}

func (eh *EventHistory) loadData() {
	provider := eh.app.Provider()
	if provider == nil {
		eh.loadMockData()
		return
	}

	eh.setLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Load enhanced events for tree/timeline views
		enhancedEvents, err := provider.GetEnhancedWorkflowHistory(ctx, eh.app.CurrentNamespace(), eh.workflowID, eh.runID)

		eh.app.JigApp().QueueUpdateDraw(func() {
			eh.setLoading(false)
			if err != nil {
				eh.showError(err)
				return
			}

			eh.enhancedEvents = enhancedEvents

			// Convert to basic events for list view
			eh.events = make([]temporal.HistoryEvent, len(enhancedEvents))
			for i, ev := range enhancedEvents {
				eh.events[i] = temporal.HistoryEvent{
					ID:      ev.ID,
					Type:    ev.Type,
					Time:    ev.Time,
					Details: ev.Details,
				}
			}

			// Build tree nodes
			eh.treeNodes = temporal.BuildEventTree(enhancedEvents)

			// Populate current view
			eh.refreshCurrentView()
		})
	}()
}

func (eh *EventHistory) loadMockData() {
	now := time.Now()

	// Create mock enhanced events
	eh.enhancedEvents = []temporal.EnhancedHistoryEvent{
		{ID: 1, Type: "WorkflowExecutionStarted", Time: now.Add(-5 * time.Minute), Details: "WorkflowType: MockWorkflow, TaskQueue: mock-tasks", TaskQueue: "mock-tasks"},
		{ID: 2, Type: "WorkflowTaskScheduled", Time: now.Add(-5 * time.Minute), Details: "TaskQueue: mock-tasks", TaskQueue: "mock-tasks"},
		{ID: 3, Type: "WorkflowTaskStarted", Time: now.Add(-5 * time.Minute), Details: "Identity: worker-1@host", ScheduledEventID: 2, Identity: "worker-1@host"},
		{ID: 4, Type: "WorkflowTaskCompleted", Time: now.Add(-5 * time.Minute), Details: "ScheduledEventId: 2", ScheduledEventID: 2, StartedEventID: 3},
		{ID: 5, Type: "ActivityTaskScheduled", Time: now.Add(-4 * time.Minute), Details: "ActivityType: ValidateOrder, TaskQueue: mock-tasks", ActivityType: "ValidateOrder", ActivityID: "1", TaskQueue: "mock-tasks"},
		{ID: 6, Type: "ActivityTaskStarted", Time: now.Add(-4 * time.Minute), Details: "Identity: worker-1@host, Attempt: 1", ScheduledEventID: 5, Attempt: 1, Identity: "worker-1@host"},
		{ID: 7, Type: "ActivityTaskCompleted", Time: now.Add(-3 * time.Minute), Details: "ScheduledEventId: 5, Result: {success: true}", ScheduledEventID: 5, StartedEventID: 6, Result: "{success: true}"},
		{ID: 8, Type: "ActivityTaskScheduled", Time: now.Add(-3 * time.Minute), Details: "ActivityType: ProcessPayment, TaskQueue: mock-tasks", ActivityType: "ProcessPayment", ActivityID: "2", TaskQueue: "mock-tasks"},
		{ID: 9, Type: "ActivityTaskStarted", Time: now.Add(-3 * time.Minute), Details: "Identity: worker-1@host, Attempt: 1", ScheduledEventID: 8, Attempt: 1, Identity: "worker-1@host"},
		{ID: 10, Type: "ActivityTaskFailed", Time: now.Add(-2 * time.Minute), Details: "ScheduledEventId: 8, Failure: timeout", ScheduledEventID: 8, StartedEventID: 9, Failure: "timeout"},
		{ID: 11, Type: "ActivityTaskStarted", Time: now.Add(-2 * time.Minute), Details: "Identity: worker-1@host, Attempt: 2", ScheduledEventID: 8, Attempt: 2, Identity: "worker-1@host"},
		{ID: 12, Type: "ActivityTaskCompleted", Time: now.Add(-1 * time.Minute), Details: "ScheduledEventId: 8, Result: {paid: true}", ScheduledEventID: 8, StartedEventID: 11, Result: "{paid: true}"},
		{ID: 13, Type: "TimerStarted", Time: now.Add(-1 * time.Minute), Details: "TimerId: wait-30s", TimerID: "wait-30s"},
		{ID: 14, Type: "TimerFired", Time: now.Add(-30 * time.Second), Details: "TimerId: wait-30s, StartedEventId: 13", TimerID: "wait-30s", StartedEventID: 13},
	}

	// Convert to basic events
	eh.events = make([]temporal.HistoryEvent, len(eh.enhancedEvents))
	for i, ev := range eh.enhancedEvents {
		eh.events[i] = temporal.HistoryEvent{
			ID:      ev.ID,
			Type:    ev.Type,
			Time:    ev.Time,
			Details: ev.Details,
		}
	}

	// Build tree nodes
	eh.treeNodes = temporal.BuildEventTree(eh.enhancedEvents)

	// Populate current view
	eh.refreshCurrentView()
}

func (eh *EventHistory) populateTable() {
	// Preserve current selection
	currentRow := eh.table.SelectedRow()

	eh.table.ClearRows()
	eh.table.SetHeaders("ID", "TIME", "TYPE", "NAME", "DETAILS")

	for _, ev := range eh.enhancedEvents {
		icon := eventIcon(ev.Type)
		color := eventColor(ev.Type)
		name := getEventName(&ev)
		eh.table.AddRowWithColor(color,
			fmt.Sprintf("%d", ev.ID),
			ev.Time.Format("15:04:05"),
			icon+" "+ev.Type,
			name,
			truncate(ev.Details, 40),
		)
	}

	if eh.table.RowCount() > 0 {
		// Restore previous selection if valid, otherwise select first row
		if currentRow >= 0 && currentRow < len(eh.enhancedEvents) {
			eh.table.SelectRow(currentRow)
			eh.updateSidePanelFromList(currentRow)
		} else {
			eh.table.SelectRow(0)
			if len(eh.enhancedEvents) > 0 {
				eh.updateSidePanelFromList(0)
			}
		}
	}
}

// getEventName returns the activity type, timer ID, or child workflow type for an event.
func getEventName(ev *temporal.EnhancedHistoryEvent) string {
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

func (eh *EventHistory) populateTreeView() {
	eh.treeView.SetNodes(eh.treeNodes)
	if len(eh.treeNodes) > 0 {
		eh.updateSidePanelFromTree(eh.treeNodes[0])
	}
}

func (eh *EventHistory) populateTimelineView() {
	eh.timelineView.SetNodes(eh.treeNodes)
}

func (eh *EventHistory) showError(err error) {
	eh.table.ClearRows()
	eh.table.SetHeaders("ID", "TIME", "TYPE", "NAME", "DETAILS")
	eh.table.AddRowWithColor(theme.Error(),
		"",
		"",
		theme.IconError+" Error loading events",
		"",
		err.Error(),
	)
}

func (eh *EventHistory) toggleSidePanel() {
	eh.ToggleDetail()
}

func (eh *EventHistory) updateSidePanelFromList(index int) {
	if index < 0 || index >= len(eh.enhancedEvents) {
		return
	}

	ev := eh.enhancedEvents[index]
	icon := eventIcon(ev.Type)
	colorTag := eventColorTag(ev.Type)

	// Pretty print details if it contains JSON
	formattedDetails := formatSidePanelDetails(ev.Details)

	// Build name section if applicable
	var nameSection string
	name := getEventName(&ev)
	if name != "" {
		nameSection = fmt.Sprintf(`

[%s::b]Name[-:-:-]
[%s]%s[-]`,
			theme.TagAccent(),
			theme.TagFg(), name)
	}

	text := fmt.Sprintf(`
[%s::b]Event ID[-:-:-]
[%s]%d[-]

[%s::b]Type[-:-:-]
[%s]%s %s[-]%s

[%s::b]Time[-:-:-]
[%s]%s[-]

[%s::b]Details[-:-:-]
%s`,
		theme.TagAccent(),
		theme.TagFg(), ev.ID,
		theme.TagAccent(),
		colorTag, icon, ev.Type, nameSection,
		theme.TagAccent(),
		theme.TagFg(), ev.Time.Format("2006-01-02 15:04:05.000"),
		theme.TagAccent(),
		formattedDetails,
	)
	eh.sidePanel.SetText(text)
}

func (eh *EventHistory) updateSidePanelFromTree(node *temporal.EventTreeNode) {
	if node == nil {
		return
	}

	status := temporal.GetWorkflowStatus(node.Status)
	statusTag := status.ColorTag()
	icon := status.Icon()

	var durationStr string
	if node.Duration > 0 {
		durationStr = temporal.FormatDuration(node.Duration)
	} else {
		durationStr = "running..."
	}

	var attemptsStr string
	if node.Attempts > 1 {
		attemptsStr = fmt.Sprintf("\n\n[%s::b]Attempts[-:-:-]\n[%s]%d[-]", theme.TagAccent(), theme.TagFg(), node.Attempts)
	}

	// Extract result/failure from events
	var dataStr string
	for _, ev := range node.Events {
		if ev.Result != "" {
			formatted := formatSidePanelDetails(ev.Result)
			dataStr += fmt.Sprintf("\n\n[%s::b]Result[-:-:-]\n%s", theme.TagAccent(), formatted)
		}
		if ev.Failure != "" {
			formatted := formatSidePanelDetails(ev.Failure)
			dataStr += fmt.Sprintf("\n\n[%s::b]Failure[-:-:-]\n[%s]%s[-]", theme.TagAccent(), theme.TagError(), formatted)
		}
	}

	var eventsStr string
	if len(node.Events) > 0 {
		eventsStr = fmt.Sprintf("\n\n[%s::b]Events[-:-:-]", theme.TagAccent())
		for _, ev := range node.Events {
			evIcon := eventIcon(ev.Type)
			eventsStr += fmt.Sprintf("\n[%s]%s %s[-] [%s](%d)[-]",
				eventColorTag(ev.Type), evIcon, ev.Type, theme.TagFgDim(), ev.ID)
		}
	}

	text := fmt.Sprintf(`
[%s::b]Name[-:-:-]
[%s]%s[-]

[%s::b]Status[-:-:-]
[%s]%s %s[-]

[%s::b]Duration[-:-:-]
[%s]%s[-]

[%s::b]Start Time[-:-:-]
[%s]%s[-]%s%s%s`,
		theme.TagAccent(),
		theme.TagFg(), node.Name,
		theme.TagAccent(),
		statusTag, icon, node.Status,
		theme.TagAccent(),
		theme.TagFg(), durationStr,
		theme.TagAccent(),
		theme.TagFg(), node.StartTime.Format("2006-01-02 15:04:05.000"),
		attemptsStr,
		dataStr,
		eventsStr,
	)
	eh.sidePanel.SetText(text)
}

// Name returns the view name.
func (eh *EventHistory) Name() string {
	return "events"
}

// Start is called when the view becomes active.
func (eh *EventHistory) Start() {
	// Set up input capture for the current view mode
	eh.setupInputCapture()
	// Load data when view becomes active
	eh.loadData()
}

func (eh *EventHistory) setupInputCapture() {
	// Clear all input captures first
	eh.table.SetInputCapture(nil)
	eh.treeView.SetInputCapture(nil)
	eh.timelineView.SetInputCapture(nil)

	// Common input handler for all modes
	inputHandler := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'v':
			eh.cycleViewMode()
			return nil
		case '1':
			eh.setViewMode(ViewModeList)
			return nil
		case '2':
			eh.setViewMode(ViewModeTree)
			return nil
		case '3':
			eh.setViewMode(ViewModeTimeline)
			return nil
		case 'p':
			eh.toggleSidePanel()
			return nil
		case 'r':
			eh.loadData()
			return nil
		case 'y':
			eh.yankEventData()
			return nil
		case 'd':
			eh.showDetailModal()
			return nil
		case 'g':
			eh.jumpToChildWorkflow()
			return nil
		}

		// View-specific handlers
		switch eh.viewMode {
		case ViewModeTree:
			switch event.Rune() {
			case 'e':
				eh.treeView.ExpandAll()
				return nil
			case 'c':
				eh.treeView.CollapseAll()
				return nil
			case 'f':
				eh.treeView.JumpToFailed()
				return nil
			}
		case ViewModeTimeline:
			// Timeline handles its own input via InputHandler
		}

		return event
	}

	// Apply input capture to the appropriate component
	switch eh.viewMode {
	case ViewModeList:
		eh.table.SetInputCapture(inputHandler)
	case ViewModeTree:
		eh.treeView.SetInputCapture(inputHandler)
	case ViewModeTimeline:
		eh.timelineView.SetInputCapture(inputHandler)
	}
}

// Stop is called when the view is deactivated.
func (eh *EventHistory) Stop() {
	eh.table.SetInputCapture(nil)
	eh.treeView.SetInputCapture(nil)
	eh.timelineView.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (eh *EventHistory) Hints() []KeyHint {
	hints := []KeyHint{
		{Key: "v", Description: "Cycle View"},
		{Key: "1/2/3", Description: "List/Tree/Timeline"},
		{Key: "d", Description: "Detail"},
		{Key: "g", Description: "Go to Child"},
		{Key: "y", Description: "Yank"},
		{Key: "p", Description: "Preview"},
		{Key: "r", Description: "Refresh"},
	}

	// Add view-specific hints
	switch eh.viewMode {
	case ViewModeTree:
		hints = append(hints,
			KeyHint{Key: "e", Description: "Expand All"},
			KeyHint{Key: "c", Description: "Collapse All"},
			KeyHint{Key: "f", Description: "Jump to Failed"},
		)
	case ViewModeTimeline:
		hints = append(hints,
			KeyHint{Key: "+/-", Description: "Zoom"},
			KeyHint{Key: "h/l", Description: "Scroll"},
		)
	}

	hints = append(hints,
		KeyHint{Key: "j/k", Description: "Navigate"},
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "esc", Description: "Back"},
	)

	return hints
}

// Focus sets focus to the current view's primary component.
func (eh *EventHistory) Focus(delegate func(p tview.Primitive)) {
	switch eh.viewMode {
	case ViewModeList:
		delegate(eh.table)
	case ViewModeTree:
		delegate(eh.treeView)
	case ViewModeTimeline:
		delegate(eh.timelineView)
	default:
		delegate(eh.table)
	}
}

// Draw applies theme colors dynamically and draws the view.
func (eh *EventHistory) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	eh.sidePanel.SetBackgroundColor(bg)
	eh.MasterDetailView.Draw(screen)
}

// eventIcon returns an icon for the event type.
func eventIcon(eventType string) string {
	switch {
	case contains(eventType, "Started"):
		return theme.IconRunning
	case contains(eventType, "Completed"):
		return theme.IconCompleted
	case contains(eventType, "Failed"):
		return theme.IconError
	case contains(eventType, "Scheduled"):
		return theme.IconPending
	case contains(eventType, "Timer"):
		return theme.IconTimedOut
	case contains(eventType, "Signal"):
		return theme.IconActivity
	case contains(eventType, "Child"):
		return theme.IconWorkflow
	default:
		return theme.IconEvent
	}
}

// eventColor returns a color for the event type.
func eventColor(eventType string) tcell.Color {
	switch {
	case contains(eventType, "Started"):
		return temporal.StatusRunning.Color()
	case contains(eventType, "Completed"):
		return temporal.StatusCompleted.Color()
	case contains(eventType, "Failed"):
		return temporal.StatusFailed.Color()
	case contains(eventType, "Scheduled"):
		return theme.FgDim()
	default:
		return theme.Fg()
	}
}

// eventColorTag returns a color tag for the event type.
func eventColorTag(eventType string) string {
	switch {
	case contains(eventType, "Started"):
		return temporal.StatusRunning.ColorTag()
	case contains(eventType, "Completed"):
		return temporal.StatusCompleted.ColorTag()
	case contains(eventType, "Failed"):
		return temporal.StatusFailed.ColorTag()
	default:
		return theme.TagFg()
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getSelectedEventData returns the raw data for the currently selected event.
func (eh *EventHistory) getSelectedEventData() (string, string) {
	switch eh.viewMode {
	case ViewModeList:
		row := eh.table.SelectedRow()
		if row >= 0 && row < len(eh.enhancedEvents) {
			ev := eh.enhancedEvents[row]
			return ev.Type, eh.formatEventDataRaw(&ev)
		}
	case ViewModeTree:
		node := eh.treeView.SelectedNode()
		if node != nil && len(node.Events) > 0 {
			// Get the most relevant event (usually the last one with data)
			for i := len(node.Events) - 1; i >= 0; i-- {
				ev := node.Events[i]
				if ev.Result != "" || ev.Failure != "" || ev.Details != "" {
					return ev.Type, eh.formatEventDataRaw(ev)
				}
			}
			// Fallback to first event
			return node.Events[0].Type, eh.formatEventDataRaw(node.Events[0])
		}
	case ViewModeTimeline:
		lane := eh.timelineView.SelectedLane()
		if lane != nil && lane.Node != nil && len(lane.Node.Events) > 0 {
			ev := lane.Node.Events[len(lane.Node.Events)-1]
			return ev.Type, eh.formatEventDataRaw(ev)
		}
	}
	return "", ""
}

// formatEventDataRaw formats event data as raw JSON/text for copying.
func (eh *EventHistory) formatEventDataRaw(ev *temporal.EnhancedHistoryEvent) string {
	var parts []string

	if ev.Details != "" {
		parts = append(parts, fmt.Sprintf("Details: %s", prettyPrintJSON(ev.Details)))
	}
	if ev.Result != "" {
		parts = append(parts, fmt.Sprintf("Result: %s", prettyPrintJSON(ev.Result)))
	}
	if ev.Failure != "" {
		parts = append(parts, fmt.Sprintf("Failure: %s", prettyPrintJSON(ev.Failure)))
	}

	if len(parts) == 0 {
		return ev.Details
	}
	return strings.Join(parts, "\n\n")
}

// yankEventData copies the selected event's data to clipboard.
func (eh *EventHistory) yankEventData() {
	eventType, data := eh.getSelectedEventData()
	if data == "" {
		return
	}

	if err := copyToClipboard(data); err != nil {
		eh.sidePanel.SetText(fmt.Sprintf("[%s]%s Failed to copy: %s[-]",
			theme.TagError(), theme.IconError, err.Error()))
		return
	}

	// Show success feedback
	eh.sidePanel.SetText(fmt.Sprintf(`[%s::b]Copied to clipboard[-:-:-]

[%s]%s[-]

[%s]%s[-]`,
		theme.TagAccent(),
		theme.TagAccent(), eventType,
		temporal.StatusCompleted.ColorTag(), "Event data copied!"))

	// Restore preview after a brief delay
	go func() {
		time.Sleep(1500 * time.Millisecond)
		eh.app.JigApp().QueueUpdateDraw(func() {
			eh.refreshSidePanel()
		})
	}()
}

// refreshSidePanel updates the side panel based on current selection.
func (eh *EventHistory) refreshSidePanel() {
	switch eh.viewMode {
	case ViewModeList:
		row := eh.table.SelectedRow()
		if row >= 0 && row < len(eh.enhancedEvents) {
			eh.updateSidePanelFromList(row)
		}
	case ViewModeTree:
		node := eh.treeView.SelectedNode()
		if node != nil {
			eh.updateSidePanelFromTree(node)
		}
	case ViewModeTimeline:
		lane := eh.timelineView.SelectedLane()
		if lane != nil && lane.Node != nil {
			eh.updateSidePanelFromTree(lane.Node)
		}
	}
}

// showDetailModal shows a full-screen modal with pretty-printed event data.
func (eh *EventHistory) showDetailModal() {
	eventType, data := eh.getSelectedEventData()
	if data == "" {
		return
	}

	// Create modal with event details
	modal := components.NewModal(components.ModalConfig{
		Title:  truncateEventType(eventType),
		Width:  80,
		Height: 30,
	})

	// Create scrollable text view for the content
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	textView.SetBackgroundColor(theme.Bg())
	textView.SetTextColor(theme.Fg())

	// Format the data with syntax highlighting
	formattedData := formatDetailWithHighlighting(data)
	textView.SetText(formattedData)

	modal.SetContent(textView)
	modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Scroll"},
		{Key: "y", Description: "Copy"},
		{Key: "esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		eh.closeDetailModal()
	})

	// Handle input
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			eh.closeDetailModal()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, col := textView.GetScrollOffset()
				textView.ScrollTo(row+1, col)
				return nil
			case 'k':
				row, col := textView.GetScrollOffset()
				if row > 0 {
					textView.ScrollTo(row-1, col)
				}
				return nil
			case 'y':
				if err := copyToClipboard(data); err == nil {
					// Brief feedback
					originalText := textView.GetText(false)
					textView.SetText(fmt.Sprintf("[%s]Copied to clipboard![-]\n\n%s",
						temporal.StatusCompleted.ColorTag(), originalText))
				}
				return nil
			case 'q':
				eh.closeDetailModal()
				return nil
			}
		}
		return event
	})

	eh.app.JigApp().Pages().Push(modal)
	eh.app.JigApp().SetFocus(textView)
}

// truncateEventType shortens long event type names for the title.
func truncateEventType(eventType string) string {
	if len(eventType) > 30 {
		return eventType[:27] + "..."
	}
	return eventType
}

// formatSidePanelDetails formats event details with pretty-printed JSON and syntax highlighting.
func formatSidePanelDetails(details string) string {
	if details == "" {
		return fmt.Sprintf("[%s]No details[-]", theme.TagFgDim())
	}

	// First try to pretty print if it's pure JSON
	trimmed := strings.TrimSpace(details)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		formatted := prettyPrintJSON(details)
		return highlightFormattedJSON(formatted)
	}

	// Handle key-value format like "WorkflowType: Foo, TaskQueue: bar, Input: {...}"
	return formatKeyValueDetails(details)
}

// formatKeyValueDetails formats key-value style details with embedded JSON.
func formatKeyValueDetails(details string) string {
	var result strings.Builder

	// Split by common delimiters but preserve JSON objects
	parts := splitPreservingJSON(details)

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if i > 0 {
			result.WriteString("\n")
		}

		// Check if this part has a key: value structure
		if colonIdx := strings.Index(part, ":"); colonIdx > 0 {
			key := strings.TrimSpace(part[:colonIdx])
			value := strings.TrimSpace(part[colonIdx+1:])

			// Write the key in accent color
			result.WriteString(fmt.Sprintf("[%s]%s:[-] ", theme.TagAccent(), key))

			// Check if value is JSON
			if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
				formatted := prettyPrintJSON(value)
				if formatted != value {
					// JSON was successfully formatted - indent it
					lines := strings.Split(formatted, "\n")
					for j, line := range lines {
						if j == 0 {
							result.WriteString(highlightJSONValueLine(line))
						} else {
							result.WriteString("\n  ")
							result.WriteString(highlightJSONValueLine(line))
						}
					}
				} else {
					result.WriteString(highlightJSONValueLine(value))
				}
			} else {
				result.WriteString(highlightJSONValueLine(value))
			}
		} else {
			// No key-value structure, just highlight as value
			result.WriteString(highlightJSONValueLine(part))
		}
	}

	return result.String()
}

// splitPreservingJSON splits a string by commas while preserving JSON objects.
func splitPreservingJSON(s string) []string {
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

// highlightFormattedJSON applies syntax highlighting to already-formatted JSON.
func highlightFormattedJSON(formatted string) string {
	lines := strings.Split(formatted, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, highlightJSONValueLine(line))
	}
	return strings.Join(result, "\n")
}

// highlightJSONValueLine highlights a single line of JSON content.
func highlightJSONValueLine(line string) string {
	// Check for key: value pattern
	if colonIdx := strings.Index(line, ":"); colonIdx > 0 {
		prefix := line[:colonIdx]
		suffix := line[colonIdx+1:]

		trimmed := strings.TrimSpace(prefix)
		if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
			// JSON key with quotes - use accent color
			return fmt.Sprintf("[%s]%s[-]:[%s]%s[-]", theme.TagAccent(), prefix, theme.TagFg(), highlightValues(suffix))
		}
	}

	return highlightValues(line)
}

// highlightValues highlights JSON values (booleans, null, numbers).
func highlightValues(s string) string {
	result := s
	result = strings.ReplaceAll(result, "true", fmt.Sprintf("[%s]true[-]", temporal.StatusCompleted.ColorTag()))
	result = strings.ReplaceAll(result, "false", fmt.Sprintf("[%s]false[-]", temporal.StatusFailed.ColorTag()))
	result = strings.ReplaceAll(result, "null", fmt.Sprintf("[%s]null[-]", theme.TagFgDim()))
	return result
}

// closeDetailModal closes the detail modal.
func (eh *EventHistory) closeDetailModal() {
	eh.app.JigApp().Pages().DismissModal()
}

// prettyPrintJSON attempts to format a string as pretty JSON.
// If it's not valid JSON, returns the original string.
func prettyPrintJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Check if it looks like JSON
	if !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[") {
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

// formatDetailWithHighlighting adds color tags for JSON syntax highlighting.
func formatDetailWithHighlighting(data string) string {
	lines := strings.Split(data, "\n")
	var result []string

	for _, line := range lines {
		highlighted := highlightJSONLine(line)
		result = append(result, highlighted)
	}

	return strings.Join(result, "\n")
}

// highlightJSONLine adds tview color tags to a single line for JSON-like content.
func highlightJSONLine(line string) string {
	// Simple highlighting:
	// - Keys (before colon) in accent color
	// - Strings in green
	// - Numbers in yellow
	// - true/false/null in purple

	// If line contains a colon that looks like a JSON key
	if idx := strings.Index(line, ":"); idx > 0 {
		prefix := line[:idx]
		suffix := line[idx:]

		// Check if prefix looks like a key (has quotes or is a simple word)
		trimmed := strings.TrimSpace(prefix)
		if strings.HasPrefix(trimmed, "\"") || strings.HasPrefix(trimmed, "'") {
			// JSON key with quotes
			return fmt.Sprintf("[%s]%s[-]%s", theme.TagAccent(), prefix, highlightJSONValue(suffix))
		} else if !strings.Contains(trimmed, " ") && len(trimmed) > 0 {
			// Simple key without quotes (like "Details:", "Result:")
			return fmt.Sprintf("[%s::b]%s[-:-:-]%s", theme.TagAccent(), prefix, highlightJSONValue(suffix))
		}
	}

	return highlightJSONValue(line)
}

// highlightJSONValue highlights JSON values (strings, numbers, booleans).
func highlightJSONValue(s string) string {
	// Replace common JSON patterns with highlighted versions
	result := s

	// Highlight string values (simple approach)
	// This is a basic implementation - a full JSON parser would be more robust

	// Highlight booleans and null
	result = strings.ReplaceAll(result, "true", fmt.Sprintf("[%s]true[-]", temporal.StatusCompleted.ColorTag()))
	result = strings.ReplaceAll(result, "false", fmt.Sprintf("[%s]false[-]", temporal.StatusFailed.ColorTag()))
	result = strings.ReplaceAll(result, "null", fmt.Sprintf("[%s]null[-]", theme.TagFgDim()))

	return result
}

// jumpToChildWorkflow navigates to the child workflow if the selected event is a child workflow event.
func (eh *EventHistory) jumpToChildWorkflow() {
	var childWorkflowID, childRunID string

	switch eh.viewMode {
	case ViewModeList:
		row := eh.table.SelectedRow()
		if row >= 0 && row < len(eh.enhancedEvents) {
			ev := eh.enhancedEvents[row]
			childWorkflowID = ev.ChildWorkflowID
			childRunID = ev.ChildRunID
		}
	case ViewModeTree:
		node := eh.treeView.SelectedNode()
		if node != nil && node.Type == temporal.GroupChildWorkflow {
			// Find child workflow info from the node's events
			for _, ev := range node.Events {
				if ev.ChildWorkflowID != "" && ev.ChildRunID != "" {
					childWorkflowID = ev.ChildWorkflowID
					childRunID = ev.ChildRunID
					break
				}
			}
		}
	case ViewModeTimeline:
		lane := eh.timelineView.SelectedLane()
		if lane != nil && lane.Node != nil && lane.Node.Type == temporal.GroupChildWorkflow {
			// Find child workflow info from the node's events
			for _, ev := range lane.Node.Events {
				if ev.ChildWorkflowID != "" && ev.ChildRunID != "" {
					childWorkflowID = ev.ChildWorkflowID
					childRunID = ev.ChildRunID
					break
				}
			}
		}
	}

	// Navigate if we have valid child workflow info
	if childWorkflowID != "" && childRunID != "" {
		eh.app.NavigateToWorkflowDetail(childWorkflowID, childRunID)
	}
}
