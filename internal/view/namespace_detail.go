package view

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/input"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NamespaceDetail displays detailed information about a namespace.
type NamespaceDetail struct {
	*tview.Flex
	app       *App
	namespace string
	detail    *temporal.NamespaceDetail
	loading   bool

	// UI components
	infoPanel     *components.Panel
	archivalPanel *components.Panel
	clusterPanel  *components.Panel
	infoView      *tview.TextView
	archivalView  *tview.TextView
	clusterView   *tview.TextView
}

// NewNamespaceDetail creates a new namespace detail view.
func NewNamespaceDetail(app *App, namespace string) *NamespaceDetail {
	nd := &NamespaceDetail{
		Flex:      tview.NewFlex().SetDirection(tview.FlexColumn),
		app:       app,
		namespace: namespace,
	}
	nd.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(nd)

	return nd
}

func (nd *NamespaceDetail) setup() {
	nd.SetBackgroundColor(theme.Bg())

	// Info view
	nd.infoView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	nd.infoView.SetBackgroundColor(theme.Bg())

	// Archival view
	nd.archivalView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	nd.archivalView.SetBackgroundColor(theme.Bg())

	// Cluster view
	nd.clusterView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	nd.clusterView.SetBackgroundColor(theme.Bg())

	// Create panels with icons (blubber pattern)
	nd.infoPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Namespace Info", theme.IconNamespace))
	nd.infoPanel.SetContent(nd.infoView)

	nd.archivalPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Archival Configuration", theme.IconDatabase))
	nd.archivalPanel.SetContent(nd.archivalView)

	nd.clusterPanel = components.NewPanel().SetTitle(fmt.Sprintf("%s Cluster & Replication", theme.IconServer))
	nd.clusterPanel.SetContent(nd.clusterView)

	// Left side: Info panel
	leftFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	leftFlex.SetBackgroundColor(theme.Bg())
	leftFlex.AddItem(nd.infoPanel, 0, 2, false)

	// Right side: Archival + Cluster stacked
	rightFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	rightFlex.SetBackgroundColor(theme.Bg())
	rightFlex.AddItem(nd.archivalPanel, 0, 1, false)
	rightFlex.AddItem(nd.clusterPanel, 0, 1, false)

	// Main layout
	nd.AddItem(leftFlex, 0, 1, true)
	nd.AddItem(rightFlex, 0, 1, false)

	// Show loading state initially
	nd.infoView.SetText(fmt.Sprintf("\n [%s]Loading...[-]", theme.TagFgDim()))
}

func (nd *NamespaceDetail) loadData() {
	provider := nd.app.Provider()
	if provider == nil {
		nd.loadMockData()
		return
	}

	nd.loading = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		detail, err := provider.DescribeNamespace(ctx, nd.namespace)

		nd.app.JigApp().QueueUpdateDraw(func() {
			nd.loading = false
			if err != nil {
				nd.showError(err)
				return
			}
			nd.detail = detail
			nd.render()
		})
	}()
}

func (nd *NamespaceDetail) loadMockData() {
	nd.detail = &temporal.NamespaceDetail{
		Namespace: temporal.Namespace{
			Name:            nd.namespace,
			State:           "Active",
			RetentionPeriod: "30 days",
			Description:     "Mock namespace for development",
			OwnerEmail:      "dev@example.com",
		},
		ID:                 "mock-namespace-id-12345",
		IsGlobalNamespace:  false,
		FailoverVersion:    0,
		HistoryArchival:    "Disabled",
		VisibilityArchival: "Disabled",
		Clusters:           []string{"active"},
	}
	nd.render()
}

func (nd *NamespaceDetail) showError(err error) {
	nd.infoView.SetText(fmt.Sprintf("\n [%s]Error: %s[-]", theme.TagError(), err.Error()))
	nd.archivalView.SetText("")
	nd.clusterView.SetText("")
}

// RefreshTheme updates all component colors after a theme change.
func (nd *NamespaceDetail) RefreshTheme() {
	bg := theme.Bg()

	// Update main container
	nd.SetBackgroundColor(bg)

	// Update text views
	nd.infoView.SetBackgroundColor(bg)
	nd.archivalView.SetBackgroundColor(bg)
	nd.clusterView.SetBackgroundColor(bg)

	// Re-render content with new theme colors
	nd.render()
}

func (nd *NamespaceDetail) render() {
	if nd.detail == nil {
		nd.infoView.SetText(fmt.Sprintf(" [%s]Namespace not found[-]", theme.TagError()))
		return
	}

	d := nd.detail
	stateColor := nd.stateColorTag(d.State)
	stateIcon := nd.stateIcon(d.State)

	// Main namespace info
	infoText := fmt.Sprintf(`
[%s::b]Name[-:-:-]           [%s]%s[-]
[%s::b]State[-:-:-]          [%s]%s %s[-]
[%s::b]Retention[-:-:-]      [%s]%s[-]
[%s::b]Description[-:-:-]    [%s]%s[-]
[%s::b]Owner Email[-:-:-]    [%s]%s[-]
[%s::b]Namespace ID[-:-:-]   [%s]%s[-]`,
		theme.TagFgDim(), theme.TagFg(), d.Name,
		theme.TagFgDim(), stateColor, stateIcon, d.State,
		theme.TagFgDim(), theme.TagFg(), d.RetentionPeriod,
		theme.TagFgDim(), theme.TagFg(), nd.valueOrNA(d.Description),
		theme.TagFgDim(), theme.TagFg(), nd.valueOrNA(d.OwnerEmail),
		theme.TagFgDim(), theme.TagFgDim(), nd.valueOrNA(d.ID),
	)
	nd.infoView.SetText(infoText)

	// Archival configuration
	archivalText := fmt.Sprintf(`
[%s::b]History Archival[-:-:-]
  [%s]%s[-]

[%s::b]Visibility Archival[-:-:-]
  [%s]%s[-]`,
		theme.TagFgDim(), theme.TagFg(), nd.valueOrNA(d.HistoryArchival),
		theme.TagFgDim(), theme.TagFg(), nd.valueOrNA(d.VisibilityArchival),
	)
	nd.archivalView.SetText(archivalText)

	// Cluster info
	globalStr := "No"
	if d.IsGlobalNamespace {
		globalStr = "Yes"
	}

	clustersStr := "None"
	if len(d.Clusters) > 0 {
		clustersStr = strings.Join(d.Clusters, ", ")
	}

	clusterText := fmt.Sprintf(`
[%s::b]Global Namespace[-:-:-]  [%s]%s[-]
[%s::b]Failover Version[-:-:-]  [%s]%d[-]
[%s::b]Clusters[-:-:-]          [%s]%s[-]`,
		theme.TagFgDim(), theme.TagFg(), globalStr,
		theme.TagFgDim(), theme.TagFg(), d.FailoverVersion,
		theme.TagFgDim(), theme.TagFg(), clustersStr,
	)
	nd.clusterView.SetText(clusterText)
}

func (nd *NamespaceDetail) valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func (nd *NamespaceDetail) stateColorTag(state string) string {
	switch state {
	case "Active":
		return temporal.NamespaceStateActive.ColorTag()
	case "Deprecated":
		return temporal.NamespaceStateDeprecated.ColorTag()
	case "Deleted":
		return temporal.NamespaceStateDeleted.ColorTag()
	default:
		return theme.TagFg()
	}
}

func (nd *NamespaceDetail) stateIcon(state string) string {
	switch state {
	case "Active":
		return "●"
	case "Deprecated":
		return "○"
	case "Deleted":
		return "×"
	default:
		return "?"
	}
}

// Name returns the view name.
func (nd *NamespaceDetail) Name() string {
	return "namespace-detail"
}

// Start is called when the view becomes active.
func (nd *NamespaceDetail) Start() {
	bindings := input.NewKeyBindings().
		OnRune('r', func(e *tcell.EventKey) bool {
			nd.loadData()
			return true
		}).
		OnRune('e', func(e *tcell.EventKey) bool {
			nd.showEditForm()
			return true
		}).
		OnRune('D', func(e *tcell.EventKey) bool {
			nd.showDeprecateConfirm()
			return true
		})

	nd.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if bindings.Handle(event) {
			return nil
		}
		return event
	})
	nd.loadData()
}

// Stop is called when the view is deactivated.
func (nd *NamespaceDetail) Stop() {
	nd.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (nd *NamespaceDetail) Hints() []KeyHint {
	hints := []KeyHint{
		{Key: "r", Description: "Refresh"},
		{Key: "e", Description: "Edit"},
	}

	// Only show deprecate for active namespaces
	if nd.detail != nil && nd.detail.State == "Active" {
		hints = append(hints, KeyHint{Key: "D", Description: "Deprecate"})
	}

	hints = append(hints,
		KeyHint{Key: "T", Description: "Theme"},
		KeyHint{Key: "esc", Description: "Back"},
	)

	return hints
}

// Focus sets focus to this view.
func (nd *NamespaceDetail) Focus(delegate func(p tview.Primitive)) {
	delegate(nd.Flex)
}

// Draw applies theme colors dynamically and draws the view.
func (nd *NamespaceDetail) Draw(screen tcell.Screen) {
	bg := theme.Bg()
	nd.SetBackgroundColor(bg)
	nd.infoView.SetBackgroundColor(bg)
	nd.archivalView.SetBackgroundColor(bg)
	nd.clusterView.SetBackgroundColor(bg)
	nd.Flex.Draw(screen)
}

// Edit functionality - implemented using jig components

func (nd *NamespaceDetail) showEditForm() {
	if nd.detail == nil {
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Edit Namespace", theme.IconNamespace),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	// Parse current retention period (e.g., "72h0m0s" -> 3)
	currentRetention := 3
	if nd.detail.RetentionPeriod != "" {
		if dur, err := time.ParseDuration(nd.detail.RetentionPeriod); err == nil {
			currentRetention = int(dur.Hours() / 24)
		}
	}

	form := components.NewFormBuilder().
		Text("description", "Description").
			Value(nd.detail.Description).
			Placeholder("Enter description").
			Done().
		Text("ownerEmail", "Owner Email").
			Value(nd.detail.OwnerEmail).
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
				Name:          nd.namespace,
				Description:   values["description"].(string),
				OwnerEmail:    values["ownerEmail"].(string),
				RetentionDays: retentionDays,
			}
			nd.closeModal()
			nd.showUpdateConfirm(updateReq)
		}).
		OnCancel(func() {
			nd.closeModal()
		}).
		Build()

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})

	nd.app.JigApp().Pages().Push(modal)
	nd.app.JigApp().SetFocus(form)
}

func (nd *NamespaceDetail) showUpdateConfirm(req temporal.NamespaceUpdateRequest) {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Confirm Update", theme.IconWarning),
		Width:    65,
		Height:   14,
		Backdrop: true,
	})

	// Show what will be updated
	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(theme.Bg())

	changesText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	changesText.SetBackgroundColor(theme.Bg())
	changesText.SetText(fmt.Sprintf(`[%s]Update namespace:[-] [%s]%s[-]

[%s]Description:[-]   [%s]%s[-]
[%s]Owner Email:[-]   [%s]%s[-]
[%s]Retention:[-]     [%s]%d days[-]`,
		theme.TagAccent(), theme.TagFg(), req.Name,
		theme.TagFgDim(), theme.TagFg(), req.Description,
		theme.TagFgDim(), theme.TagFg(), req.OwnerEmail,
		theme.TagFgDim(), theme.TagFg(), req.RetentionDays))

	contentFlex.AddItem(changesText, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Update"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		nd.closeModal()
		nd.executeUpdate(req)
	})
	modal.SetOnCancel(func() {
		nd.closeModal()
	})

	nd.app.JigApp().Pages().Push(modal)
}

func (nd *NamespaceDetail) executeUpdate(req temporal.NamespaceUpdateRequest) {
	provider := nd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.UpdateNamespace(ctx, req)

		nd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				nd.showError(err)
				return
			}
			nd.loadData() // Refresh to show updated values
		})
	}()
}

func (nd *NamespaceDetail) showDeprecateConfirm() {
	if nd.detail == nil {
		return
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
		theme.TagFgDim(), theme.TagFg(), nd.namespace))

	form := components.NewFormBuilder().
		Text("confirm", "Type namespace name to confirm").
			Placeholder(nd.namespace).
			Validate(validators.Required()).
			Done().
		OnSubmit(func(values map[string]any) {
			confirm := values["confirm"].(string)
			if confirm != nd.namespace {
				return // Must match namespace name
			}
			nd.closeModal()
			nd.executeDeprecate()
		}).
		OnCancel(func() {
			nd.closeModal()
		}).
		Build()

	contentFlex.AddItem(warningText, 8, 0, false)
	contentFlex.AddItem(form, 0, 1, true)

	modal.SetContent(contentFlex)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Deprecate"},
		{Key: "Esc", Description: "Cancel"},
	})

	nd.app.JigApp().Pages().Push(modal)
	nd.app.JigApp().SetFocus(form)
}

func (nd *NamespaceDetail) executeDeprecate() {
	provider := nd.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := provider.DeprecateNamespace(ctx, nd.namespace)

		nd.app.JigApp().QueueUpdateDraw(func() {
			if err != nil {
				nd.showError(err)
				return
			}
			nd.loadData() // Refresh to show updated state
		})
	}()
}

func (nd *NamespaceDetail) closeModal() {
	nd.app.JigApp().Pages().DismissModal()
}
