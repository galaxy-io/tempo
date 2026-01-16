package view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atterpac/jig/async"
	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/input"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ScheduleList displays a list of schedules with actions.
type ScheduleList struct {
	*components.MasterDetailView
	app          *App
	namespace    string
	table        *components.Table
	preview      *tview.TextView
	allSchedules []temporal.Schedule // Full unfiltered list
	schedules    []temporal.Schedule // Filtered list for display
	loading      bool
}

// NewScheduleList creates a new schedule list view.
func NewScheduleList(app *App, namespace string) *ScheduleList {
	sl := &ScheduleList{
		app:       app,
		namespace: namespace,
		table:     components.NewTable(),
		preview:   tview.NewTextView(),
		schedules: []temporal.Schedule{},
	}
	sl.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(sl)

	return sl
}

func (sl *ScheduleList) setup() {
	sl.table.SetHeaders("SCHEDULE ID", "WORKFLOW TYPE", "SPEC", "STATUS", "NEXT RUN")
	sl.table.SetBorder(false)
	sl.table.SetBackgroundColor(theme.Bg())

	// Configure preview
	sl.preview.SetDynamicColors(true)
	sl.preview.SetBackgroundColor(theme.Bg())
	sl.preview.SetTextColor(theme.Fg())
	sl.preview.SetWordWrap(true)

	// Create MasterDetailView
	sl.MasterDetailView = components.NewMasterDetailView().
		SetMasterTitle(fmt.Sprintf("%s Schedules", theme.IconSchedule)).
		SetDetailTitle(fmt.Sprintf("%s Preview", theme.IconInfo)).
		SetMasterContent(sl.table).
		SetDetailContent(sl.preview).
		SetRatio(0.6).
		ConfigureEmpty(theme.IconInfo, "No Selection", "Select a schedule to view details").
		EnableSearch(func(current string, cb components.SearchCallbacks) {
			sl.app.ShowFilterMode(current, FilterModeCallbacks{
				OnChange: cb.OnChange,
				OnSubmit: cb.OnSubmit,
				OnCancel: cb.OnCancel,
			})
		}).
		SetOnSearch(func(query string) {
			sl.applyFilter(query)
		})

	// Selection change handler to update preview
	sl.table.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row-1 < len(sl.schedules) {
			sl.updatePreview(sl.schedules[row-1])
		}
	})
}

func (sl *ScheduleList) togglePreview() {
	sl.ToggleDetail()
}

// RefreshTheme updates all component colors after a theme change.
func (sl *ScheduleList) RefreshTheme() {
	bg := theme.Bg()

	// Update table
	sl.table.SetBackgroundColor(bg)

	// Update preview
	sl.preview.SetBackgroundColor(bg)
	sl.preview.SetTextColor(theme.Fg())

	// Re-render table with new theme colors
	sl.populateTable()
}

func (sl *ScheduleList) updatePreview(s temporal.Schedule) {
	pauseStatus := "Active"
	pauseColor := temporal.StatusCompleted.ColorTag()
	if s.Paused {
		pauseStatus = "Paused"
		pauseColor = temporal.StatusCanceled.ColorTag()
	}

	nextRun := "-"
	if s.NextRunTime != nil {
		nextRun = formatRelativeTime(time.Now(), *s.NextRunTime)
	}

	lastRun := "-"
	if s.LastRunTime != nil {
		lastRun = formatRelativeTime(time.Now(), *s.LastRunTime)
	}

	text := fmt.Sprintf(`[%s::b]Schedule[-:-:-]
[%s]%s[-]

[%s]Status[-]
[%s]%s[-]

[%s]Workflow Type[-]
[%s]%s[-]

[%s]Spec[-]
[%s]%s[-]

[%s]Next Run[-]
[%s]%s[-]

[%s]Last Run[-]
[%s]%s[-]

[%s]Total Actions[-]
[%s]%d[-]

[%s]Notes[-]
[%s]%s[-]`,
		theme.TagAccent(),
		theme.TagFg(), s.ID,
		theme.TagFgDim(),
		pauseColor, pauseStatus,
		theme.TagFgDim(),
		theme.TagFg(), s.WorkflowType,
		theme.TagFgDim(),
		theme.TagFg(), s.Spec,
		theme.TagFgDim(),
		theme.TagFg(), nextRun,
		theme.TagFgDim(),
		theme.TagFg(), lastRun,
		theme.TagFgDim(),
		theme.TagFg(), s.TotalActions,
		theme.TagFgDim(),
		theme.TagFgDim(), s.Notes,
	)
	sl.preview.SetText(text)
}

func (sl *ScheduleList) applyFilter(query string) {
	if query == "" {
		sl.schedules = sl.allSchedules
	} else {
		sl.schedules = nil
		q := strings.ToLower(query)
		for _, s := range sl.allSchedules {
			if strings.Contains(strings.ToLower(s.ID), q) ||
				strings.Contains(strings.ToLower(s.WorkflowType), q) ||
				strings.Contains(strings.ToLower(s.Spec), q) {
				sl.schedules = append(sl.schedules, s)
			}
		}
	}
	sl.populateTable()
}

func (sl *ScheduleList) loadData() {
	provider := sl.app.Provider()
	if provider == nil {
		sl.loadMockData()
		return
	}

	sl.loading = true
	namespace := sl.namespace

	async.NewLoader[[]temporal.Schedule]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(schedules []temporal.Schedule) {
			sl.allSchedules = schedules
			sl.applyFilter(sl.MasterDetailView.GetSearchText())
		}).
		OnError(func(err error) {
			sl.showError(err)
		}).
		OnFinally(func() {
			sl.loading = false
		}).
		Run(func(ctx context.Context) ([]temporal.Schedule, error) {
			schedules, _, err := provider.ListSchedules(ctx, namespace, temporal.ListOptions{PageSize: 100})
			return schedules, err
		})
}

func (sl *ScheduleList) loadMockData() {
	now := time.Now()
	nextRun := now.Add(5 * time.Minute)
	lastRun := now.Add(-1 * time.Hour)
	sl.allSchedules = []temporal.Schedule{
		{
			ID:           "daily-report",
			WorkflowType: "ReportWorkflow",
			Spec:         "0 9 * * *",
			Paused:       false,
			NextRunTime:  &nextRun,
			LastRunTime:  &lastRun,
			TotalActions: 365,
			Notes:        "Daily report generation",
		},
		{
			ID:           "hourly-cleanup",
			WorkflowType: "CleanupWorkflow",
			Spec:         "every 1h",
			Paused:       false,
			NextRunTime:  &nextRun,
			LastRunTime:  &lastRun,
			TotalActions: 2190,
			Notes:        "Hourly cleanup tasks",
		},
		{
			ID:           "weekly-backup",
			WorkflowType: "BackupWorkflow",
			Spec:         "0 0 * * 0",
			Paused:       true,
			NextRunTime:  nil,
			LastRunTime:  &lastRun,
			TotalActions: 52,
			Notes:        "Weekly backups (paused)",
		},
	}
	sl.applyFilter(sl.MasterDetailView.GetSearchText())
}

func (sl *ScheduleList) populateTable() {
	// Preserve current selection
	currentRow := sl.table.SelectedRow()

	sl.table.ClearRows()
	sl.table.SetHeaders("SCHEDULE ID", "WORKFLOW TYPE", "SPEC", "STATUS", "NEXT RUN")

	for _, s := range sl.schedules {
		status := "Active"
		statusColor := temporal.StatusCompleted.Color()
		if s.Paused {
			status = "Paused"
			statusColor = temporal.StatusCanceled.Color()
		}

		nextRun := "-"
		if s.NextRunTime != nil {
			nextRun = formatRelativeTime(time.Now(), *s.NextRunTime)
		}

		sl.table.AddRowWithColor(statusColor,
			truncate(s.ID, 20),
			truncate(s.WorkflowType, 20),
			truncate(s.Spec, 15),
			status,
			nextRun,
		)
	}

	if sl.table.RowCount() > 0 {
		// Restore previous selection if valid, otherwise select first row
		if currentRow >= 0 && currentRow < len(sl.schedules) {
			sl.table.SelectRow(currentRow)
			sl.updatePreview(sl.schedules[currentRow])
		} else {
			sl.table.SelectRow(0)
			if len(sl.schedules) > 0 {
				sl.updatePreview(sl.schedules[0])
			}
		}
	}
}

func (sl *ScheduleList) showError(err error) {
	sl.table.ClearRows()
	sl.table.SetHeaders("SCHEDULE ID", "WORKFLOW TYPE", "SPEC", "STATUS", "NEXT RUN")
	sl.table.AddRowWithColor(theme.Error(),
		theme.IconError+" Error loading schedules",
		err.Error(),
		"",
		"",
		"",
	)
}

func (sl *ScheduleList) getSelectedSchedule() *temporal.Schedule {
	row := sl.table.SelectedRow() // Use SelectedRow() which accounts for header
	if row >= 0 && row < len(sl.schedules) {
		return &sl.schedules[row]
	}
	return nil
}

// Mutation methods - implemented using jig components

func (sl *ScheduleList) showPauseConfirm() {
	schedule := sl.getSelectedSchedule()
	if schedule == nil {
		return
	}

	// If already paused, show unpause confirmation instead
	if schedule.Paused {
		sl.showUnpauseConfirm(schedule)
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Pause Schedule", theme.IconWarning),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf("[%s]Schedule:[-] [%s]%s[-]\n[%s]Workflow:[-] [%s]%s[-]",
		theme.TagFgDim(), theme.TagFg(), schedule.ID,
		theme.TagFgDim(), theme.TagFg(), schedule.WorkflowType))

	form := components.NewFormBuilder().
		Text("reason", "Reason").
			Value("Paused via tempo").
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			reason := values["reason"].(string)
			sl.closeModal()
			sl.executePauseSchedule(schedule.ID, reason)
		}).
		OnCancel(func() {
			sl.closeModal()
		}).
		Build()

	contentFlex.AddItem(infoText, 3, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Pause"},
		{Key: "Esc", Description: "Cancel"},
	})

	sl.app.JigApp().Pages().Push(modal)
	sl.app.JigApp().SetFocus(form)
}

func (sl *ScheduleList) showUnpauseConfirm(s *temporal.Schedule) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Unpause Schedule", theme.IconInfo),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf("[%s]Schedule:[-] [%s]%s[-]\n[%s]Workflow:[-] [%s]%s[-]",
		theme.TagFgDim(), theme.TagFg(), s.ID,
		theme.TagFgDim(), theme.TagFg(), s.WorkflowType))

	form := components.NewFormBuilder().
		Text("reason", "Reason").
			Value("Unpaused via tempo").
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			reason := values["reason"].(string)
			sl.closeModal()
			sl.executeUnpauseSchedule(s.ID, reason)
		}).
		OnCancel(func() {
			sl.closeModal()
		}).
		Build()

	contentFlex.AddItem(infoText, 3, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Unpause"},
		{Key: "Esc", Description: "Cancel"},
	})

	sl.app.JigApp().Pages().Push(modal)
	sl.app.JigApp().SetFocus(form)
}

func (sl *ScheduleList) executePauseSchedule(scheduleID, reason string) {
	provider := sl.app.Provider()
	if provider == nil {
		return
	}

	namespace := sl.namespace
	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			sl.loadData()
		}).
		OnError(func(err error) {
			sl.showError(err)
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.PauseSchedule(ctx, namespace, scheduleID, reason)
		})
}

func (sl *ScheduleList) executeUnpauseSchedule(scheduleID, reason string) {
	provider := sl.app.Provider()
	if provider == nil {
		return
	}

	namespace := sl.namespace
	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			sl.loadData()
		}).
		OnError(func(err error) {
			sl.showError(err)
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.UnpauseSchedule(ctx, namespace, scheduleID, reason)
		})
}

func (sl *ScheduleList) showTriggerConfirm() {
	schedule := sl.getSelectedSchedule()
	if schedule == nil {
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Trigger Schedule", theme.IconSignal),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	infoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	infoText.SetBackgroundColor(theme.Bg())
	infoText.SetText(fmt.Sprintf(`[%s]Trigger schedule immediately?[-]

[%s]Schedule:[-] [%s]%s[-]
[%s]Workflow:[-] [%s]%s[-]`,
		theme.TagAccent(),
		theme.TagFgDim(), theme.TagFg(), schedule.ID,
		theme.TagFgDim(), theme.TagFg(), schedule.WorkflowType))

	contentFlex.AddItem(infoText, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Trigger"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		sl.closeModal()
		sl.executeTriggerSchedule(schedule.ID)
	})
	modal.SetOnCancel(func() {
		sl.closeModal()
	})

	sl.app.JigApp().Pages().Push(modal)
}

func (sl *ScheduleList) executeTriggerSchedule(scheduleID string) {
	provider := sl.app.Provider()
	if provider == nil {
		return
	}

	namespace := sl.namespace
	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			sl.loadData()
		}).
		OnError(func(err error) {
			sl.showError(err)
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.TriggerSchedule(ctx, namespace, scheduleID)
		})
}

func (sl *ScheduleList) showDeleteConfirm() {
	schedule := sl.getSelectedSchedule()
	if schedule == nil {
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Delete Schedule", theme.IconError),
		Width:    65,
		Height:   14,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	warningText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf(`[%s]Warning: This will permanently delete the schedule.
This action cannot be undone.[-]

[%s]Schedule:[-] [%s]%s[-]
[%s]Workflow:[-] [%s]%s[-]`,
		theme.TagError(),
		theme.TagFgDim(), theme.TagFg(), schedule.ID,
		theme.TagFgDim(), theme.TagFg(), schedule.WorkflowType))

	form := components.NewFormBuilder().
		Text("confirm", "Type schedule ID to confirm").
			Placeholder(schedule.ID).
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			confirm := values["confirm"].(string)
			if confirm != schedule.ID {
				return // Must match schedule ID
			}
			sl.closeModal()
			sl.executeDeleteSchedule(schedule.ID)
		}).
		OnCancel(func() {
			sl.closeModal()
		}).
		Build()

	contentFlex.AddItem(warningText, 6, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Delete"},
		{Key: "Esc", Description: "Cancel"},
	})

	sl.app.JigApp().Pages().Push(modal)
	sl.app.JigApp().SetFocus(form)
}

func (sl *ScheduleList) executeDeleteSchedule(scheduleID string) {
	provider := sl.app.Provider()
	if provider == nil {
		return
	}

	namespace := sl.namespace
	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			sl.loadData()
		}).
		OnError(func(err error) {
			sl.showError(err)
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.DeleteSchedule(ctx, namespace, scheduleID)
		})
}

func (sl *ScheduleList) closeModal() {
	sl.app.JigApp().Pages().DismissModal()
}

// Name returns the view name.
func (sl *ScheduleList) Name() string {
	return "schedules"
}

// Start is called when the view becomes active.
func (sl *ScheduleList) Start() {
	bindings := input.NewKeyBindings().
		OnRune('r', func(e *tcell.EventKey) bool {
			sl.loadData()
			return true
		}).
		OnRune('/', func(e *tcell.EventKey) bool {
			sl.MasterDetailView.ShowSearch()
			return true
		}).
		OnRune('p', func(e *tcell.EventKey) bool {
			sl.togglePreview()
			return true
		}).
		OnRune('P', func(e *tcell.EventKey) bool {
			sl.showPauseConfirm()
			return true
		}).
		OnRune('t', func(e *tcell.EventKey) bool {
			sl.showTriggerConfirm()
			return true
		}).
		OnRune('D', func(e *tcell.EventKey) bool {
			sl.showDeleteConfirm()
			return true
		})

	sl.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if bindings.Handle(event) {
			return nil
		}
		return event
	})
	sl.loadData()
}

// Stop is called when the view is deactivated.
func (sl *ScheduleList) Stop() {
	sl.table.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (sl *ScheduleList) Hints() []KeyHint {
	hints := []KeyHint{
		{Key: "/", Description: "Search"},
		{Key: "r", Description: "Refresh"},
		{Key: "j/k", Description: "Navigate"},
		{Key: "p", Description: "Preview"},
		{Key: "P", Description: "Pause/Unpause"},
		{Key: "t", Description: "Trigger"},
		{Key: "D", Description: "Delete"},
		{Key: "T", Description: "Theme"},
		{Key: "esc", Description: "Back"},
	}
	return hints
}

// Focus sets focus to the table.
func (sl *ScheduleList) Focus(delegate func(p tview.Primitive)) {
	delegate(sl.table)
}

// Draw applies theme colors dynamically and draws the view.
func (sl *ScheduleList) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	sl.preview.SetBackgroundColor(bg)
	sl.preview.SetTextColor(theme.Fg())
	sl.MasterDetailView.Draw(screen)
}
