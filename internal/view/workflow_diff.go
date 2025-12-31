package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowDiff displays a side-by-side comparison of two workflows.
type WorkflowDiff struct {
	*tview.Flex
	app       *App
	namespace string

	// Workflow data
	workflowA *temporal.Workflow
	workflowB *temporal.Workflow
	eventsA   []temporal.HistoryEvent
	eventsB   []temporal.HistoryEvent

	// UI components
	leftPanel   *components.Panel
	rightPanel  *components.Panel
	leftInfo    *tview.TextView
	rightInfo   *tview.TextView
	leftEvents  *components.Table
	rightEvents *components.Table

	// State
	focusLeft bool
	loading   bool
}

// NewWorkflowDiff creates a new workflow diff view.
func NewWorkflowDiff(app *App, namespace string) *WorkflowDiff {
	wd := &WorkflowDiff{
		Flex:      tview.NewFlex().SetDirection(tview.FlexColumn),
		app:       app,
		namespace: namespace,
		focusLeft: true,
	}
	wd.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(wd)

	return wd
}

// NewWorkflowDiffWithWorkflows creates a diff view with pre-loaded workflows.
func NewWorkflowDiffWithWorkflows(app *App, namespace string, workflowA, workflowB *temporal.Workflow) *WorkflowDiff {
	wd := NewWorkflowDiff(app, namespace)
	wd.workflowA = workflowA
	wd.workflowB = workflowB
	return wd
}

func (wd *WorkflowDiff) setup() {
	wd.SetBackgroundColor(theme.Bg())

	// Create left side components
	wd.leftInfo = tview.NewTextView().SetDynamicColors(true)
	wd.leftInfo.SetBackgroundColor(theme.Bg())
	wd.leftEvents = components.NewTable()
	wd.leftEvents.SetHeaders("EVENT", "TYPE", "TIME")

	leftContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(wd.leftInfo, 8, 0, false).
		AddItem(wd.leftEvents, 0, 1, true)
	leftContent.SetBackgroundColor(theme.Bg())

	wd.leftPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Workflow A", theme.IconWorkflow))
	wd.leftPanel.SetContent(leftContent)

	// Create right side components
	wd.rightInfo = tview.NewTextView().SetDynamicColors(true)
	wd.rightInfo.SetBackgroundColor(theme.Bg())
	wd.rightEvents = components.NewTable()
	wd.rightEvents.SetHeaders("EVENT", "TYPE", "TIME")

	rightContent := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(wd.rightInfo, 8, 0, false).
		AddItem(wd.rightEvents, 0, 1, true)
	rightContent.SetBackgroundColor(theme.Bg())

	wd.rightPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Workflow B", theme.IconWorkflow))
	wd.rightPanel.SetContent(rightContent)

	// Build layout
	wd.AddItem(wd.leftPanel, 0, 1, true)
	wd.AddItem(wd.rightPanel, 0, 1, false)
}

// Name returns the view name.
func (wd *WorkflowDiff) Name() string {
	return "workflow-diff"
}

// Start is called when the view becomes active.
func (wd *WorkflowDiff) Start() {
	wd.leftEvents.SetInputCapture(wd.inputHandler)
	wd.rightEvents.SetInputCapture(wd.inputHandler)

	// Show empty state or prompt for workflows
	if wd.workflowA == nil && wd.workflowB == nil {
		wd.showEmptyState()
	} else {
		wd.loadData()
	}
}

// Stop is called when the view is deactivated.
func (wd *WorkflowDiff) Stop() {
	wd.leftEvents.SetInputCapture(nil)
	wd.rightEvents.SetInputCapture(nil)
}

// RefreshTheme updates all component colors after a theme change.
func (wd *WorkflowDiff) RefreshTheme() {
	bg := theme.Bg()

	// Update main container
	wd.SetBackgroundColor(bg)

	// Update text views
	wd.leftInfo.SetBackgroundColor(bg)
	wd.rightInfo.SetBackgroundColor(bg)

	// Update tables
	wd.leftEvents.SetBackgroundColor(bg)
	wd.rightEvents.SetBackgroundColor(bg)

	// Re-render content with new theme colors
	wd.updateLeftInfo()
	wd.updateRightInfo()
	wd.updateLeftEvents()
	wd.updateRightEvents()
}

// Hints returns keybinding hints for this view.
func (wd *WorkflowDiff) Hints() []KeyHint {
	return []KeyHint{
		{Key: "Tab", Description: "Switch Panel"},
		{Key: "a", Description: "Set Left"},
		{Key: "b", Description: "Set Right"},
		{Key: "r", Description: "Refresh"},
		{Key: "esc", Description: "Back"},
	}
}

// Focus sets focus to the current panel.
func (wd *WorkflowDiff) Focus(delegate func(p tview.Primitive)) {
	if wd.focusLeft {
		delegate(wd.leftEvents)
	} else {
		delegate(wd.rightEvents)
	}
}

// Draw applies theme colors dynamically and draws the view.
func (wd *WorkflowDiff) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	wd.SetBackgroundColor(bg)
	wd.leftInfo.SetBackgroundColor(bg)
	wd.rightInfo.SetBackgroundColor(bg)
	wd.Flex.Draw(screen)
}

func (wd *WorkflowDiff) inputHandler(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		wd.toggleFocus()
		return nil
	}

	switch event.Rune() {
	case 'a':
		wd.promptWorkflowInput(true)
		return nil
	case 'b':
		wd.promptWorkflowInput(false)
		return nil
	case 'r':
		wd.loadData()
		return nil
	}

	return event
}

func (wd *WorkflowDiff) toggleFocus() {
	wd.focusLeft = !wd.focusLeft
	if wd.focusLeft {
		wd.app.JigApp().SetFocus(wd.leftEvents)
		wd.leftPanel.SetBorderColor(theme.Accent())
		wd.rightPanel.SetBorderColor(theme.PanelBorder())
	} else {
		wd.app.JigApp().SetFocus(wd.rightEvents)
		wd.rightPanel.SetBorderColor(theme.Accent())
		wd.leftPanel.SetBorderColor(theme.PanelBorder())
	}
}

func (wd *WorkflowDiff) showEmptyState() {
	emptyText := fmt.Sprintf(`[%s::b]Workflow Comparison[-:-:-]

[%s]No workflows selected for comparison.[-]

[%s]Press 'a' to set the left workflow
Press 'b' to set the right workflow[-]`,
		theme.TagAccent(),
		theme.TagFgDim(),
		theme.TagFg())

	wd.leftInfo.SetText(emptyText)
	wd.rightInfo.SetText("")
	wd.leftEvents.ClearRows()
	wd.rightEvents.ClearRows()
}

func (wd *WorkflowDiff) promptWorkflowInput(isLeft bool) {
	side := "Left"
	if !isLeft {
		side = "Right"
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Set %s Workflow", theme.IconWorkflow, side),
		Width:    70,
		Height:   14,
		Backdrop: true,
	})

	form := components.NewFormBuilder().
		Text("workflowID", "Workflow ID").
			Placeholder("Enter workflow ID").
			Validate(validators.Required()).
			Done().
		Text("runID", "Run ID (optional)").
			Placeholder("Leave empty for latest run").
			Done().
		OnSubmit(func(values map[string]any) {
			workflowID := values["workflowID"].(string)
			runID := values["runID"].(string)
			wd.closeModal()
			wd.loadWorkflow(isLeft, workflowID, runID)
		}).
		OnCancel(func() {
			wd.closeModal()
		}).
		Build()

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Load workflow"},
		{Key: "Esc", Description: "Cancel"},
	})

	wd.app.JigApp().Pages().Push(modal)
	wd.app.JigApp().SetFocus(form)
}

func (wd *WorkflowDiff) closeModal() {
	wd.app.JigApp().Pages().DismissModal()
}

func (wd *WorkflowDiff) loadWorkflow(isLeft bool, workflowID, runID string) {
	provider := wd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		workflow, err := provider.GetWorkflow(ctx, wd.namespace, workflowID, runID)
		if err != nil {
			wd.app.JigApp().QueueUpdateDraw(func() {
				errorText := fmt.Sprintf("[%s]Error: %s[-]", theme.TagError(), err.Error())
				if isLeft {
					wd.leftInfo.SetText(errorText)
				} else {
					wd.rightInfo.SetText(errorText)
				}
			})
			return
		}

		events, _ := provider.GetWorkflowHistory(ctx, wd.namespace, workflow.ID, workflow.RunID)

		wd.app.JigApp().QueueUpdateDraw(func() {
			if isLeft {
				wd.workflowA = workflow
				wd.eventsA = events
				wd.leftPanel.SetTitle(fmt.Sprintf("%s Workflow A: %s", theme.IconWorkflow, truncate(workflow.ID, 25)))
				wd.updateLeftInfo()
				wd.updateLeftEvents()
			} else {
				wd.workflowB = workflow
				wd.eventsB = events
				wd.rightPanel.SetTitle(fmt.Sprintf("%s Workflow B: %s", theme.IconWorkflow, truncate(workflow.ID, 25)))
				wd.updateRightInfo()
				wd.updateRightEvents()
			}
		})
	}()
}

func (wd *WorkflowDiff) loadData() {
	if wd.workflowA != nil {
		wd.loadWorkflow(true, wd.workflowA.ID, wd.workflowA.RunID)
	}
	if wd.workflowB != nil {
		wd.loadWorkflow(false, wd.workflowB.ID, wd.workflowB.RunID)
	}
}

func (wd *WorkflowDiff) updateLeftInfo() {
	if wd.workflowA == nil {
		wd.leftInfo.SetText("")
		return
	}
	wd.leftInfo.SetText(wd.formatWorkflowInfo(wd.workflowA, len(wd.eventsA)))
}

func (wd *WorkflowDiff) updateRightInfo() {
	if wd.workflowB == nil {
		wd.rightInfo.SetText("")
		return
	}
	wd.rightInfo.SetText(wd.formatWorkflowInfo(wd.workflowB, len(wd.eventsB)))
}

func (wd *WorkflowDiff) formatWorkflowInfo(w *temporal.Workflow, eventCount int) string {
	statusHandle := temporal.GetWorkflowStatus(w.Status)
	statusColor := statusHandle.ColorTag()
	statusIcon := statusHandle.Icon()

	duration := "-"
	if w.EndTime != nil {
		duration = w.EndTime.Sub(w.StartTime).Round(time.Second).String()
	} else if w.Status == "Running" {
		duration = time.Since(w.StartTime).Round(time.Second).String() + " (running)"
	}

	return fmt.Sprintf(`[%s]Type:[-] [%s]%s[-]
[%s]Status:[-] [%s]%s %s[-]
[%s]Started:[-] [%s]%s[-]
[%s]Duration:[-] [%s]%s[-]
[%s]Events:[-] [%s]%d[-]
[%s]Task Queue:[-] [%s]%s[-]`,
		theme.TagFgDim(), theme.TagFg(), w.Type,
		theme.TagFgDim(), statusColor, statusIcon, w.Status,
		theme.TagFgDim(), theme.TagFg(), w.StartTime.Format("2006-01-02 15:04:05"),
		theme.TagFgDim(), theme.TagFg(), duration,
		theme.TagFgDim(), theme.TagAccent(), eventCount,
		theme.TagFgDim(), theme.TagFg(), w.TaskQueue)
}

func (wd *WorkflowDiff) updateLeftEvents() {
	wd.leftEvents.ClearRows()
	for _, e := range wd.eventsA {
		wd.leftEvents.AddRow(
			fmt.Sprintf("%d", e.ID),
			e.Type,
			e.Time.Format("15:04:05"),
		)
	}
	if wd.leftEvents.RowCount() > 0 {
		wd.leftEvents.SelectRow(0)
	}
}

func (wd *WorkflowDiff) updateRightEvents() {
	wd.rightEvents.ClearRows()
	for _, e := range wd.eventsB {
		wd.rightEvents.AddRow(
			fmt.Sprintf("%d", e.ID),
			e.Type,
			e.Time.Format("15:04:05"),
		)
	}
	if wd.rightEvents.RowCount() > 0 {
		wd.rightEvents.SelectRow(0)
	}
}

// SetWorkflowA sets the left workflow for comparison.
func (wd *WorkflowDiff) SetWorkflowA(w *temporal.Workflow) {
	wd.workflowA = w
}

// SetWorkflowB sets the right workflow for comparison.
func (wd *WorkflowDiff) SetWorkflowB(w *temporal.Workflow) {
	wd.workflowB = w
}
