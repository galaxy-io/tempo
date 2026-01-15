package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/input"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// taskQueueEntry represents a task queue in the list.
type taskQueueEntry struct {
	Name        string
	Type        string
	PollerCount int
	Backlog     int
}

// TaskQueueView displays task queue information.
type TaskQueueView struct {
	*tview.Flex
	app            *App
	queueTable     *components.Table
	pollerTable    *components.Table
	queuePanel     *components.Panel
	pollerPanel    *components.Panel
	queues         []taskQueueEntry
	pollers        []temporal.Poller
	selectedQueue  string
	loading        bool
	suppressSelect bool // Prevent recursive selection handling
}

// NewTaskQueueView creates a new task queue view.
func NewTaskQueueView(app *App) *TaskQueueView {
	tq := &TaskQueueView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexColumn),
		app:         app,
		queueTable:  components.NewTable(),
		pollerTable: components.NewTable(),
		queues:      []taskQueueEntry{},
		pollers:     []temporal.Poller{},
	}
	tq.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(tq)

	return tq
}

func (tq *TaskQueueView) setup() {
	tq.SetBackgroundColor(theme.Bg())

	// Task queues table
	tq.queueTable.SetHeaders("NAME", "TYPE", "POLLERS", "BACKLOG")
	tq.queueTable.SetBorder(false)
	tq.queueTable.SetBackgroundColor(theme.Bg())

	// Pollers table
	tq.pollerTable.SetHeaders("IDENTITY", "TYPE", "LAST ACCESS")
	tq.pollerTable.SetBorder(false)
	tq.pollerTable.SetBackgroundColor(theme.Bg())

	// Create panels with icons (blubber pattern)
	tq.queuePanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Task Queues", theme.IconTaskQueue))
	tq.queuePanel.SetContent(tq.queueTable)

	tq.pollerPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Pollers", theme.IconActivity))
	tq.pollerPanel.SetContent(tq.pollerTable)

	// Update pollers when queue selection changes
	tq.queueTable.SetSelectionChangedFunc(func(row, col int) {
		// Skip if we're suppressing selection events (during programmatic updates)
		if tq.suppressSelect {
			return
		}
		if row > 0 && row-1 < len(tq.queues) {
			tq.loadPollers(row - 1)
		}
	})

	// Two-column layout
	tq.AddItem(tq.queuePanel, 0, 1, true)
	tq.AddItem(tq.pollerPanel, 0, 1, false)
}

func (tq *TaskQueueView) setLoading(loading bool) {
	tq.loading = loading
}

// RefreshTheme updates all component colors after a theme change.
func (tq *TaskQueueView) RefreshTheme() {
	bg := theme.Bg()

	// Update main container
	tq.SetBackgroundColor(bg)

	// Update tables
	tq.queueTable.SetBackgroundColor(bg)
	tq.pollerTable.SetBackgroundColor(bg)

	// Re-render tables with new theme colors
	tq.populateQueueTable()
	if len(tq.queues) > 0 && tq.queueTable.SelectedRow() >= 0 {
		tq.populatePollerTable(tq.queues[tq.queueTable.SelectedRow()].Type)
	}
}

func (tq *TaskQueueView) loadData() {
	provider := tq.app.Provider()
	if provider == nil {
		tq.loadMockQueues()
		return
	}

	// Get task queues by listing workflows and extracting unique queue names
	tq.setLoading(true)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// List workflows to discover task queues
		workflows, _, err := provider.ListWorkflows(ctx, tq.app.CurrentNamespace(), temporal.ListOptions{PageSize: 100})

		tq.app.JigApp().QueueUpdateDraw(func() {
			tq.setLoading(false)
			if err != nil {
				tq.showQueueError(err)
				return
			}

			// Extract unique task queue names
			queueSet := make(map[string]bool)
			for _, wf := range workflows {
				if wf.TaskQueue != "" {
					queueSet[wf.TaskQueue] = true
				}
			}

			// Build queue entries
			tq.queues = []taskQueueEntry{}
			for name := range queueSet {
				tq.queues = append(tq.queues, taskQueueEntry{
					Name:        name,
					Type:        "Combined",
					PollerCount: 0,
					Backlog:     0,
				})
			}

			if len(tq.queues) == 0 {
				tq.queues = append(tq.queues, taskQueueEntry{
					Name:        "(no task queues found)",
					Type:        "-",
					PollerCount: 0,
					Backlog:     0,
				})
			}

			tq.populateQueueTable()

			// Load details for first queue
			if len(tq.queues) > 0 && tq.queues[0].Name != "(no task queues found)" {
				tq.loadPollers(0)
			}
		})
	}()
}

func (tq *TaskQueueView) showQueueError(err error) {
	tq.queueTable.ClearRows()
	tq.queueTable.SetHeaders("NAME", "TYPE", "POLLERS", "BACKLOG")
	tq.queueTable.AddRowWithColor(theme.Error(),
		"Error loading task queues",
		err.Error(),
		"",
		"",
	)
}

func (tq *TaskQueueView) loadMockQueues() {
	tq.queues = []taskQueueEntry{
		{Name: "order-tasks", Type: "Combined", PollerCount: 5, Backlog: 12},
		{Name: "payment-tasks", Type: "Combined", PollerCount: 3, Backlog: 0},
		{Name: "shipment-tasks", Type: "Combined", PollerCount: 2, Backlog: 5},
		{Name: "notification-tasks", Type: "Combined", PollerCount: 2, Backlog: 0},
	}
	tq.populateQueueTable()
}

func (tq *TaskQueueView) populateQueueTable() {
	// Preserve current selection
	currentRow := tq.queueTable.SelectedRow()

	tq.queueTable.ClearRows()
	tq.queueTable.SetHeaders("NAME", "TYPE", "POLLERS", "BACKLOG")

	for _, q := range tq.queues {
		backlogIcon := theme.IconCompleted
		backlogColor := temporal.StatusCompleted.Color()
		if q.Backlog > 50 {
			backlogIcon = theme.IconError
			backlogColor = temporal.StatusFailed.Color()
		} else if q.Backlog > 10 {
			backlogIcon = theme.IconRunning
			backlogColor = temporal.StatusRunning.Color()
		}

		typeIcon := theme.IconWorkflow
		if q.Type == "Activity" {
			typeIcon = theme.IconActivity
		}

		// Track row position before adding
		tableRow := tq.queueTable.Table.GetRowCount()
		tq.queueTable.AddRow(
			theme.IconTaskQueue+" "+q.Name,
			typeIcon+" "+q.Type,
			fmt.Sprintf("%d", q.PollerCount),
			fmt.Sprintf("%s %d", backlogIcon, q.Backlog),
		)
		// Color the backlog cell
		cell := tq.queueTable.GetCell(tableRow, 3)
		cell.SetTextColor(backlogColor)
	}

	if tq.queueTable.RowCount() > 0 {
		// Only manage suppressSelect if it's not already being managed by caller
		wasSuppress := tq.suppressSelect
		if !wasSuppress {
			tq.suppressSelect = true
		}
		// Restore previous selection if valid, otherwise select first row
		if currentRow >= 0 && currentRow < len(tq.queues) {
			tq.queueTable.SelectRow(currentRow)
		} else {
			tq.queueTable.SelectRow(0)
		}
		if !wasSuppress {
			tq.suppressSelect = false
		}
	}
}

func (tq *TaskQueueView) loadPollers(queueIndex int) {
	if queueIndex < 0 || queueIndex >= len(tq.queues) {
		return
	}

	queue := tq.queues[queueIndex]
	tq.selectedQueue = queue.Name

	provider := tq.app.Provider()
	if provider == nil {
		tq.loadMockPollers(queue)
		return
	}

	// Load pollers from provider
	tq.pollerTable.ClearRows()
	tq.pollerTable.SetHeaders("IDENTITY", "TYPE", "LAST ACCESS")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		info, pollers, err := provider.DescribeTaskQueue(ctx, tq.app.CurrentNamespace(), queue.Name)

		tq.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				tq.showPollerError(err)
				return
			}

			// Update queue info if we got real data
			if info != nil {
				tq.updateQueueInfo(queueIndex, info)
			}

			tq.pollers = pollers
			tq.populatePollerTable("")
		})
	}()
}

func (tq *TaskQueueView) updateQueueInfo(queueIndex int, info *temporal.TaskQueueInfo) {
	if queueIndex < 0 || queueIndex >= len(tq.queues) {
		return
	}
	// Update the queue entry with real data
	tq.queues[queueIndex].PollerCount = info.PollerCount
	tq.queues[queueIndex].Backlog = info.Backlog
	// Suppress selection events during table refresh to avoid recursive loop
	tq.suppressSelect = true
	// Refresh the queue table display
	tq.populateQueueTable()
	// Reselect the current row
	tq.queueTable.SelectRow(queueIndex)
	tq.suppressSelect = false
}

func (tq *TaskQueueView) loadMockPollers(queue taskQueueEntry) {
	now := time.Now()
	tq.pollers = []temporal.Poller{
		{Identity: "worker-1@host-001", LastAccessTime: now.Add(-5 * time.Second), TaskQueueType: "Workflow"},
		{Identity: "worker-1@host-001", LastAccessTime: now.Add(-3 * time.Second), TaskQueueType: "Activity"},
		{Identity: "worker-2@host-002", LastAccessTime: now.Add(-10 * time.Second), TaskQueueType: "Workflow"},
		{Identity: "worker-2@host-002", LastAccessTime: now.Add(-2 * time.Second), TaskQueueType: "Activity"},
		{Identity: "worker-3@host-003", LastAccessTime: now.Add(-1 * time.Second), TaskQueueType: "Activity"},
	}
	tq.populatePollerTable("")
}

func (tq *TaskQueueView) populatePollerTable(queueType string) {
	tq.pollerTable.ClearRows()
	tq.pollerTable.SetHeaders("IDENTITY", "TYPE", "LAST ACCESS")

	now := time.Now()
	for _, p := range tq.pollers {
		// Filter by queue type if specified
		if queueType != "" && p.TaskQueueType != queueType {
			continue
		}

		typeIcon := theme.IconWorkflow
		if p.TaskQueueType == "Activity" {
			typeIcon = theme.IconActivity
		}

		lastAccess := formatRelativeTime(now, p.LastAccessTime)
		tq.pollerTable.AddRow(
			theme.IconConnected+" "+p.Identity,
			typeIcon+" "+p.TaskQueueType,
			lastAccess,
		)
	}
}

func (tq *TaskQueueView) showPollerError(err error) {
	tq.pollerTable.ClearRows()
	tq.pollerTable.SetHeaders("IDENTITY", "TYPE", "LAST ACCESS")
	tq.pollerTable.AddRowWithColor(theme.Error(),
		theme.IconError+" Error loading pollers",
		err.Error(),
		"",
	)
}

func (tq *TaskQueueView) refreshCurrentQueue() {
	row := tq.queueTable.SelectedRow()
	if row >= 0 && row < len(tq.queues) {
		tq.loadPollers(row)
	}
}

// Name returns the view name.
func (tq *TaskQueueView) Name() string {
	return "task-queues"
}

// Start is called when the view becomes active.
func (tq *TaskQueueView) Start() {
	queueBindings := input.NewKeyBindings().
		On(tcell.KeyTab, func(e *tcell.EventKey) bool {
			tq.app.JigApp().SetFocus(tq.pollerTable)
			return true
		}).
		OnRune('r', func(e *tcell.EventKey) bool {
			tq.refreshCurrentQueue()
			return true
		})

	pollerBindings := input.NewKeyBindings().
		On(tcell.KeyTab, func(e *tcell.EventKey) bool {
			tq.app.JigApp().SetFocus(tq.queueTable)
			return true
		}).
		OnRune('r', func(e *tcell.EventKey) bool {
			tq.refreshCurrentQueue()
			return true
		})

	tq.queueTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if queueBindings.Handle(event) {
			return nil
		}
		return event
	})

	tq.pollerTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if pollerBindings.Handle(event) {
			return nil
		}
		return event
	})

	// Load data when view becomes active
	tq.loadData()
}

// Stop is called when the view is deactivated.
func (tq *TaskQueueView) Stop() {
	tq.queueTable.SetInputCapture(nil)
	tq.pollerTable.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (tq *TaskQueueView) Hints() []KeyHint {
	return []KeyHint{
		{Key: "r", Description: "Refresh"},
		{Key: "tab", Description: "Switch Panel"},
		{Key: "j/k", Description: "Navigate"},
		{Key: "T", Description: "Theme"},
		{Key: "esc", Description: "Back"},
	}
}

// Focus sets focus to the queue table.
func (tq *TaskQueueView) Focus(delegate func(p tview.Primitive)) {
	delegate(tq.queueTable)
}

// Draw applies theme colors dynamically and draws the view.
func (tq *TaskQueueView) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	tq.SetBackgroundColor(bg)
	tq.Flex.Draw(screen)
}
