package view

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NamespaceList displays a list of Temporal namespaces with a preview panel.
type NamespaceList struct {
	*tview.Flex
	table         *components.Table
	leftPanel     *components.Panel
	rightPanel    *components.Panel
	preview       *tview.TextView
	emptyState    *components.EmptyState
	app           *App
	namespaces    []temporal.Namespace
	loading       bool
	autoRefresh   bool
	showPreview   bool
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}
}

// NewNamespaceList creates a new namespace list view.
func NewNamespaceList(app *App) *NamespaceList {
	nl := &NamespaceList{
		Flex:        tview.NewFlex().SetDirection(tview.FlexColumn),
		table:       components.NewTable(),
		preview:     tview.NewTextView(),
		app:         app,
		namespaces:  []temporal.Namespace{},
		showPreview: true,
		autoRefresh: true,
		stopRefresh: make(chan struct{}),
	}
	nl.setup()
	return nl
}

func (nl *NamespaceList) setup() {
	nl.table.SetHeaders("NAME", "STATE", "RETENTION")
	nl.table.SetBorder(false)
	nl.table.SetBackgroundColor(theme.Bg())
	nl.SetBackgroundColor(theme.Bg())

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

	// Create panels with icons (blubber pattern)
	nl.leftPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Namespaces", theme.IconNamespace))
	nl.leftPanel.SetContent(nl.table)

	nl.rightPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Details", theme.IconInfo))
	nl.rightPanel.SetContent(nl.preview)

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

	nl.buildLayout()
}

func (nl *NamespaceList) buildLayout() {
	nl.Clear()
	if nl.showPreview {
		nl.AddItem(nl.leftPanel, 0, 3, true)
		nl.AddItem(nl.rightPanel, 0, 2, false)
	} else {
		nl.AddItem(nl.leftPanel, 0, 1, true)
	}
}

func (nl *NamespaceList) togglePreview() {
	nl.showPreview = !nl.showPreview
	nl.buildLayout()
}

// RefreshTheme updates all component colors after a theme change.
func (nl *NamespaceList) RefreshTheme() {
	bg := theme.Bg()

	// Update main container
	nl.SetBackgroundColor(bg)

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
	stateColor := theme.StatusColorTag("Running")
	if ns.State == "Deprecated" {
		stateIcon = theme.IconDisconnected
		stateColor = theme.StatusColorTag("Failed")
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		namespaces, err := provider.ListNamespaces(ctx)

		nl.app.JigApp().QueueUpdateDraw(func() {
			nl.setLoading(false)
			if err != nil {
				nl.showError(err)
				return
			}
			nl.namespaces = namespaces
			nl.populateTable()
		})
	}()
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
		nl.leftPanel.SetContent(nl.emptyState)
		nl.preview.SetText("")
		return
	}

	nl.leftPanel.SetContent(nl.table)

	for _, ns := range nl.namespaces {
		nl.table.AddStyledRowSimple(ns.State,
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
	nl.refreshTicker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-nl.refreshTicker.C:
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
			// TODO: Deprecate confirm
			return nil
		case 'X':
			// TODO: Delete confirm
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

	hints = append(hints,
		KeyHint{Key: "S", Description: "Signal+Start"},
		KeyHint{Key: "p", Description: "Preview"},
		KeyHint{Key: "r", Description: "Refresh"},
		KeyHint{Key: "a", Description: "Auto-refresh"},
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "?", Description: "Help"},
		KeyHint{Key: "q", Description: "Quit"},
	)
	return hints
}

// Focus sets focus to the table.
func (nl *NamespaceList) Focus(delegate func(p tview.Primitive)) {
	if len(nl.namespaces) == 0 {
		delegate(nl.Flex)
		return
	}
	delegate(nl.table)
}

// Draw applies theme colors dynamically and draws the view.
func (nl *NamespaceList) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	nl.SetBackgroundColor(bg)
	nl.preview.SetBackgroundColor(bg)
	nl.preview.SetTextColor(theme.Fg())
	nl.Flex.Draw(screen)
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

		nl.closeModal("signal-with-start")
		nl.executeSignalWithStart(namespace, workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	form.SetOnCancel(func() {
		nl.closeModal("signal-with-start")
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

		nl.closeModal("signal-with-start")
		nl.executeSignalWithStart(namespace, workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput)
	})
	modal.SetOnCancel(func() {
		nl.closeModal("signal-with-start")
	})

	nl.app.JigApp().Pages().AddPage("signal-with-start", modal, true, true)
	nl.app.JigApp().SetFocus(form)
}

// executeSignalWithStart performs the SignalWithStart operation asynchronously.
func (nl *NamespaceList) executeSignalWithStart(namespace, workflowID, workflowType, taskQueue, signalName, signalInput, workflowInput string) {
	provider := nl.app.Provider()
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

		runID, err := provider.SignalWithStartWorkflow(ctx, namespace, req)

		nl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(nl.app.JigApp(), "SignalWithStart Failed", err.Error())
				return
			}

			ShowInfoModal(nl.app.JigApp(), "SignalWithStart Successful",
				fmt.Sprintf("Workflow: %s\nRun ID: %s", workflowID, runID))
		})
	}()
}

// closeModal removes a modal page and restores focus to the current view.
func (nl *NamespaceList) closeModal(name string) {
	nl.app.JigApp().Pages().RemovePage(name)
	if current := nl.app.JigApp().Pages().Current(); current != nil {
		nl.app.JigApp().SetFocus(current)
	}
}

// showCreateNamespaceForm displays a modal for creating a new namespace.
func (nl *NamespaceList) showCreateNamespaceForm() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Create Namespace", theme.IconNamespace),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("name", "Namespace Name", "")
	form.AddTextField("description", "Description", "")
	form.AddTextField("ownerEmail", "Owner Email", "")
	form.AddTextField("retention", "Retention (days)", "3")

	form.SetOnSubmit(func(values map[string]any) {
		name := values["name"].(string)
		if name == "" {
			return // Name is required
		}

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
		nl.closeModal("create-namespace")
		nl.executeCreateNamespace(createReq)
	})
	form.SetOnCancel(func() {
		nl.closeModal("create-namespace")
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Create"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		name := values["name"].(string)
		if name == "" {
			return
		}

		retentionStr := values["retention"].(string)
		retentionDays, err := strconv.Atoi(retentionStr)
		if err != nil || retentionDays < 1 {
			retentionDays = 3
		}

		createReq := temporal.NamespaceCreateRequest{
			Name:          name,
			Description:   values["description"].(string),
			OwnerEmail:    values["ownerEmail"].(string),
			RetentionDays: retentionDays,
		}
		nl.closeModal("create-namespace")
		nl.executeCreateNamespace(createReq)
	})
	modal.SetOnCancel(func() {
		nl.closeModal("create-namespace")
	})

	nl.app.JigApp().Pages().AddPage("create-namespace", modal, true, true)
	nl.app.JigApp().SetFocus(form)
}

// executeCreateNamespace performs the namespace creation asynchronously.
func (nl *NamespaceList) executeCreateNamespace(req temporal.NamespaceCreateRequest) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Create Failed", "No provider connected")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.CreateNamespace(ctx, req)

		nl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(nl.app.JigApp(), "Create Namespace Failed", err.Error())
				return
			}

			ShowInfoModal(nl.app.JigApp(), "Namespace Created",
				fmt.Sprintf("Namespace '%s' created successfully", req.Name))
			nl.loadData() // Refresh the list
		})
	}()
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

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		detail, err := provider.DescribeNamespace(ctx, ns.Name)

		nl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				// Fall back to list data
				nl.showEditFormWithData(ns.Name, ns.Description, ns.OwnerEmail, ns.RetentionPeriod)
				return
			}
			nl.showEditFormWithData(detail.Name, detail.Description, detail.OwnerEmail, detail.RetentionPeriod)
		})
	}()
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

	form := components.NewForm()
	form.AddTextField("description", "Description", description)
	form.AddTextField("ownerEmail", "Owner Email", ownerEmail)
	form.AddTextField("retention", "Retention (days)", strconv.Itoa(currentRetention))

	form.SetOnSubmit(func(values map[string]any) {
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
		nl.closeModal("edit-namespace")
		nl.executeUpdateNamespace(updateReq)
	})
	form.SetOnCancel(func() {
		nl.closeModal("edit-namespace")
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		retentionStr := values["retention"].(string)
		retentionDays, err := strconv.Atoi(retentionStr)
		if err != nil || retentionDays < 1 {
			return
		}

		updateReq := temporal.NamespaceUpdateRequest{
			Name:          name,
			Description:   values["description"].(string),
			OwnerEmail:    values["ownerEmail"].(string),
			RetentionDays: retentionDays,
		}
		nl.closeModal("edit-namespace")
		nl.executeUpdateNamespace(updateReq)
	})
	modal.SetOnCancel(func() {
		nl.closeModal("edit-namespace")
	})

	nl.app.JigApp().Pages().AddPage("edit-namespace", modal, true, true)
	nl.app.JigApp().SetFocus(form)
}

// executeUpdateNamespace performs the namespace update asynchronously.
func (nl *NamespaceList) executeUpdateNamespace(req temporal.NamespaceUpdateRequest) {
	provider := nl.app.Provider()
	if provider == nil {
		ShowErrorModal(nl.app.JigApp(), "Update Failed", "No provider connected")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.UpdateNamespace(ctx, req)

		nl.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				ShowErrorModal(nl.app.JigApp(), "Update Namespace Failed", err.Error())
				return
			}

			ShowInfoModal(nl.app.JigApp(), "Namespace Updated",
				fmt.Sprintf("Namespace '%s' updated successfully", req.Name))
			nl.loadData() // Refresh the list
		})
	}()
}
