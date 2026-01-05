package view

import (
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowList displays a list of workflows with a preview panel.
type WorkflowList struct {
	*components.MasterDetailView
	app              *App
	namespace        string
	table            *components.Table
	preview          *tview.TextView
	emptyState       *components.EmptyState
	noResultsState   *components.EmptyState
	allWorkflows     []temporal.Workflow // Full unfiltered list
	workflows        []temporal.Workflow // Filtered list for display
	filterText       string
	visibilityQuery  string // Temporal visibility query
	loading          bool
	autoRefresh      bool
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
		app:            app,
		namespace:      namespace,
		table:          components.NewTable(),
		preview:        tview.NewTextView(),
		workflows:      []temporal.Workflow{},
		stopRefresh:    make(chan struct{}, 1), // Buffered to ensure stop signal isn't lost
		searchHistory:  make([]string, 0, 50),
		historyIndex:   -1,
		maxHistorySize: 50,
	}
	wl.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(wl)

	return wl
}

func (wl *WorkflowList) setup() {
	wl.table.SetHeaders("WORKFLOW ID", "STATUS", "TYPE", "START TIME")
	wl.table.SetBorder(false)
	wl.table.SetBackgroundColor(theme.Bg())

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

	// Create MasterDetailView
	wl.MasterDetailView = components.NewMasterDetailView().
		SetMasterTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow)).
		SetDetailTitle(fmt.Sprintf("%s Preview", theme.IconInfo)).
		SetMasterContent(wl.table).
		SetDetailContent(wl.preview).
		SetRatio(0.6).
		ConfigureEmpty(theme.IconInfo, "No Selection", "Select a workflow to view details")

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
}

func (wl *WorkflowList) togglePreview() {
	wl.ToggleDetail()
	// Repopulate table to recalculate column widths for new layout
	wl.populateTable()
}

// RefreshTheme updates all component colors after a theme change.
func (wl *WorkflowList) RefreshTheme() {
	bg := theme.Bg()

	// Update table
	wl.table.SetBackgroundColor(bg)

	// Update preview
	wl.preview.SetBackgroundColor(bg)
	wl.preview.SetTextColor(theme.Fg())

	// Re-render table with new theme colors
	wl.populateTable()
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
		delegate(wl.MasterDetailView)
		return
	}
	delegate(wl.table)
}

// Draw applies theme colors dynamically and draws the view.
func (wl *WorkflowList) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	wl.preview.SetBackgroundColor(bg)
	wl.preview.SetTextColor(theme.Fg())
	wl.MasterDetailView.Draw(screen)
}
