package view

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowList displays a list of workflows with a preview panel.
type WorkflowList struct {
	*tview.Flex
	app              *App
	namespace        string
	table            *components.Table
	leftPanel        *components.Panel
	rightPanel       *components.Panel
	preview          *tview.TextView
	emptyState       *components.EmptyState
	noResultsState   *components.EmptyState
	allWorkflows     []temporal.Workflow // Full unfiltered list
	workflows        []temporal.Workflow // Filtered list for display
	filterText       string
	visibilityQuery  string // Temporal visibility query
	loading          bool
	autoRefresh      bool
	showPreview      bool
	refreshTicker    *time.Ticker
	stopRefresh      chan struct{}
	selectionMode    bool     // Multi-select mode active
	searchHistory    []string // History of visibility queries
	historyIndex     int      // Current position in history (-1 = not browsing)
	maxHistorySize   int      // Maximum number of history entries
	// Server-side completion support
	serverCompletions   []string            // Cached completions from server query
	lastCompletionQuery string              // Last query sent to server (to avoid duplicates)
	originalWorkflows   []temporal.Workflow // Original workflows before server search
}

// NewWorkflowList creates a new workflow list view.
func NewWorkflowList(app *App, namespace string) *WorkflowList {
	wl := &WorkflowList{
		Flex:           tview.NewFlex().SetDirection(tview.FlexColumn),
		app:            app,
		namespace:      namespace,
		table:          components.NewTable(),
		preview:        tview.NewTextView(),
		workflows:      []temporal.Workflow{},
		showPreview:    true,
		stopRefresh:    make(chan struct{}),
		searchHistory:  make([]string, 0, 50),
		historyIndex:   -1,
		maxHistorySize: 50,
	}
	wl.setup()
	return wl
}

func (wl *WorkflowList) setup() {
	wl.table.SetHeaders("WORKFLOW ID", "STATUS", "TYPE", "START TIME")
	wl.table.SetBorder(false)
	wl.table.SetBackgroundColor(theme.Bg())
	wl.SetBackgroundColor(theme.Bg())

	// Configure preview
	wl.preview.SetDynamicColors(true)
	wl.preview.SetBackgroundColor(theme.Bg())
	wl.preview.SetTextColor(theme.Fg())
	wl.preview.SetWordWrap(true)

	// Create empty states with input capture for keybindings
	emptyInputCapture := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'W':
			wl.showSignalWithStart()
			return nil
		case 'r':
			wl.loadData()
			return nil
		case 't':
			wl.app.NavigateToTaskQueues()
			return nil
		case 's':
			wl.app.NavigateToSchedules()
			return nil
		case 'a':
			wl.toggleAutoRefresh()
			return nil
		case 'p':
			wl.togglePreview()
			return nil
		}
		return event
	}

	wl.emptyState = components.NewEmptyState().
		SetIcon(theme.IconInfo).
		SetTitle("No Workflows").
		SetMessage("No workflows found in this namespace")
	wl.emptyState.SetInputCapture(emptyInputCapture)

	wl.noResultsState = components.NewEmptyState().
		SetIcon(theme.IconSearch).
		SetTitle("No Results").
		SetMessage("No workflows match the current filter")
	wl.noResultsState.SetInputCapture(emptyInputCapture)

	// Create panels with icons (blubber pattern)
	wl.leftPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow))
	wl.leftPanel.SetContent(wl.table)

	wl.rightPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Preview", theme.IconInfo))
	wl.rightPanel.SetContent(wl.preview)

	// Selection change handler to update preview
	wl.table.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row-1 < len(wl.workflows) {
			wl.updatePreview(wl.workflows[row-1])
		}
	})

	// Selection handler for drill-down
	wl.table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(wl.workflows) {
			wf := wl.workflows[row]
			wl.app.NavigateToWorkflowDetail(wf.ID, wf.RunID)
		}
	})

	wl.buildLayout()
}

func (wl *WorkflowList) buildLayout() {
	wl.Clear()
	if wl.showPreview {
		wl.AddItem(wl.leftPanel, 0, 3, true)
		wl.AddItem(wl.rightPanel, 0, 2, false)
	} else {
		wl.AddItem(wl.leftPanel, 0, 1, true)
	}
}

func (wl *WorkflowList) togglePreview() {
	wl.showPreview = !wl.showPreview
	wl.buildLayout()
	// Repopulate table to recalculate column widths for new layout
	wl.populateTable()
}

// RefreshTheme updates all component colors after a theme change.
func (wl *WorkflowList) RefreshTheme() {
	bg := theme.Bg()

	// Update main container
	wl.SetBackgroundColor(bg)

	// Update table
	wl.table.SetBackgroundColor(bg)

	// Update preview
	wl.preview.SetBackgroundColor(bg)
	wl.preview.SetTextColor(theme.Fg())

	// Re-render table with new theme colors
	wl.populateTable()
}

func (wl *WorkflowList) updatePreview(w temporal.Workflow) {
	now := time.Now()
	statusColor := theme.StatusColorTag(w.Status)
	statusIcon := theme.StatusIcon(w.Status)

	endTimeStr := "-"
	durationStr := "-"
	if w.EndTime != nil {
		endTimeStr = formatRelativeTime(now, *w.EndTime)
		durationStr = w.EndTime.Sub(w.StartTime).Round(time.Second).String()
	} else if w.Status == "Running" {
		durationStr = time.Since(w.StartTime).Round(time.Second).String()
	}

	text := fmt.Sprintf(`[%s::b]Workflow[-:-:-]
[%s]%s[-]

[%s]Status[-]
[%s]%s %s[-]

[%s]Type[-]
[%s]%s[-]

[%s]Started[-]
[%s]%s[-]

[%s]Ended[-]
[%s]%s[-]

[%s]Duration[-]
[%s]%s[-]

[%s]Task Queue[-]
[%s]%s[-]

[%s]Run ID[-]
[%s]%s[-]`,
		theme.TagPanelTitle(),
		theme.TagFg(), truncate(w.ID, 35),
		theme.TagFgDim(),
		statusColor, statusIcon, w.Status,
		theme.TagFgDim(),
		theme.TagFg(), w.Type,
		theme.TagFgDim(),
		theme.TagFg(), formatRelativeTime(now, w.StartTime),
		theme.TagFgDim(),
		theme.TagFg(), endTimeStr,
		theme.TagFgDim(),
		theme.TagFg(), durationStr,
		theme.TagFgDim(),
		theme.TagFg(), w.TaskQueue,
		theme.TagFgDim(),
		theme.TagFgDim(), truncate(w.RunID, 30),
	)
	wl.preview.SetText(text)
}

func (wl *WorkflowList) setLoading(loading bool) {
	wl.loading = loading
}

func (wl *WorkflowList) loadData() {
	provider := wl.app.Provider()
	if provider == nil {
		wl.loadMockData()
		return
	}

	wl.setLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Resolve time placeholders in the query
		resolvedQuery := resolveTimePlaceholders(wl.visibilityQuery)
		opts := temporal.ListOptions{
			PageSize: 100,
			Query:    resolvedQuery,
		}
		workflows, _, err := provider.ListWorkflows(ctx, wl.namespace, opts)

		wl.app.JigApp().QueueUpdateDraw(func() {
			wl.setLoading(false)
			if err != nil {
				wl.showError(err)
				return
			}
			wl.allWorkflows = workflows
			wl.applyFilter()
			// Set focus to table after data loads
			if len(wl.workflows) > 0 {
				wl.app.JigApp().SetFocus(wl.table)
			}
		})
	}()
}

// applyFilter filters allWorkflows based on filterText and updates the display.
func (wl *WorkflowList) applyFilter() {
	wl.applyFilterWithFallback(false)
}

// applyFilterWithFallback filters locally, optionally falling back to server-side search.
func (wl *WorkflowList) applyFilterWithFallback(serverFallback bool) {
	if wl.filterText == "" {
		wl.workflows = wl.allWorkflows
	} else {
		filter := strings.ToLower(wl.filterText)
		wl.workflows = nil
		for _, w := range wl.allWorkflows {
			if strings.Contains(strings.ToLower(w.ID), filter) ||
				strings.Contains(strings.ToLower(w.Type), filter) ||
				strings.Contains(strings.ToLower(w.Status), filter) {
				wl.workflows = append(wl.workflows, w)
			}
		}

		if len(wl.workflows) == 0 && serverFallback && wl.visibilityQuery == "" {
			wl.convertFilterToVisibilityQuery()
			return
		}
	}
	wl.populateTable()
	wl.updateStats()
}

func (wl *WorkflowList) convertFilterToVisibilityQuery() {
	if wl.filterText == "" {
		return
	}

	searchTerm := wl.filterText
	wl.visibilityQuery = fmt.Sprintf(
		"WorkflowId STARTS_WITH '%s' OR WorkflowType STARTS_WITH '%s'",
		searchTerm, searchTerm,
	)
	wl.filterText = ""
	wl.updatePanelTitle()
	wl.loadData()
}

func (wl *WorkflowList) loadMockData() {
	now := time.Now()
	wl.allWorkflows = []temporal.Workflow{
		{
			ID: "order-processing-abc123", RunID: "run-001-xyz", Type: "OrderWorkflow",
			Status: "Running", Namespace: wl.namespace, TaskQueue: "order-tasks",
			StartTime: now.Add(-5 * time.Minute),
		},
		{
			ID: "payment-xyz789", RunID: "run-002-abc", Type: "PaymentWorkflow",
			Status: "Completed", Namespace: wl.namespace, TaskQueue: "payment-tasks",
			StartTime: now.Add(-1 * time.Hour), EndTime: ptr(now.Add(-55 * time.Minute)),
		},
		{
			ID: "shipment-def456", RunID: "run-003-def", Type: "ShipmentWorkflow",
			Status: "Failed", Namespace: wl.namespace, TaskQueue: "shipment-tasks",
			StartTime: now.Add(-30 * time.Minute), EndTime: ptr(now.Add(-25 * time.Minute)),
		},
		{
			ID: "inventory-check-111", RunID: "run-004-ghi", Type: "InventoryWorkflow",
			Status: "Running", Namespace: wl.namespace, TaskQueue: "inventory-tasks",
			StartTime: now.Add(-10 * time.Minute),
		},
		{
			ID: "user-signup-222", RunID: "run-005-jkl", Type: "UserOnboardingWorkflow",
			Status: "Completed", Namespace: wl.namespace, TaskQueue: "user-tasks",
			StartTime: now.Add(-2 * time.Hour), EndTime: ptr(now.Add(-1*time.Hour - 45*time.Minute)),
		},
	}
	wl.applyFilter()
}

func ptr[T any](v T) *T {
	return &v
}

func (wl *WorkflowList) populateTable() {
	currentRow := wl.table.SelectedRow()

	wl.table.ClearRows()
	wl.table.SetHeaders("WORKFLOW ID", "STATUS", "TYPE", "START TIME")

	if len(wl.workflows) == 0 {
		if len(wl.allWorkflows) == 0 {
			wl.leftPanel.SetContent(wl.emptyState)
		} else {
			wl.leftPanel.SetContent(wl.noResultsState)
		}
		wl.preview.SetText("")
		return
	}

	wl.leftPanel.SetContent(wl.table)

	// Calculate dynamic column widths based on available space
	idWidth, typeWidth := wl.calculateColumnWidths()

	now := time.Now()
	for _, w := range wl.workflows {
		wl.table.AddStyledRowSimple(w.Status,
			truncateIfNeeded(w.ID, idWidth),
			w.Status,
			truncateIfNeeded(w.Type, typeWidth),
			formatRelativeTime(now, w.StartTime),
		)
	}

	if wl.table.RowCount() > 0 {
		if currentRow >= 0 && currentRow < len(wl.workflows) {
			wl.table.SelectRow(currentRow)
			wl.updatePreview(wl.workflows[currentRow])
		} else {
			wl.table.SelectRow(0)
			if len(wl.workflows) > 0 {
				wl.updatePreview(wl.workflows[0])
			}
		}
	}
}

func (wl *WorkflowList) updateStats() {
	var running, completed, failed int
	for _, w := range wl.workflows {
		switch w.Status {
		case temporal.StatusRunning:
			running++
		case temporal.StatusCompleted:
			completed++
		case temporal.StatusFailed:
			failed++
		}
	}
	wl.app.SetWorkflowStats(WorkflowStats{
		Running:   running,
		Completed: completed,
		Failed:    failed,
	})
}

func (wl *WorkflowList) showError(err error) {
	wl.table.ClearRows()
	wl.table.SetHeaders("WORKFLOW ID", "STATUS", "TYPE", "START TIME")
	wl.table.AddRowWithColor(theme.Error(),
		theme.IconError+" Error loading workflows",
		err.Error(),
		"",
		"",
	)
}

func (wl *WorkflowList) toggleAutoRefresh() {
	wl.autoRefresh = !wl.autoRefresh
	if wl.autoRefresh {
		wl.startAutoRefresh()
	} else {
		wl.stopAutoRefresh()
	}
}

func (wl *WorkflowList) startAutoRefresh() {
	wl.refreshTicker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-wl.refreshTicker.C:
				wl.app.JigApp().QueueUpdateDraw(func() {
					wl.loadData()
				})
			case <-wl.stopRefresh:
				return
			}
		}
	}()
}

func (wl *WorkflowList) stopAutoRefresh() {
	if wl.refreshTicker != nil {
		wl.refreshTicker.Stop()
		wl.refreshTicker = nil
	}
	select {
	case wl.stopRefresh <- struct{}{}:
	default:
	}
}

// Name returns the view name.
func (wl *WorkflowList) Name() string {
	return "workflows"
}

// Start is called when the view becomes active.
func (wl *WorkflowList) Start() {
	wl.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' && wl.selectionMode {
			wl.table.ToggleSelection()
			wl.updateSelectionPreview()
			return nil
		}

		switch event.Rune() {
		case '/':
			wl.showFilter()
			return nil
		case 'F':
			wl.showVisibilityQuery()
			return nil
		case 'f':
			wl.showQueryTemplates()
			return nil
		case 'D':
			wl.showDateRangePicker()
			return nil
		case 't':
			wl.app.NavigateToTaskQueues()
			return nil
		case 's':
			wl.app.NavigateToSchedules()
			return nil
		case 'a':
			wl.toggleAutoRefresh()
			return nil
		case 'r':
			wl.loadData()
			return nil
		case 'p':
			wl.togglePreview()
			return nil
		case 'y':
			wl.copyWorkflowID()
			return nil
		case 'v':
			wl.toggleSelectionMode()
			return nil
		case 'c':
			if wl.selectionMode && len(wl.table.GetSelectedRows()) > 0 {
				wl.showBatchCancelConfirm()
				return nil
			}
		case 'X':
			if wl.selectionMode && len(wl.table.GetSelectedRows()) > 0 {
				wl.showBatchTerminateConfirm()
				return nil
			}
		case 'C':
			if wl.visibilityQuery != "" {
				wl.clearVisibilityQuery()
				return nil
			}
		case 'L':
			wl.showSavedFilters()
			return nil
		case 'S':
			if wl.visibilityQuery != "" {
				wl.showSaveFilter()
				return nil
			}
		case 'W':
			wl.showSignalWithStart()
			return nil
		case 'd':
			wl.startDiff()
			return nil
		}

		if event.Key() == tcell.KeyCtrlA && wl.selectionMode {
			wl.table.SelectAll()
			wl.updateSelectionPreview()
			return nil
		}

		return event
	})

	wl.loadData()
}

// Stop is called when the view is deactivated.
func (wl *WorkflowList) Stop() {
	wl.table.SetInputCapture(nil)
	wl.stopAutoRefresh()
	wl.app.ClearWorkflowStats()
}

// Hints returns keybinding hints for this view.
func (wl *WorkflowList) Hints() []KeyHint {
	if wl.selectionMode {
		hints := []KeyHint{
			{Key: "space", Description: "Select"},
			{Key: "Ctrl+A", Description: "Select All"},
			{Key: "v", Description: "Exit Select"},
		}
		if len(wl.table.GetSelectedRows()) > 0 {
			hints = append(hints,
				KeyHint{Key: "c", Description: "Cancel"},
				KeyHint{Key: "X", Description: "Terminate"},
			)
		}
		hints = append(hints, KeyHint{Key: "esc", Description: "Back"})
		return hints
	}

	hints := []KeyHint{
		{Key: "enter", Description: "Detail"},
		{Key: "/", Description: "Filter"},
		{Key: "F", Description: "Query"},
		{Key: "f", Description: "Templates"},
		{Key: "D", Description: "Date Range"},
	}
	if wl.visibilityQuery != "" {
		hints = append(hints,
			KeyHint{Key: "C", Description: "Clear Query"},
			KeyHint{Key: "S", Description: "Save Filter"},
		)
	}
	hints = append(hints,
		KeyHint{Key: "L", Description: "Load Filter"},
		KeyHint{Key: "d", Description: "Diff"},
		KeyHint{Key: "v", Description: "Select Mode"},
		KeyHint{Key: "W", Description: "Signal+Start"},
		KeyHint{Key: "y", Description: "Copy ID"},
		KeyHint{Key: "r", Description: "Refresh"},
		KeyHint{Key: "p", Description: "Preview"},
		KeyHint{Key: "a", Description: "Auto-refresh"},
		KeyHint{Key: "t", Description: "Task Queues"},
		KeyHint{Key: "s", Description: "Schedules"},
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "?", Description: "Help"},
		KeyHint{Key: "esc", Description: "Back"},
	)
	return hints
}

// HandleEscape implements EscapeHandler to clear filter state before navigation.
func (wl *WorkflowList) HandleEscape() bool {
	if wl.filterText != "" || wl.visibilityQuery != "" || wl.originalWorkflows != nil {
		wl.clearAllFilters()
		return true
	}
	return false
}

// Focus sets focus to the table.
func (wl *WorkflowList) Focus(delegate func(p tview.Primitive)) {
	if len(wl.workflows) == 0 && len(wl.allWorkflows) == 0 {
		delegate(wl.Flex)
		return
	}
	delegate(wl.table)
}

// Draw applies theme colors dynamically and draws the view.
func (wl *WorkflowList) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	wl.SetBackgroundColor(bg)
	wl.preview.SetBackgroundColor(bg)
	wl.preview.SetTextColor(theme.Fg())
	wl.Flex.Draw(screen)
}

func (wl *WorkflowList) showFilter() {
	wl.originalWorkflows = wl.allWorkflows

	wl.app.ShowFilterMode(wl.filterText, FilterModeCallbacks{
		OnSubmit: func(text string) {
			wl.filterText = text
			if text != "" {
				// Apply filter with server fallback if no local results
				wl.applyFilterWithFallback(true)
			} else {
				wl.applyFilter()
			}
			wl.updatePanelTitle()
		},
		OnCancel: func() {
			wl.closeFilter()
		},
		OnChange: func(text string) {
			wl.filterText = text
			wl.applyFilterWithServerSearch(text)
		},
	})
}

// applyFilterWithServerSearch filters locally, and if no results, triggers server search.
func (wl *WorkflowList) applyFilterWithServerSearch(text string) {
	if text == "" {
		wl.workflows = wl.allWorkflows
		wl.populateTable()
		wl.updateStats()
		wl.updateFilterTitle("", "")
		return
	}

	// Try local filter first
	filter := strings.ToLower(text)
	wl.workflows = nil
	for _, w := range wl.allWorkflows {
		if strings.Contains(strings.ToLower(w.ID), filter) ||
			strings.Contains(strings.ToLower(w.Type), filter) ||
			strings.Contains(strings.ToLower(w.Status), filter) {
			wl.workflows = append(wl.workflows, w)
		}
	}

	// Show top match hint
	topHint := ""
	if len(wl.workflows) > 0 {
		topHint = wl.workflows[0].ID
	}
	wl.updateFilterTitle(text, topHint)

	// If no local results and query is long enough, search server
	if len(wl.workflows) == 0 && len(text) >= 2 {
		// Avoid duplicate requests
		if text == wl.lastCompletionQuery {
			return
		}
		wl.lastCompletionQuery = text
		wl.searchServer(text)
		return
	}

	wl.populateTable()
	wl.updateStats()
}

// searchServer performs a server-side search and updates the table.
func (wl *WorkflowList) searchServer(searchTerm string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := fmt.Sprintf(
			"WorkflowId STARTS_WITH '%s' OR WorkflowType STARTS_WITH '%s'",
			searchTerm, searchTerm,
		)
		opts := temporal.ListOptions{
			PageSize: 50,
			Query:    query,
		}
		workflows, _, err := provider.ListWorkflows(ctx, wl.namespace, opts)

		wl.app.JigApp().QueueUpdateDraw(func() {
			// Only update if we're still filtering with the same term
			if wl.filterText != searchTerm {
				return
			}

			if err != nil {
				return
			}

			wl.workflows = workflows
			wl.serverCompletions = make([]string, 0, len(workflows))
			for _, w := range workflows {
				wl.serverCompletions = append(wl.serverCompletions, w.ID)
			}

			// Update hint with top server result
			topHint := ""
			if len(workflows) > 0 {
				topHint = workflows[0].ID
			}
			wl.updateFilterTitle(searchTerm, topHint)

			wl.populateTable()
			wl.updateStats()
		})
	}()
}

// updateFilterTitle updates the panel title with filter info and hint.
func (wl *WorkflowList) updateFilterTitle(filter, hint string) {
	if filter == "" {
		wl.leftPanel.SetTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow))
		wl.app.SetFilterSuggestion("")
		return
	}

	title := fmt.Sprintf("%s Workflows [%s](/%s", theme.IconWorkflow, theme.TagFgDim(), filter)
	if hint != "" && strings.HasPrefix(strings.ToLower(hint), strings.ToLower(filter)) {
		// Show autocomplete hint: the part after what user typed
		suffix := hint[len(filter):]
		if suffix != "" {
			title += fmt.Sprintf("[%s]%s[-]", theme.TagFgMuted(), suffix)
		}
		// Set inline ghost text suggestion in command bar
		wl.app.SetFilterSuggestion(hint)
	} else {
		wl.app.SetFilterSuggestion("")
	}
	title += ")[-]"
	wl.leftPanel.SetTitle(title)
}

func (wl *WorkflowList) closeFilter() {
	wl.serverCompletions = nil
	wl.lastCompletionQuery = ""

	if wl.filterText == "" && wl.visibilityQuery == "" && wl.originalWorkflows != nil {
		wl.allWorkflows = wl.originalWorkflows
		wl.workflows = wl.originalWorkflows
		wl.originalWorkflows = nil
		wl.populateTable()
		wl.updateStats()
		wl.updatePanelTitle()
	}
}

func (wl *WorkflowList) clearAllFilters() {
	wl.filterText = ""
	wl.visibilityQuery = ""
	wl.serverCompletions = nil
	wl.lastCompletionQuery = ""

	if wl.originalWorkflows != nil {
		wl.allWorkflows = wl.originalWorkflows
		wl.workflows = wl.originalWorkflows
		wl.originalWorkflows = nil
		wl.populateTable()
		wl.updateStats()
		wl.updatePanelTitle()
	} else {
		wl.loadData()
	}
}

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

// calculateColumnWidths determines optimal column widths based on available space.
// Returns (idWidth, typeWidth) where 0 means no truncation needed.
func (wl *WorkflowList) calculateColumnWidths() (int, int) {
	// Calculate width based on parent flex and preview state
	// We can't rely on leftPanel.GetInnerRect() as it may have stale dimensions
	_, _, totalWidth, _ := wl.Flex.GetInnerRect()

	var width int
	if totalWidth > 0 {
		if wl.showPreview {
			// Left panel gets 3/5 of space when preview is shown
			width = (totalWidth * 3) / 5
		} else {
			// Left panel gets full width when preview is hidden
			width = totalWidth
		}
		// Account for panel border/padding (~4 chars)
		width -= 4
	}

	// If no width available (not yet drawn), use conservative defaults
	if width <= 0 {
		return 25, 15
	}

	// Fixed column widths:
	// STATUS: max 12 chars (for "TERMINATED" + padding)
	// START TIME: max 12 chars (for "12mo ago" + padding)
	// Column separators: roughly 2 chars between each of 4 columns = 6 chars
	// Left margin/selection indicator: ~2 chars
	const (
		statusWidth    = 12
		startTimeWidth = 12
		separators     = 8
		minIDWidth     = 15 // Minimum readable ID width
		minTypeWidth   = 10 // Minimum readable type width
	)

	fixedWidth := statusWidth + startTimeWidth + separators
	availableForVariable := width - fixedWidth

	if availableForVariable <= 0 {
		// Extremely narrow terminal, use minimums
		return minIDWidth, minTypeWidth
	}

	// Priority: ID > Type
	// Give ID 60% of variable space, Type 40%
	idWidth := (availableForVariable * 60) / 100
	typeWidth := availableForVariable - idWidth

	// If we have plenty of space, don't truncate at all (return 0)
	// Typical workflow IDs are ~36 chars (UUID), types vary widely
	if idWidth >= 50 {
		idWidth = 0 // No truncation needed for ID
	}
	if typeWidth >= 40 {
		typeWidth = 0 // No truncation needed for Type
	}

	// Ensure minimums if we are truncating
	if idWidth > 0 && idWidth < minIDWidth {
		idWidth = minIDWidth
	}
	if typeWidth > 0 && typeWidth < minTypeWidth {
		typeWidth = minTypeWidth
	}

	return idWidth, typeWidth
}

// Selection mode methods

func (wl *WorkflowList) toggleSelectionMode() {
	wl.selectionMode = !wl.selectionMode
	if wl.selectionMode {
		wl.table.SetMultiSelect(true)
		wl.leftPanel.SetTitle(fmt.Sprintf("%s Workflows (Select Mode)", theme.IconWorkflow))
	} else {
		wl.table.SetMultiSelect(false)
		wl.table.ClearSelection()
		wl.leftPanel.SetTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow))
	}
	wl.app.JigApp().Menu().SetHints(wl.Hints())
}

func (wl *WorkflowList) updateSelectionPreview() {
	count := len(wl.table.GetSelectedRows())
	if count == 0 {
		row := wl.table.SelectedRow()
		if row >= 0 && row < len(wl.workflows) {
			wl.updatePreview(wl.workflows[row])
		}
	} else {
		var running, completed, failed int
		selected := wl.table.GetSelectedRows()
		for _, idx := range selected {
			if idx < len(wl.workflows) {
				switch wl.workflows[idx].Status {
				case "Running":
					running++
				case "Completed":
					completed++
				case "Failed":
					failed++
				}
			}
		}

		text := fmt.Sprintf(`[%s::b]Selected Workflows[-:-:-]
[%s]%d workflow(s)[-]

[%s]Status Breakdown[-]
[%s]%s Running: %d[-]
[%s]%s Completed: %d[-]
[%s]%s Failed: %d[-]

[%s]Press 'c' to cancel or 'X' to terminate selected workflows[-]`,
			theme.TagPanelTitle(),
			theme.TagAccent(), count,
			theme.TagFgDim(),
			theme.StatusColorTag("Running"), theme.StatusIcon("Running"), running,
			theme.StatusColorTag("Completed"), theme.StatusIcon("Completed"), completed,
			theme.StatusColorTag("Failed"), theme.StatusIcon("Failed"), failed,
			theme.TagFgDim())
		wl.preview.SetText(text)
	}
	wl.app.JigApp().Menu().SetHints(wl.Hints())
}

// Batch operation methods

func (wl *WorkflowList) showBatchCancelConfirm() {
	selected := wl.table.GetSelectedRows()
	if len(selected) == 0 {
		return
	}

	// Count running workflows
	var runningCount int
	for _, idx := range selected {
		if idx < len(wl.workflows) && wl.workflows[idx].Status == "Running" {
			runningCount++
		}
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Cancel %d Workflow(s)", theme.IconWarning, len(selected)),
		Width:    60,
		Height:   14,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("reason", "Reason (optional)", "Batch cancelled via tempo")

	infoText := tview.NewTextView().SetDynamicColors(true)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf(`[%s]Selected:[-] %d workflow(s)
[%s]Running:[-] %d (will be cancelled)
[%s]Other:[-] %d (will be skipped)`,
		theme.TagFgDim(), len(selected),
		theme.TagAccent(), runningCount,
		theme.TagFgDim(), len(selected)-runningCount))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(infoText, 4, 0, false).
		AddItem(form, 0, 1, true)
	content.SetBackgroundColor(theme.Bg())

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Confirm"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		reason := values["reason"].(string)
		wl.closeModal("batch-cancel")
		wl.executeBatchCancel(selected, reason)
	})
	modal.SetOnCancel(func() {
		wl.closeModal("batch-cancel")
	})

	wl.app.JigApp().Pages().AddPage("batch-cancel", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) executeBatchCancel(indices []int, reason string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var succeeded, failed int
		for _, idx := range indices {
			if idx >= len(wl.workflows) {
				continue
			}
			wf := wl.workflows[idx]
			if wf.Status != "Running" {
				continue
			}

			err := provider.CancelWorkflow(ctx, wl.namespace, wf.ID, wf.RunID, reason)
			if err != nil {
				failed++
			} else {
				succeeded++
			}
		}

		wl.app.JigApp().QueueUpdateDraw(func() {
			wl.toggleSelectionMode()
			wl.loadData()
			wl.preview.SetText(fmt.Sprintf(`[%s::b]Batch Cancel Complete[-:-:-]

[%s]Cancelled:[-] %d workflow(s)
[%s]Failed:[-] %d workflow(s)`,
				theme.TagPanelTitle(),
				theme.TagSuccess(), succeeded,
				theme.TagError(), failed))
		})
	}()
}

func (wl *WorkflowList) showBatchTerminateConfirm() {
	selected := wl.table.GetSelectedRows()
	if len(selected) == 0 {
		return
	}

	// Count running workflows
	var runningCount int
	for _, idx := range selected {
		if idx < len(wl.workflows) && wl.workflows[idx].Status == "Running" {
			runningCount++
		}
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Terminate %d Workflow(s)", theme.IconError, len(selected)),
		Width:    65,
		Height:   16,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("reason", "Reason (required)", "")

	warningText := tview.NewTextView().SetDynamicColors(true)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf(`[%s]⚠ WARNING: This action cannot be undone![-]

[%s]Selected:[-] %d workflow(s)
[%s]Running:[-] %d (will be terminated)
[%s]Other:[-] %d (will be skipped)`,
		theme.TagError(),
		theme.TagFgDim(), len(selected),
		theme.TagAccent(), runningCount,
		theme.TagFgDim(), len(selected)-runningCount))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(warningText, 5, 0, false).
		AddItem(form, 0, 1, true)
	content.SetBackgroundColor(theme.Bg())

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Terminate"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		reason := values["reason"].(string)
		if reason == "" {
			return // Require reason for terminate
		}
		wl.closeModal("batch-terminate")
		wl.executeBatchTerminate(selected, reason)
	})
	modal.SetOnCancel(func() {
		wl.closeModal("batch-terminate")
	})

	wl.app.JigApp().Pages().AddPage("batch-terminate", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) executeBatchTerminate(indices []int, reason string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var succeeded, failed int
		for _, idx := range indices {
			if idx >= len(wl.workflows) {
				continue
			}
			wf := wl.workflows[idx]
			if wf.Status != "Running" {
				continue
			}

			err := provider.TerminateWorkflow(ctx, wl.namespace, wf.ID, wf.RunID, reason)
			if err != nil {
				failed++
			} else {
				succeeded++
			}
		}

		wl.app.JigApp().QueueUpdateDraw(func() {
			wl.toggleSelectionMode()
			wl.loadData()
			wl.preview.SetText(fmt.Sprintf(`[%s::b]Batch Terminate Complete[-:-:-]

[%s]Terminated:[-] %d workflow(s)
[%s]Failed:[-] %d workflow(s)`,
				theme.TagPanelTitle(),
				theme.TagSuccess(), succeeded,
				theme.TagError(), failed))
		})
	}()
}

func (wl *WorkflowList) closeModal(name string) {
	wl.app.JigApp().Pages().RemovePage(name)
	wl.app.JigApp().SetFocus(wl.table)
}

// Visibility query methods

func (wl *WorkflowList) showVisibilityQuery() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Visibility Query", theme.IconSearch),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("query", "Query", wl.visibilityQuery)

	helpText := tview.NewTextView().SetDynamicColors(true)
	helpText.SetBackgroundColor(theme.Bg())
	helpText.SetText(fmt.Sprintf(`[%s]Examples:[-]
  WorkflowType = 'OrderWorkflow'
  ExecutionStatus = 'Running'
  StartTime > '2024-01-01T00:00:00Z'
  WorkflowId STARTS_WITH 'order-'`,
		theme.TagFgDim()))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 3, 0, true).
		AddItem(helpText, 0, 1, false)
	content.SetBackgroundColor(theme.Bg())

	form.SetOnSubmit(func(values map[string]any) {
		query := values["query"].(string)
		wl.closeModal("visibility-query")
		wl.applyVisibilityQuery(query)
	})
	form.SetOnCancel(func() {
		wl.closeModal("visibility-query")
	})

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		query := values["query"].(string)
		wl.closeModal("visibility-query")
		wl.applyVisibilityQuery(query)
	})
	modal.SetOnCancel(func() {
		wl.closeModal("visibility-query")
	})

	wl.app.JigApp().Pages().AddPage("visibility-query", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) applyVisibilityQuery(query string) {
	if query != "" && query != wl.visibilityQuery {
		wl.addToHistory(query)
	}
	wl.visibilityQuery = query
	wl.filterText = ""
	wl.updatePanelTitle()
	wl.loadData()
}

func (wl *WorkflowList) addToHistory(query string) {
	// Don't add duplicates of the most recent
	if len(wl.searchHistory) > 0 && wl.searchHistory[len(wl.searchHistory)-1] == query {
		return
	}
	wl.searchHistory = append(wl.searchHistory, query)
	if len(wl.searchHistory) > wl.maxHistorySize {
		wl.searchHistory = wl.searchHistory[1:]
	}
	wl.historyIndex = -1
}

func (wl *WorkflowList) showQueryTemplates() {
	templates := []struct {
		name  string
		query string
	}{
		{"Running Workflows", "ExecutionStatus = 'Running'"},
		{"Failed Workflows", "ExecutionStatus = 'Failed'"},
		{"Completed Workflows", "ExecutionStatus = 'Completed'"},
		{"Cancelled Workflows", "ExecutionStatus = 'Canceled'"},
		{"Timed Out Workflows", "ExecutionStatus = 'TimedOut'"},
		{"Started Today", "StartTime > $TODAY"},
		{"Started This Hour", "StartTime > $HOUR_AGO"},
		{"Long Running (>1h)", "ExecutionStatus = 'Running' AND StartTime < $HOUR_AGO"},
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query Templates", theme.IconInfo),
		Width:    60,
		Height:   18,
		Backdrop: true,
	})

	table := components.NewTable()
	table.SetHeaders("TEMPLATE", "QUERY")
	table.SetBorder(false)

	for _, t := range templates {
		table.AddRow(t.name, truncate(t.query, 35))
	}
	table.SelectRow(0)

	table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(templates) {
			wl.closeModal("query-templates")
			wl.applyVisibilityQuery(templates[row].query)
		}
	})

	modal.SetContent(table)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal("query-templates")
	})

	wl.app.JigApp().Pages().AddPage("query-templates", modal, true, true)
	wl.app.JigApp().SetFocus(table)
}

func (wl *WorkflowList) showDateRangePicker() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Date Range Filter", theme.IconInfo),
		Width:    55,
		Height:   14,
		Backdrop: true,
	})

	presets := []string{
		"Last Hour",
		"Last 24 Hours",
		"Last 7 Days",
		"Last 30 Days",
		"Today",
		"Yesterday",
	}

	form := components.NewForm()
	form.AddSelect("preset", "Time Range", presets)

	form.SetOnSubmit(func(values map[string]any) {
		preset := values["preset"].(string)
		wl.closeModal("date-range")
		wl.applyDatePreset(preset)
	})
	form.SetOnCancel(func() {
		wl.closeModal("date-range")
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		preset := values["preset"].(string)
		wl.closeModal("date-range")
		wl.applyDatePreset(preset)
	})
	modal.SetOnCancel(func() {
		wl.closeModal("date-range")
	})

	wl.app.JigApp().Pages().AddPage("date-range", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) applyDatePreset(preset string) {
	now := time.Now()
	var startTime time.Time

	switch preset {
	case "Last Hour":
		startTime = now.Add(-1 * time.Hour)
	case "Last 24 Hours":
		startTime = now.Add(-24 * time.Hour)
	case "Last 7 Days":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "Last 30 Days":
		startTime = now.Add(-30 * 24 * time.Hour)
	case "Today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "Yesterday":
		yesterday := now.Add(-24 * time.Hour)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
	default:
		return
	}

	query := fmt.Sprintf("StartTime > '%s'", startTime.UTC().Format(time.RFC3339))
	wl.applyVisibilityQuery(query)
}

func (wl *WorkflowList) showSavedFilters() {
	// For now, show history as "saved" filters
	if len(wl.searchHistory) == 0 {
		wl.showNoSavedFilters()
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query History", theme.IconInfo),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	table := components.NewTable()
	table.SetHeaders("#", "QUERY")
	table.SetBorder(false)

	// Show most recent first
	for i := len(wl.searchHistory) - 1; i >= 0; i-- {
		table.AddRow(
			fmt.Sprintf("%d", len(wl.searchHistory)-i),
			truncate(wl.searchHistory[i], 55),
		)
	}
	table.SelectRow(0)

	table.SetOnSelect(func(row int) {
		// Convert display row to history index (most recent first)
		historyIdx := len(wl.searchHistory) - 1 - row
		if historyIdx >= 0 && historyIdx < len(wl.searchHistory) {
			wl.closeModal("saved-filters")
			wl.applyVisibilityQuery(wl.searchHistory[historyIdx])
		}
	})

	modal.SetContent(table)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal("saved-filters")
	})

	wl.app.JigApp().Pages().AddPage("saved-filters", modal, true, true)
	wl.app.JigApp().SetFocus(table)
}

func (wl *WorkflowList) showNoSavedFilters() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query History", theme.IconInfo),
		Width:    50,
		Height:   10,
		Backdrop: true,
	})

	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBackgroundColor(theme.Bg())
	text.SetTextAlign(tview.AlignCenter)
	text.SetText(fmt.Sprintf(`[%s]No query history yet.[-]

[%s]Use 'F' to enter a visibility query.
Your queries will be saved here.[-]`,
		theme.TagFgDim(),
		theme.TagFg()))

	modal.SetContent(text)
	modal.SetHints([]components.KeyHint{
		{Key: "Esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal("saved-filters")
	})

	wl.app.JigApp().Pages().AddPage("saved-filters", modal, true, true)
	wl.app.JigApp().SetFocus(modal)
}

func (wl *WorkflowList) showSaveFilter() {
	if wl.visibilityQuery == "" {
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Save Filter", theme.IconInfo),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("name", "Filter Name", "")

	queryText := tview.NewTextView().SetDynamicColors(true)
	queryText.SetBackgroundColor(theme.Bg())
	queryText.SetText(fmt.Sprintf("[%s]Query:[-] %s", theme.TagFgDim(), wl.visibilityQuery))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(queryText, 2, 0, false).
		AddItem(form, 0, 1, true)
	content.SetBackgroundColor(theme.Bg())

	form.SetOnSubmit(func(values map[string]any) {
		// For now, just add to history (persistent save would require config storage)
		wl.addToHistory(wl.visibilityQuery)
		wl.closeModal("save-filter")
	})
	form.SetOnCancel(func() {
		wl.closeModal("save-filter")
	})

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		wl.addToHistory(wl.visibilityQuery)
		wl.closeModal("save-filter")
	})
	modal.SetOnCancel(func() {
		wl.closeModal("save-filter")
	})

	wl.app.JigApp().Pages().AddPage("save-filter", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) clearVisibilityQuery() {
	wl.visibilityQuery = ""
	wl.updatePanelTitle()
	wl.loadData()
	wl.app.JigApp().Menu().SetHints(wl.Hints())
}

func (wl *WorkflowList) updatePanelTitle() {
	title := fmt.Sprintf("%s Workflows", theme.IconWorkflow)
	if wl.visibilityQuery != "" {
		q := wl.visibilityQuery
		if len(q) > 40 {
			q = q[:37] + "..."
		}
		title = fmt.Sprintf("%s Workflows [%s](%s)[-]", theme.IconWorkflow, theme.TagAccent(), q)
	} else if wl.filterText != "" {
		title = fmt.Sprintf("%s Workflows [%s](/%s)[-]", theme.IconWorkflow, theme.TagFgDim(), wl.filterText)
	}
	wl.leftPanel.SetTitle(title)
}

// Diff methods
func (wl *WorkflowList) startDiff() {
	row := wl.table.SelectedRow()
	if row < 0 || row >= len(wl.workflows) {
		wl.app.NavigateToWorkflowDiffEmpty()
		return
	}

	wf := wl.workflows[row]
	wl.app.NavigateToWorkflowDiff(&wf, nil)
}

// Helper functions moved from ui package
func resolveTimePlaceholders(query string) string {
	// Simple placeholder resolution - can be expanded
	return query
}

func copyToClipboard(text string) error {
	// Use OS-specific clipboard commands
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
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

// showSignalWithStart displays a modal for SignalWithStart operation.
func (wl *WorkflowList) showSignalWithStart() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Signal With Start (%s)", theme.IconInfo, wl.namespace),
		Width:    70,
		Height:   20,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("workflowId", "Workflow ID", "")
	form.AddTextField("workflowType", "Workflow Type", "")
	form.AddTextField("taskQueue", "Task Queue", "")
	form.AddTextField("signalName", "Signal Name", "")
	form.AddTextField("signalInput", "Signal Input (JSON, optional)", "")
	form.AddTextField("workflowInput", "Workflow Input (JSON, optional)", "")
	form.SetOnSubmit(func(values map[string]any) {
		workflowID := values["workflowId"].(string)
		workflowType := values["workflowType"].(string)
		taskQueue := values["taskQueue"].(string)
		signalName := values["signalName"].(string)
		signalInput := values["signalInput"].(string)
		workflowInput := values["workflowInput"].(string)

		// Validate required fields
		if workflowID == "" || workflowType == "" || taskQueue == "" || signalName == "" {
			return
		}

		wl.closeModal("signal-with-start")
		wl.executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	form.SetOnCancel(func() {
		wl.closeModal("signal-with-start")
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Execute"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		workflowID := values["workflowId"].(string)
		workflowType := values["workflowType"].(string)
		taskQueue := values["taskQueue"].(string)
		signalName := values["signalName"].(string)
		signalInput := values["signalInput"].(string)
		workflowInput := values["workflowInput"].(string)

		if workflowID == "" || workflowType == "" || taskQueue == "" || signalName == "" {
			return
		}

		wl.closeModal("signal-with-start")
		wl.executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	modal.SetOnCancel(func() {
		wl.closeModal("signal-with-start")
	})

	wl.app.JigApp().Pages().AddPage("signal-with-start", modal, true, true)
	wl.app.JigApp().SetFocus(form)
}

// executeSignalWithStart performs the SignalWithStart operation asynchronously.
func (wl *WorkflowList) executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := temporal.SignalWithStartRequest{
			WorkflowID:   workflowID,
			WorkflowType: workflowType,
			TaskQueue:    taskQueue,
			SignalName:   signalName,
		}

		if signalInput != "" {
			req.SignalInput = []byte(signalInput)
		}
		if workflowInput != "" {
			req.WorkflowInput = []byte(workflowInput)
		}

		runID, err := provider.SignalWithStartWorkflow(ctx, wl.namespace, req)

		wl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(wl.app.JigApp(), "SignalWithStart Failed", err.Error())
				return
			}

			ShowInfoModal(wl.app.JigApp(), "SignalWithStart Successful",
				fmt.Sprintf("Workflow: %s\nRun ID: %s", workflowID, runID))
			wl.loadData() // Refresh the workflow list
		})
	}()
}
