package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
)

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

		wl.closeModal()
		wl.executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	form.SetOnCancel(func() {
		wl.closeModal()
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

		wl.closeModal()
		wl.executeSignalWithStart(workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
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
	row := wl.table.SelectedRow()
	if row < 0 || row >= len(wl.workflows) {
		wl.app.NavigateToWorkflowDiffEmpty()
		return
	}

	wf := wl.workflows[row]
	wl.app.NavigateToWorkflowDiff(&wf, nil)
}
