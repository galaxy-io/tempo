package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
)

// showSignalWithStart displays a modal for SignalWithStart operation.
func (wl *WorkflowList) showSignalWithStart() {
	form := components.NewFormBuilder().
		Text("workflowId", "Workflow ID").
			Placeholder("Enter workflow ID").
			Validate(validators.Required()).
			Done().
		Text("workflowType", "Workflow Type").
			Placeholder("Enter workflow type").
			Validate(validators.Required()).
			Done().
		Text("taskQueue", "Task Queue").
			Placeholder("Enter task queue").
			Validate(validators.Required()).
			Done().
		Text("signalName", "Signal Name").
			Placeholder("Enter signal name").
			Validate(validators.Required()).
			Done().
		Text("signalInput", "Signal Input (JSON, optional)").
			Placeholder("{}").
			Done().
		Text("workflowInput", "Workflow Input (JSON, optional)").
			Placeholder("{}").
			Done().
		OnSubmit(func(values map[string]any) {
			workflowID := values["workflowId"].(string)
			workflowType := values["workflowType"].(string)
			taskQueue := values["taskQueue"].(string)
			signalName := values["signalName"].(string)
			signalInput := values["signalInput"].(string)
			workflowInput := values["workflowInput"].(string)

			wl.closeModal()
			wl.executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
		}).
		OnCancel(func() {
			wl.closeModal()
		}).
		Build()

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Signal With Start (%s)", theme.IconInfo, wl.namespace),
		Width:    70,
		Height:   20,
		Backdrop: true,
	})
	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Ctrl+S", Description: "Execute"},
		{Key: "Esc", Description: "Cancel"},
	})

	wl.app.JigApp().Pages().Push(modal)
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

// startDiff initiates workflow diff view.
func (wl *WorkflowList) startDiff() {
	// Check if we have 2 selected workflows in selection mode
	selected := wl.table.GetSelectedRows()
	if len(selected) == 2 {
		if selected[0] < len(wl.workflows) && selected[1] < len(wl.workflows) {
			wfA := wl.workflows[selected[0]]
			wfB := wl.workflows[selected[1]]
			wl.app.NavigateToWorkflowDiff(&wfA, &wfB)
			return
		}
	}

	// Fall back to single workflow (left side only)
	row := wl.table.SelectedRow()
	if row < 0 || row >= len(wl.workflows) {
		wl.app.NavigateToWorkflowDiffEmpty()
		return
	}

	wf := wl.workflows[row]
	wl.app.NavigateToWorkflowDiff(&wf, nil)
}
