package view

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/atterpac/jig/async"
	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NamespaceList displays a list of Temporal namespaces with a preview panel.
type NamespaceList struct {
	*components.MasterDetailView
	table         *components.Table
	preview       *tview.TextView
	emptyState    *components.EmptyState
	app           *App
	namespaces    []temporal.Namespace
	loading       bool
	autoRefresh   bool
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}
}

// NewNamespaceList creates a new namespace list view.
func NewNamespaceList(app *App) *NamespaceList {
	nl := &NamespaceList{
		table:       components.NewTable(),
		preview:     tview.NewTextView(),
		app:         app,
		namespaces:  []temporal.Namespace{},
		autoRefresh: true,
		stopRefresh: make(chan struct{}, 1), // Buffered to ensure stop signal isn't lost
	}
	nl.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(nl)

	return nl
}

func (nl *NamespaceList) setup() {
	nl.table.SetHeaders("NAME", "STATE", "RETENTION")
	nl.table.SetBorder(false)
	nl.table.SetBackgroundColor(theme.Bg())

	// Configure preview
	nl.preview.SetDynamicColors(true)
	nl.preview.SetBackgroundColor(theme.Bg())
	nl.preview.SetTextColor(theme.Fg())
	nl.preview.SetWordWrap(true)

	// Create empty state
	nl.emptyState = components.NewEmptyState().
		SetIcon(theme.IconDatabase).
		SetTitle("No Namespaces").
		SetMessage("No namespaces found")

	// Create MasterDetailView
	nl.MasterDetailView = components.NewMasterDetailView().
		SetMasterTitle(fmt.Sprintf("%s Namespaces", theme.IconNamespace)).
		SetDetailTitle(fmt.Sprintf("%s Details", theme.IconInfo)).
		SetMasterContent(nl.table).
		SetDetailContent(nl.preview).
		SetRatio(0.6).
		ConfigureEmpty(theme.IconInfo, "No Selection", "Select a namespace to view details")

	// Selection change handler to update preview and hints
	nl.table.SetSelectionChangedFunc(func(row, col int) {
		dataRow := row - 1
		if dataRow >= 0 && dataRow < len(nl.namespaces) {
			nl.updatePreview(nl.namespaces[dataRow])
			nl.app.JigApp().Menu().SetHints(nl.Hints())
		}
	})

	// Selection handler - Enter navigates to workflows
	nl.table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(nl.namespaces) {
			nl.app.NavigateToWorkflows(nl.namespaces[row].Name)
		}
	})
}

func (nl *NamespaceList) togglePreview() {
	nl.ToggleDetail()
}

// RefreshTheme updates all component colors after a theme change.
func (nl *NamespaceList) RefreshTheme() {
	bg := theme.Bg()

	// Update table
	nl.table.SetBackgroundColor(bg)

	// Update preview
	nl.preview.SetBackgroundColor(bg)
	nl.preview.SetTextColor(theme.Fg())

	// Re-render table with new theme colors
	nl.populateTable()
}

func (nl *NamespaceList) updatePreview(ns temporal.Namespace) {
	stateIcon := theme.IconConnected
	stateStatus := temporal.GetNamespaceState(ns.State)
	stateColor := stateStatus.ColorTag()
	if ns.State == "Deprecated" {
		stateIcon = theme.IconDisconnected
	}

	text := fmt.Sprintf(`[%s::b]Name[-:-:-]
  [%s]%s[-]

[%s::b]State[-:-:-]
  [%s]%s %s[-]

[%s::b]Retention[-:-:-]
  [%s]%s[-]

[%s::b]Description[-:-:-]
  [%s]%s[-]

[%s::b]Owner[-:-:-]
  [%s]%s[-]`,
		theme.TagFgDim(),
		theme.TagFg(), ns.Name,
		theme.TagFgDim(),
		stateColor, stateIcon, ns.State,
		theme.TagFgDim(),
		theme.TagFg(), ns.RetentionPeriod,
		theme.TagFgDim(),
		theme.TagFg(), valueOrEmpty(ns.Description, "No description"),
		theme.TagFgDim(),
		theme.TagFg(), valueOrEmpty(ns.OwnerEmail, "No owner"),
	)
	nl.preview.SetText(text)
}

func valueOrEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func (nl *NamespaceList) setLoading(loading bool) {
	nl.loading = loading
}

func (nl *NamespaceList) loadData() {
	provider := nl.app.Provider()
	if provider == nil {
		nl.loadMockData()
		return
	}

	nl.setLoading(true)
	async.NewLoader[[]temporal.Namespace]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(namespaces []temporal.Namespace) {
			nl.namespaces = namespaces
			nl.populateTable()
		}).
		OnError(func(err error) {
			nl.showError(err)
		}).
		OnFinally(func() {
			nl.setLoading(false)
		}).
		Run(func(ctx context.Context) ([]temporal.Namespace, error) {
			return provider.ListNamespaces(ctx)
		})
}

func (nl *NamespaceList) loadMockData() {
	nl.namespaces = []temporal.Namespace{
		{Name: "default", State: "Active", RetentionPeriod: "7 days"},
		{Name: "production", State: "Active", RetentionPeriod: "30 days"},
		{Name: "staging", State: "Active", RetentionPeriod: "3 days"},
		{Name: "development", State: "Active", RetentionPeriod: "1 day"},
		{Name: "archived", State: "Deprecated", RetentionPeriod: "90 days"},
	}
	nl.populateTable()
}

func (nl *NamespaceList) populateTable() {
	currentRow := nl.table.SelectedRow()

	nl.table.ClearRows()
	nl.table.SetHeaders("NAME", "STATE", "RETENTION")

	if len(nl.namespaces) == 0 {
		nl.SetMasterContent(nl.emptyState)
		nl.preview.SetText("")
		return
	}

	nl.SetMasterContent(nl.table)

	for _, ns := range nl.namespaces {
		stateStatus := temporal.GetNamespaceState(ns.State)
		nl.table.AddRowWithStatus(stateStatus, 1, // status column is index 1
			theme.IconDatabase+" "+ns.Name,
			ns.State,
			ns.RetentionPeriod,
		)
	}

	if nl.table.RowCount() > 0 {
		if currentRow >= 0 && currentRow < len(nl.namespaces) {
			nl.table.SelectRow(currentRow)
			nl.updatePreview(nl.namespaces[currentRow])
		} else {
			nl.table.SelectRow(0)
			if len(nl.namespaces) > 0 {
				nl.updatePreview(nl.namespaces[0])
			}
		}
	}
}

func (nl *NamespaceList) showError(err error) {
	nl.table.ClearRows()
	nl.table.SetHeaders("NAME", "STATE", "RETENTION")
	nl.table.AddRowWithColor(theme.Error(),
		theme.IconError+" Error loading namespaces",
		err.Error(),
		"",
	)
}

func (nl *NamespaceList) toggleAutoRefresh() {
	nl.autoRefresh = !nl.autoRefresh
	if nl.autoRefresh {
		nl.startAutoRefresh()
	} else {
		nl.stopAutoRefresh()
	}
}

func (nl *NamespaceList) startAutoRefresh() {
	// Drain any stale stop signal from previous stop
	select {
	case <-nl.stopRefresh:
	default:
	}

	ticker := time.NewTicker(5 * time.Second)
	nl.refreshTicker = ticker
	go func() {
		for {
			select {
			case <-ticker.C:
				nl.app.JigApp().QueueUpdateDraw(func() {
					nl.loadData()
				})
			case <-nl.stopRefresh:
				return
			}
		}
	}()
}

func (nl *NamespaceList) stopAutoRefresh() {
	if nl.refreshTicker != nil {
		nl.refreshTicker.Stop()
		nl.refreshTicker = nil
	}
	select {
	case nl.stopRefresh <- struct{}{}:
	default:
	}
}

// Name returns the view name.
func (nl *NamespaceList) Name() string {
	return "namespaces"
}

// Start is called when the view becomes active.
func (nl *NamespaceList) Start() {
	nl.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			nl.app.Stop()
			return nil
		case 'a':
			nl.toggleAutoRefresh()
			return nil
		case 'r':
			nl.loadData()
			return nil
		case 'p':
			nl.togglePreview()
			return nil
		case 'i':
			ns := nl.getSelectedNamespace()
			if ns != nil {
				nl.app.NavigateToNamespaceDetail(ns.Name)
			}
			return nil
		case 'n':
			nl.showCreateNamespaceForm()
			return nil
		case 'e':
			nl.showEditNamespaceForm()
			return nil
		case 'D':
			ns := nl.getSelectedNamespace()
			if ns != nil && ns.State != "Deprecated" {
				nl.showDeprecateConfirm()
			}
			return nil
		case 'X':
			ns := nl.getSelectedNamespace()
			if ns != nil && ns.State == "Deprecated" {
				nl.showDeleteConfirm()
			}
			return nil
		case 'S':
			ns := nl.getSelectedNamespace()
			if ns != nil {
				nl.showSignalWithStart(ns.Name)
			}
			return nil
		}
		return event
	})
	nl.loadData()
	if nl.autoRefresh {
		nl.startAutoRefresh()
	}
}

// Stop is called when the view is deactivated.
func (nl *NamespaceList) Stop() {
	nl.table.SetInputCapture(nil)
	nl.stopAutoRefresh()
}

// Hints returns keybinding hints for this view.
func (nl *NamespaceList) Hints() []KeyHint {
	hints := []KeyHint{
		{Key: "enter", Description: "Workflows"},
		{Key: "i", Description: "Info"},
		{Key: "n", Description: "Create"},
		{Key: "e", Description: "Edit"},
	}

	ns := nl.getSelectedNamespace()
	if ns != nil && ns.State == "Deprecated" {
		hints = append(hints, KeyHint{Key: "X", Description: "Delete"})
	} else {
		hints = append(hints, KeyHint{Key: "D", Description: "Deprecate"})
	}

	autoHint := "Auto: Off"
	if nl.autoRefresh {
		autoHint = "Auto: On"
	}

	hints = append(hints,
		KeyHint{Key: "S", Description: "Signal+Start"},
		KeyHint{Key: "p", Description: "Preview"},
		KeyHint{Key: "r", Description: "Refresh"},
		KeyHint{Key: "a", Description: autoHint},
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "?", Description: "Help"},
		KeyHint{Key: "q", Description: "Quit"},
	)
	return hints
}

// Focus sets focus to the table.
func (nl *NamespaceList) Focus(delegate func(p tview.Primitive)) {
	if len(nl.namespaces) == 0 {
		delegate(nl.MasterDetailView)
		return
	}
	delegate(nl.table)
}

// Draw applies theme colors dynamically and draws the view.
func (nl *NamespaceList) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	nl.preview.SetBackgroundColor(bg)
	nl.preview.SetTextColor(theme.Fg())
	nl.MasterDetailView.Draw(screen)
}

// getSelectedNamespace returns the currently selected namespace.
func (nl *NamespaceList) getSelectedNamespace() *temporal.Namespace {
	row := nl.table.SelectedRow()
	if row >= 0 && row < len(nl.namespaces) {
		return &nl.namespaces[row]
	}
	return nil
}

// showSignalWithStart displays a modal for SignalWithStart operation.
func (nl *NamespaceList) showSignalWithStart(namespace string) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Signal With Start (%s)", theme.IconInfo, namespace),
		Width:    70,
		Height:   20,
		Backdrop: true,
	})

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

			nl.closeModal()
			nl.executeSignalWithStart(namespace, workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
		}).
		OnCancel(func() {
			nl.closeModal()
		}).
		Build()

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Execute"},
		{Key: "Esc", Description: "Cancel"},
	})

	nl.app.JigApp().Pages().Push(modal)
	nl.app.JigApp().SetFocus(form)
}

// executeSignalWithStart performs the SignalWithStart operation asynchronously.
func (nl *NamespaceList) executeSignalWithStart(namespace, workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput string) {
	provider := nl.app.Provider()
	if provider == nil {
		return
	}

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

	async.NewLoader[string]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ string) {
			nl.app.ToastSuccess(fmt.Sprintf("SignalWithStart: %s", workflowID))
		}).
		OnError(func(err error) {
			ShowErrorModal(nl.app.JigApp(), "SignalWithStart Failed", err.Error())
		}).
		Run(func(ctx context.Context) (string, error) {
			return provider.SignalWithStartWorkflow(ctx, namespace, req)
		})
}

// closeModal dismisses the current modal and restores focus.
func (nl *NamespaceList) closeModal() {
	nl.app.JigApp().Pages().DismissModal()
}

// showCreateNamespaceForm displays a modal for creating a new namespace.
func (nl *NamespaceList) showCreateNamespaceForm() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Create Namespace", theme.IconNamespace),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	form := components.NewFormBuilder().
		Text("name", "Namespace Name").
			Placeholder("Enter namespace name").
			Validate(validators.Required()).
			Done().
		Text("description", "Description").
			Placeholder("Enter description").
			Done().
		Text("ownerEmail", "Owner Email").
			Placeholder("owner@example.com").
			Done().
		Text("retention", "Retention (days)").
			Value("3").
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			name := values["name"].(string)

			retentionStr := values["retention"].(string)
			retentionDays, err := strconv.Atoi(retentionStr)
			if err != nil || retentionDays < 1 {
				retentionDays = 3 // Default to 3 days
			}

			createReq := temporal.NamespaceCreateRequest{
				Name:          name,
				Description:   values["description"].(string),
				OwnerEmail:    values["ownerEmail"].(string),
				RetentionDays: retentionDays,
			}
			nl.closeModal()
			nl.executeCreateNamespace(createReq)
		}).
		OnCancel(func() {
			nl.closeModal()
		}).
		Build()

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Create"},
		{Key: "Esc", Description: "Cancel"},
	})

	nl.app.JigApp().Pages().Push(modal)
	nl.app.JigApp().SetFocus(form)
}

// executeCreateNamespace performs the namespace creation asynchronously.
func (nl *NamespaceList) executeCreateNamespace(req temporal.NamespaceCreateRequest) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Create Failed", "No provider connected")
		return
	}

	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			nl.app.ToastSuccess(fmt.Sprintf("Namespace '%s' created", req.Name))
			nl.loadData()
		}).
		OnError(func(err error) {
			ShowErrorModal(nl.app.JigApp(), "Create Namespace Failed", err.Error())
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.CreateNamespace(ctx, req)
		})
}

// showEditNamespaceForm displays a modal for editing the selected namespace.
func (nl *NamespaceList) showEditNamespaceForm() {
	ns := nl.getSelectedNamespace()
	if ns == nil {
		return
	}

	// First, fetch the full details to get current values
	provider := nl.app.Provider()
	if provider == nil {
		// Use available data from list
		nl.showEditFormWithData(ns.Name, ns.Description, ns.OwnerEmail, ns.RetentionPeriod)
		return
	}

	// Capture values for closure
	name, desc, owner, retention := ns.Name, ns.Description, ns.OwnerEmail, ns.RetentionPeriod

	async.NewLoader[*temporal.NamespaceDetail]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(detail *temporal.NamespaceDetail) {
			nl.showEditFormWithData(detail.Name, detail.Description, detail.OwnerEmail, detail.RetentionPeriod)
		}).
		OnError(func(_ error) {
			// Fall back to list data
			nl.showEditFormWithData(name, desc, owner, retention)
		}).
		Run(func(ctx context.Context) (*temporal.NamespaceDetail, error) {
			return provider.DescribeNamespace(ctx, name)
		})
}

// showEditFormWithData displays the edit form with pre-populated values.
func (nl *NamespaceList) showEditFormWithData(name, description, ownerEmail, retentionPeriod string) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Edit Namespace: %s", theme.IconNamespace, name),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	// Parse retention period (e.g., "72h0m0s" or "3 days" -> 3)
	currentRetention := 3
	if retentionPeriod != "" {
		if dur, err := time.ParseDuration(retentionPeriod); err == nil {
			currentRetention = int(dur.Hours() / 24)
		}
	}

	form := components.NewFormBuilder().
		Text("description", "Description").
			Value(description).
			Placeholder("Enter description").
			Done().
		Text("ownerEmail", "Owner Email").
			Value(ownerEmail).
			Placeholder("owner@example.com").
			Done().
		Text("retention", "Retention (days)").
			Value(strconv.Itoa(currentRetention)).
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			retentionStr := values["retention"].(string)
			retentionDays, err := strconv.Atoi(retentionStr)
			if err != nil || retentionDays < 1 {
				return // Invalid retention
			}

			updateReq := temporal.NamespaceUpdateRequest{
				Name:          name,
				Description:   values["description"].(string),
				OwnerEmail:    values["ownerEmail"].(string),
				RetentionDays: retentionDays,
			}
			nl.closeModal()
			nl.executeUpdateNamespace(updateReq)
		}).
		OnCancel(func() {
			nl.closeModal()
		}).
		Build()

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})

	nl.app.JigApp().Pages().Push(modal)
	nl.app.JigApp().SetFocus(form)
}

// executeUpdateNamespace performs the namespace update asynchronously.
func (nl *NamespaceList) executeUpdateNamespace(req temporal.NamespaceUpdateRequest) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Update Failed", "No provider connected")
		return
	}

	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			nl.app.ToastSuccess(fmt.Sprintf("Namespace '%s' updated", req.Name))
			nl.loadData()
		}).
		OnError(func(err error) {
			ShowErrorModal(nl.app.JigApp(), "Update Namespace Failed", err.Error())
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.UpdateNamespace(ctx, req)
		})
}

// showDeprecateConfirm displays a confirmation modal for deprecating a namespace.
func (nl *NamespaceList) showDeprecateConfirm() {
	ns := nl.getSelectedNamespace()
	if ns == nil {
		return
	}

	// Capture name as value to avoid pointer issues with slice reallocation
	name := ns.Name

	// Pause auto-refresh while modal is open
	wasAutoRefresh := nl.autoRefresh
	if wasAutoRefresh {
		nl.stopAutoRefresh()
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Deprecate Namespace", theme.IconError),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	warningText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf(`[%s]Warning: Deprecating a namespace has the following effects:[-]

• New workflows cannot be started in this namespace
• Existing workflows will continue to run normally
• This action may be difficult to reverse

[%s]Namespace:[-] [%s]%s[-]`,
		theme.TagError(),
		theme.TagFgDim(), theme.TagFg(), name))

	form := components.NewFormBuilder().
		Text("confirm", "Type namespace name to confirm").
			Placeholder(name).
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			confirm := values["confirm"].(string)
			if confirm != name {
				return // Must match namespace name
			}
			nl.closeModal()
			if wasAutoRefresh {
				nl.autoRefresh = true
				nl.startAutoRefresh()
			}
			nl.executeDeprecateNamespace(name)
		}).
		OnCancel(func() {
			nl.closeModal()
			if wasAutoRefresh {
				nl.autoRefresh = true
				nl.startAutoRefresh()
			}
		}).
		Build()

	contentFlex.AddItem(warningText, 8, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Deprecate"},
		{Key: "Esc", Description: "Cancel"},
	})

	nl.app.JigApp().Pages().Push(modal)
	nl.app.JigApp().SetFocus(form)
}

// executeDeprecateNamespace performs the namespace deprecation asynchronously.
func (nl *NamespaceList) executeDeprecateNamespace(name string) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Deprecate Failed", "No provider connected")
		return
	}

	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			nl.app.ToastSuccess(fmt.Sprintf("Namespace '%s' deprecated", name))
			nl.loadData()
		}).
		OnError(func(err error) {
			ShowErrorModal(nl.app.JigApp(), "Deprecate Namespace Failed", err.Error())
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.DeprecateNamespace(ctx, name)
		})
}

// showDeleteConfirm displays a confirmation modal for deleting a deprecated namespace.
func (nl *NamespaceList) showDeleteConfirm() {
	ns := nl.getSelectedNamespace()
	if ns == nil || ns.State != "Deprecated" {
		return
	}

	// Capture values to avoid pointer issues with slice reallocation
	name := ns.Name
	state := ns.State

	// Pause auto-refresh while modal is open
	wasAutoRefresh := nl.autoRefresh
	if wasAutoRefresh {
		nl.stopAutoRefresh()
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Delete Namespace", theme.IconError),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	warningText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	warningText.SetBackgroundColor(theme.Bg())
	warningText.SetText(fmt.Sprintf(`[%s]DANGER: This action is irreversible![-]

Deleting a namespace will permanently remove:
• All workflow history
• All schedules
• All configuration

[%s]Namespace:[-] [%s]%s[-]
[%s]State:[-] [%s]%s[-]`,
		theme.TagError(),
		theme.TagFgDim(), theme.TagFg(), name,
		theme.TagFgDim(), theme.TagError(), state))

	form := components.NewFormBuilder().
		Text("confirm", "Type namespace name to confirm").
			Placeholder(name).
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			confirm := values["confirm"].(string)
			if confirm != name {
				return // Must match namespace name
			}
			nl.closeModal()
			if wasAutoRefresh {
				nl.autoRefresh = true
				nl.startAutoRefresh()
			}
			nl.executeDeleteNamespace(name)
		}).
		OnCancel(func() {
			nl.closeModal()
			if wasAutoRefresh {
				nl.autoRefresh = true
				nl.startAutoRefresh()
			}
		}).
		Build()

	contentFlex.AddItem(warningText, 10, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Delete"},
		{Key: "Esc", Description: "Cancel"},
	})

	nl.app.JigApp().Pages().Push(modal)
	nl.app.JigApp().SetFocus(form)
}

// executeDeleteNamespace performs the namespace deletion asynchronously.
func (nl *NamespaceList) executeDeleteNamespace(name string) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Delete Failed", "No provider connected")
		return
	}

	async.NewLoader[struct{}]().
		WithTimeout(10 * time.Second).
		OnSuccess(func(_ struct{}) {
			nl.app.ToastSuccess(fmt.Sprintf("Namespace '%s' deleted", name))
			nl.loadData()
		}).
		OnError(func(err error) {
			ShowErrorModal(nl.app.JigApp(), "Delete Namespace Failed", err.Error())
		}).
		Run(func(ctx context.Context) (struct{}, error) {
			return struct{}{}, provider.DeleteNamespace(ctx, name)
		})
}
