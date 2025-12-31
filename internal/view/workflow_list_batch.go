package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/rivo/tview"
)

// Selection mode methods

func (wl *WorkflowList) toggleSelectionMode() {
	wl.selectionMode = !wl.selectionMode
	if wl.selectionMode {
		wl.table.SetMultiSelect(true)
		wl.SetMasterTitle(fmt.Sprintf("%s Workflows (Select Mode)", theme.IconWorkflow))
	} else {
		wl.table.SetMultiSelect(false)
		wl.table.ClearSelection()
		wl.SetMasterTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow))
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
			temporal.StatusRunning.ColorTag(), temporal.StatusRunning.Icon(), running,
			temporal.StatusCompleted.ColorTag(), temporal.StatusCompleted.Icon(), completed,
			temporal.StatusFailed.ColorTag(), temporal.StatusFailed.Icon(), failed,
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

	form := components.NewFormBuilder().
		Text("reason", "Reason (optional)").
			Value("Batch cancelled via tempo").
			Done().
		OnSubmit(func(values map[string]any) {
			reason := values["reason"].(string)
			wl.closeModal()
			wl.executeBatchCancel(selected, reason)
		}).
		OnCancel(func() {
			wl.closeModal()
		}).
		Build()

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

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Cancel %d Workflow(s)", theme.IconWarning, len(selected)),
		Width:    60,
		Height:   14,
		Backdrop: true,
	})
	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Ctrl+S", Description: "Confirm"},
		{Key: "Esc", Description: "Cancel"},
	})

	wl.app.JigApp().Pages().Push(modal)
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

	form := components.NewFormBuilder().
		Text("reason", "Reason (required)").
			Placeholder("Enter reason for termination").
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			reason := values["reason"].(string)
			wl.closeModal()
			wl.executeBatchTerminate(selected, reason)
		}).
		OnCancel(func() {
			wl.closeModal()
		}).
		Build()

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

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Terminate %d Workflow(s)", theme.IconError, len(selected)),
		Width:    65,
		Height:   16,
		Backdrop: true,
	})
	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Ctrl+S", Description: "Terminate"},
		{Key: "Esc", Description: "Cancel"},
	})

	wl.app.JigApp().Pages().Push(modal)
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

func (wl *WorkflowList) closeModal() {
	wl.app.JigApp().Pages().DismissModal()
}
