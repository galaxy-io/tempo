package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
)

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
		resolvedQuery, err := resolveTimePlaceholders(wl.visibilityQuery)
		if err != nil {
			wl.app.ShowToastError(fmt.Sprintf("Invalid query: %v", err))
			wl.app.JigApp().QueueUpdateDraw(func() {
				wl.setLoading(false)
			})
			return
		}
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

func (wl *WorkflowList) populateTable() {
	currentRow := wl.table.SelectedRow()

	wl.table.ClearRows()
	wl.table.SetHeaders("WORKFLOW ID", "STATUS", "TYPE", "START TIME")

	if len(wl.workflows) == 0 {
		if len(wl.allWorkflows) == 0 {
			wl.SetMasterContent(wl.emptyState)
		} else {
			wl.SetMasterContent(wl.noResultsState)
		}
		wl.preview.SetText("")
		return
	}

	wl.SetMasterContent(wl.table)

	// Calculate dynamic column widths based on available space
	idWidth, typeWidth := wl.calculateColumnWidths()

	now := time.Now()
	for _, w := range wl.workflows {
		statusHandle := temporal.GetWorkflowStatus(w.Status)
		wl.table.AddRowWithStatus(statusHandle, 1, // status column is index 1
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

func (wl *WorkflowList) updatePreview(w temporal.Workflow) {
	now := time.Now()
	statusHandle := temporal.GetWorkflowStatus(w.Status)
	statusColor := statusHandle.ColorTag()
	statusIcon := statusHandle.Icon()

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

func (wl *WorkflowList) updateStats() {
	var running, completed, failed int
	for _, w := range wl.workflows {
		switch w.Status {
		case "Running":
			running++
		case "Completed":
			completed++
		case "Failed":
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

// calculateColumnWidths determines optimal column widths based on available space.
// Returns (idWidth, typeWidth) where 0 means no truncation needed.
func (wl *WorkflowList) calculateColumnWidths() (int, int) {
	// Calculate width based on parent and preview state
	_, _, totalWidth, _ := wl.MasterDetailView.GetInnerRect()

	var width int
	if totalWidth > 0 {
		if wl.IsDetailVisible() {
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

// Auto-refresh methods

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
