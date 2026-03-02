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

// startWorkflowPrefill holds the pre-fill values for the start workflow modal.
type startWorkflowPrefill struct {
	WorkflowID   string
	WorkflowType string
	TaskQueue    string
	Input        string
}

// showStartWorkflowModal displays the start workflow form and executes it on submit.
func showStartWorkflowModal(app *App, prefill startWorkflowPrefill) {
	form := components.NewFormBuilder().
		Text("workflowId", "Workflow ID").
			Placeholder("Enter workflow ID").
			Value(prefill.WorkflowID).
			Validate(validators.Required()).
			Done().
		Text("workflowType", "Workflow Type").
			Placeholder("Enter workflow type").
			Value(prefill.WorkflowType).
			Validate(validators.Required()).
			Done().
		Text("taskQueue", "Task Queue").
			Placeholder("Enter task queue").
			Value(prefill.TaskQueue).
			Validate(validators.Required()).
			Done().
		Text("input", "Input (JSON, optional)").
			Placeholder("{}").
			Value(prefill.Input).
			Done().
		OnSubmit(func(values map[string]any) {
			workflowID := values["workflowId"].(string)
			workflowType := values["workflowType"].(string)
			taskQueue := values["taskQueue"].(string)
			input := values["input"].(string)

			app.JigApp().Pages().DismissModal()
			executeStartWorkflow(app, workflowID, workflowType, taskQueue, input)
		}).
		OnCancel(func() {
			app.JigApp().Pages().DismissModal()
		}).
		Build()

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Start Workflow", theme.IconInfo),
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

	app.JigApp().Pages().Push(modal)
	app.JigApp().SetFocus(form)
}

// executeStartWorkflow performs the StartWorkflow operation asynchronously.
func executeStartWorkflow(app *App, workflowID, workflowType, taskQueue, input string) {
	provider := app.Provider()
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

		runID, err := provider.StartWorkflow(ctx, app.CurrentNamespace(), req)

		app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(app.JigApp(), "Start Workflow Failed", err.Error())
				return
			}

			app.ShowToastSuccess(fmt.Sprintf("Workflow %s started", workflowID))
			app.NavigateToWorkflowDetail(workflowID, runID)
		})
	}()
}

// showStartWorkflow displays the start workflow modal pre-filled from the selected workflow.
func (wl *WorkflowList) showStartWorkflow() {
	row := wl.table.SelectedRow()

	var prefill startWorkflowPrefill
	if row >= 0 && row < len(wl.workflows) {
		wf := wl.workflows[row]
		prefill = startWorkflowPrefill{
			WorkflowID:   wf.ID,
			WorkflowType: wf.Type,
			TaskQueue:    wf.TaskQueue,
			Input:        wf.Input,
		}
	}

	showStartWorkflowModal(wl.app, prefill)
}
