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

// showStartWorkflow displays a modal for starting a new workflow execution.
func (wl *WorkflowList) showStartWorkflow() {
	row := wl.table.SelectedRow()

	var prefillID, prefillType, prefillQueue, prefillInput string
	if row >= 0 && row < len(wl.workflows) {
		wf := wl.workflows[row]
		prefillID = wf.ID
		prefillType = wf.Type
		prefillQueue = wf.TaskQueue
		prefillInput = wf.Input
	}

	form := components.NewFormBuilder().
		Text("workflowId", "Workflow ID").
			Placeholder("Enter workflow ID").
			Value(prefillID).
			Validate(validators.Required()).
			Done().
		Text("workflowType", "Workflow Type").
			Placeholder("Enter workflow type").
			Value(prefillType).
			Validate(validators.Required()).
			Done().
		Text("taskQueue", "Task Queue").
			Placeholder("Enter task queue").
			Value(prefillQueue).
			Validate(validators.Required()).
			Done().
		Text("input", "Input (JSON, optional)").
			Placeholder("{}").
			Value(prefillInput).
			Done().
		OnSubmit(func(values map[string]any) {
			workflowID := values["workflowId"].(string)
			workflowType := values["workflowType"].(string)
			taskQueue := values["taskQueue"].(string)
			input := values["input"].(string)

			wl.closeModal()
			wl.executeStartWorkflow(workflowID, workflowType, taskQueue, input)
		}).
		OnCancel(func() {
			wl.closeModal()
		}).
		Build()

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Start Workflow (%s)", theme.IconInfo, wl.namespace),
		Width:    70,
		Height:   18,
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

// executeStartWorkflow performs the StartWorkflow operation asynchronously.
func (wl *WorkflowList) executeStartWorkflow(workflowID, workflowType, taskQueue, input string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := temporal.StartWorkflowRequest{
			WorkflowID:   workflowID,
			WorkflowType: workflowType,
			TaskQueue:    taskQueue,
		}

		if input != "" {
			req.Input = []byte(input)
		}

		runID, err := provider.StartWorkflow(ctx, wl.namespace, req)

		wl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(wl.app.JigApp(), "Start Workflow Failed", err.Error())
				return
			}

			wl.app.ShowToastSuccess(fmt.Sprintf("Workflow %s started", workflowID))
			wl.app.NavigateToWorkflowDetail(workflowID, runID)
		})
	}()
}
