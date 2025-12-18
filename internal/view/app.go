package view

import (
	"context"
	"strings"
	"time"

	"github.com/atterpac/loom/internal/config"
	"github.com/atterpac/loom/internal/temporal"
	"github.com/atterpac/loom/internal/ui"
	"github.com/gdamore/tcell/v2"
)

const (
	connectionCheckInterval  = 10 * time.Second
	reconnectInitialBackoff  = 2 * time.Second
	reconnectMaxBackoff      = 30 * time.Second
	connectionCheckTimeout   = 5 * time.Second
)

// App is the main application controller.
type App struct {
	ui            *ui.App
	provider      temporal.Provider
	namespaceList *NamespaceList
	currentNS     string

	// Connection monitor
	stopMonitor  chan struct{}
	reconnecting bool

	// Profile management
	config        *config.Config
	activeProfile string

	// Dev mode
	devMode bool
}

// NewApp creates a new application controller with no provider (uses mock data).
func NewApp() *App {
	a := &App{
		ui:        ui.NewApp(),
		currentNS: "default",
	}
	a.setup()
	return a
}

// NewAppWithProvider creates a new application controller with a Temporal provider.
func NewAppWithProvider(provider temporal.Provider, defaultNamespace string, cfg *config.Config, activeProfile string) *App {
	a := &App{
		ui:            ui.NewApp(),
		provider:      provider,
		currentNS:     defaultNamespace,
		stopMonitor:   make(chan struct{}),
		config:        cfg,
		activeProfile: activeProfile,
	}
	a.setup()
	// Set initial connection status based on provider
	if provider != nil {
		a.ui.StatsBar().SetConnected(provider.IsConnected())
	}
	// Set initial profile name in stats bar
	a.ui.StatsBar().SetProfile(activeProfile)
	return a
}

func (a *App) setup() {
	// Set up page change handler
	a.ui.Pages().SetOnChange(func(c ui.Component) {
		a.ui.Menu().SetHints(c.Hints())
		a.updateCrumbs()
	})

	// Global key handler
	a.ui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Skip global handling when command bar or modal inputs are active
		if a.ui.CommandBar().IsActive() {
			return event // Let the command bar handle it
		}
		frontPage, _ := a.ui.Pages().GetFrontPage()
		// Skip global handling for modal pages (confirm-*, signal-*, reset-*, theme-selector, help-modal, profile-*, splash-test, etc.)
		if strings.HasPrefix(frontPage, "confirm-") ||
			strings.HasPrefix(frontPage, "signal-") ||
			strings.HasPrefix(frontPage, "reset-") ||
			frontPage == "theme-selector" ||
			frontPage == "help-modal" ||
			frontPage == "profile-selector" ||
			frontPage == "profile-form" ||
			frontPage == "splash-test" ||
			frontPage == "visibility-query" ||
			frontPage == "query-templates" ||
			frontPage == "template-input" ||
			frontPage == "date-range" ||
			frontPage == "saved-filters" ||
			frontPage == "save-filter" ||
			frontPage == "diff-input" ||
			frontPage == "event-detail" {
			return event // Let the modal handle it
		}

		// Global quit
		if event.Rune() == 'q' {
			if a.ui.Pages().Depth() <= 1 {
				a.Stop()
				return nil
			}
		}

		// Global back navigation
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if a.ui.Pages().CanPop() {
				a.ui.Pages().Pop()
				// Restore focus to the now-current view
				if current := a.ui.Pages().Current(); current != nil {
					a.ui.SetFocus(current)
				}
				return nil
			}
		}

		// Help
		if event.Rune() == '?' {
			a.showHelp()
			return nil
		}

		// Theme selector (capital T)
		if event.Rune() == 'T' {
			a.showThemeSelector()
			return nil
		}

		// Profile selector (capital P)
		if event.Rune() == 'P' {
			a.ShowProfileSelector()
			return nil
		}

		// Command bar (: key)
		if event.Rune() == ':' {
			a.showCommandBar()
			return nil
		}

		// Dev mode: splash screen test (capital S)
		if a.devMode && event.Rune() == 'S' {
			a.showSplashTest()
			return nil
		}

		return event
	})

	// Create and push the home view
	a.namespaceList = NewNamespaceList(a)
	a.ui.Pages().Push(a.namespaceList)
}

func (a *App) updateCrumbs() {
	current := a.ui.Pages().Current()
	if current == nil {
		return
	}

	var path []string
	switch current.Name() {
	case "namespaces":
		path = []string{"Namespaces"}
	case "workflows":
		path = []string{"Namespaces", a.currentNS, "Workflows"}
	case "workflow-detail":
		path = []string{"Namespaces", a.currentNS, "Workflows", "Detail"}
	case "events":
		path = []string{"Namespaces", a.currentNS, "Workflows", "Detail", "Events"}
	case "task-queues":
		path = []string{"Namespaces", a.currentNS, "Task Queues"}
	case "schedules":
		path = []string{"Namespaces", a.currentNS, "Schedules"}
	case "workflow-diff":
		path = []string{"Namespaces", a.currentNS, "Workflows", "Diff"}
	}
	a.ui.Crumbs().SetPath(path)
}

// UI returns the underlying UI application.
func (a *App) UI() *ui.App {
	return a.ui
}

// Provider returns the Temporal provider.
func (a *App) Provider() temporal.Provider {
	return a.provider
}

// SetNamespace sets the current namespace context.
func (a *App) SetNamespace(ns string) {
	a.currentNS = ns
	a.ui.StatsBar().SetNamespace(ns)
}

// CurrentNamespace returns the current namespace.
func (a *App) CurrentNamespace() string {
	return a.currentNS
}

// NavigateToWorkflows pushes the workflow list view.
func (a *App) NavigateToWorkflows(namespace string) {
	a.SetNamespace(namespace)
	wl := NewWorkflowList(a, namespace)
	a.ui.Pages().Push(wl)
}

// NavigateToWorkflowDetail pushes the workflow detail view.
func (a *App) NavigateToWorkflowDetail(workflowID, runID string) {
	wd := NewWorkflowDetail(a, workflowID, runID)
	a.ui.Pages().Push(wd)
}

// NavigateToEvents pushes the event history view.
func (a *App) NavigateToEvents(workflowID, runID string) {
	ev := NewEventHistory(a, workflowID, runID)
	a.ui.Pages().Push(ev)
}

// NavigateToTaskQueues pushes the task queue view.
func (a *App) NavigateToTaskQueues() {
	tq := NewTaskQueueView(a)
	a.ui.Pages().Push(tq)
}

// NavigateToSchedules pushes the schedule list view.
func (a *App) NavigateToSchedules() {
	sl := NewScheduleList(a, a.currentNS)
	a.ui.Pages().Push(sl)
}

// NavigateToNamespaceDetail pushes the namespace detail view.
func (a *App) NavigateToNamespaceDetail(namespace string) {
	nd := NewNamespaceDetail(a, namespace)
	a.ui.Pages().Push(nd)
}

// NavigateToWorkflowDiff pushes the workflow diff view.
func (a *App) NavigateToWorkflowDiff(workflowA, workflowB *temporal.Workflow) {
	wd := NewWorkflowDiffWithWorkflows(a, a.currentNS, workflowA, workflowB)
	a.ui.Pages().Push(wd)
}

// NavigateToWorkflowDiffEmpty pushes an empty workflow diff view.
func (a *App) NavigateToWorkflowDiffEmpty() {
	wd := NewWorkflowDiff(a, a.currentNS)
	a.ui.Pages().Push(wd)
}

// Run starts the application.
func (a *App) Run() error {
	// Start connection monitor if we have a provider
	if a.provider != nil && a.stopMonitor != nil {
		go a.connectionMonitor()
	}
	return a.ui.Run()
}

// connectionMonitor periodically checks the connection and attempts reconnection if needed.
func (a *App) connectionMonitor() {
	ticker := time.NewTicker(connectionCheckInterval)
	defer ticker.Stop()

	backoff := reconnectInitialBackoff

	for {
		select {
		case <-a.stopMonitor:
			return
		case <-ticker.C:
			if a.provider == nil {
				continue
			}

			// Check connection
			ctx, cancel := context.WithTimeout(context.Background(), connectionCheckTimeout)
			err := a.provider.CheckConnection(ctx)
			cancel()

			if err != nil {
				// Connection lost - update UI
				a.ui.QueueUpdateDraw(func() {
					a.ui.StatsBar().SetConnected(false)
				})

				// Attempt reconnection with backoff
				if !a.reconnecting {
					a.reconnecting = true
					go a.attemptReconnect(backoff)
					backoff = backoff * 2
					if backoff > reconnectMaxBackoff {
						backoff = reconnectMaxBackoff
					}
				}
			} else {
				// Connection is good - reset backoff
				backoff = reconnectInitialBackoff
				a.reconnecting = false

				// Ensure UI shows connected (in case we just reconnected)
				a.ui.QueueUpdateDraw(func() {
					a.ui.StatsBar().SetConnected(true)
				})
			}
		}
	}
}

// attemptReconnect tries to reconnect to the Temporal server.
func (a *App) attemptReconnect(backoff time.Duration) {
	// Wait before attempting reconnection
	select {
	case <-a.stopMonitor:
		return
	case <-time.After(backoff):
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := a.provider.Reconnect(ctx)
	cancel()

	a.ui.QueueUpdateDraw(func() {
		if err == nil {
			a.ui.StatsBar().SetConnected(true)
			a.reconnecting = false
		}
		// If reconnection failed, the next connectionMonitor tick will retry
	})
}

// Stop stops the application and connection monitor.
func (a *App) Stop() {
	if a.stopMonitor != nil {
		select {
		case <-a.stopMonitor:
			// Already closed
		default:
			close(a.stopMonitor)
		}
	}
	a.ui.Stop()
}

// SetDevMode enables or disables development mode.
func (a *App) SetDevMode(enabled bool) {
	a.devMode = enabled
}

// showSplashTest shows the splash screen for testing gradients and themes.
func (a *App) showSplashTest() {
	splash := ui.NewSplashModal()
	splash.SetOnClose(func() {
		a.closeSplashTest()
	})

	a.ui.Pages().AddPage("splash-test", splash, true, true)
	a.ui.SetFocus(splash)
}

func (a *App) closeSplashTest() {
	a.ui.Pages().RemovePage("splash-test")
	if current := a.ui.Pages().Current(); current != nil {
		a.ui.SetFocus(current)
	}
}

func (a *App) showHelp() {
	helpModal := ui.NewHelpModal()

	// Get current view's hints
	current := a.ui.Pages().Current()
	if current != nil {
		helpModal.SetViewHints(current.Name(), current.Hints())
	}

	helpModal.SetOnClose(func() {
		a.closeHelp()
	})

	a.ui.Pages().AddPage("help-modal", helpModal, true, true)
	a.ui.SetFocus(helpModal)
}

func (a *App) closeHelp() {
	a.ui.Pages().RemovePage("help-modal")
	// Restore focus to current view
	if current := a.ui.Pages().Current(); current != nil {
		a.ui.SetFocus(current)
	}
}

func (a *App) showCommandBar() {
	a.ui.CommandBar().Activate(ui.CommandAction)
	a.ui.ShowCommandBar(ui.CommandAction)

	a.ui.CommandBar().SetOnSubmit(func(cmdType ui.CommandType, text string) {
		a.ui.HideCommandBar()
		a.ui.CommandBar().Deactivate()

		// Parse the command
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "profile") {
			args := strings.TrimPrefix(text, "profile")
			a.handleProfileCommand(strings.TrimSpace(args))
		}

		// Restore focus to current view
		if current := a.ui.Pages().Current(); current != nil {
			a.ui.SetFocus(current)
		}
	})

	a.ui.CommandBar().SetOnCancel(func() {
		a.ui.HideCommandBar()
		a.ui.CommandBar().Deactivate()
		// Restore focus to current view
		if current := a.ui.Pages().Current(); current != nil {
			a.ui.SetFocus(current)
		}
	})

	a.ui.SetFocus(a.ui.CommandBar())
}

func (a *App) showThemeSelector() {
	modal := ui.NewThemeSelectorModal()

	modal.SetOnSelect(func(themeName string) {
		// Save to config
		go func() {
			cfg, _ := config.Load()
			if cfg == nil {
				cfg = config.DefaultConfig()
			}
			cfg.Theme = themeName
			_ = config.Save(cfg)
		}()
		a.closeThemeSelector()
	})

	modal.SetOnCancel(func() {
		a.closeThemeSelector()
	})

	a.ui.Pages().AddPage("theme-selector", modal, true, true)
	a.ui.SetFocus(modal)
}

func (a *App) closeThemeSelector() {
	a.ui.Pages().RemovePage("theme-selector")
	// Restore focus to current view
	if current := a.ui.Pages().Current(); current != nil {
		a.ui.SetFocus(current)
	}
}

// Profile management methods

// ShowProfileSelector opens the profile selector modal.
func (a *App) ShowProfileSelector() {
	if a.config == nil {
		return
	}

	modal := ui.NewProfileModal()
	modal.SetProfiles(a.config.ListProfiles(), a.activeProfile)
	modal.SetOnSelect(func(name string) {
		a.closeProfileSelector()
		a.SwitchProfile(name)
	})
	modal.SetOnNew(func() {
		a.closeProfileSelector()
		a.showProfileForm("")
	})
	modal.SetOnEdit(func(name string) {
		a.closeProfileSelector()
		a.showProfileForm(name)
	})
	modal.SetOnDelete(func(name string) {
		a.deleteProfile(name)
		// Refresh the modal
		modal.SetProfiles(a.config.ListProfiles(), a.activeProfile)
	})
	modal.SetOnClose(func() {
		a.closeProfileSelector()
	})

	a.ui.Pages().AddPage("profile-selector", modal, true, true)
	a.ui.SetFocus(modal)
}

func (a *App) closeProfileSelector() {
	a.ui.Pages().RemovePage("profile-selector")
	if current := a.ui.Pages().Current(); current != nil {
		a.ui.SetFocus(current)
	}
}

func (a *App) showProfileForm(editName string) {
	form := ui.NewProfileForm()

	if editName != "" {
		// Edit existing profile
		if cfg, ok := a.config.GetProfile(editName); ok {
			form.SetProfile(editName, cfg)
		}
	}

	form.SetOnSave(func(name string, cfg config.ConnectionConfig) {
		a.closeProfileForm()
		a.config.SaveProfile(name, cfg)
		if err := a.config.Save(); err != nil {
			// Log error but continue
		}
		// Optionally switch to the new/edited profile
		a.SwitchProfile(name)
	})
	form.SetOnCancel(func() {
		a.closeProfileForm()
	})

	a.ui.Pages().AddPage("profile-form", form, true, true)
	a.ui.SetFocus(form)
}

func (a *App) closeProfileForm() {
	a.ui.Pages().RemovePage("profile-form")
	if current := a.ui.Pages().Current(); current != nil {
		a.ui.SetFocus(current)
	}
}

func (a *App) deleteProfile(name string) {
	if a.config == nil {
		return
	}
	if err := a.config.DeleteProfile(name); err != nil {
		// Can't delete active profile or non-existent profile
		return
	}
	_ = a.config.Save()
}

// SwitchProfile switches to a different connection profile.
func (a *App) SwitchProfile(name string) {
	if a.config == nil || a.provider == nil {
		return
	}

	// Get the profile config
	profileCfg, ok := a.config.GetProfile(name)
	if !ok {
		return
	}

	// Convert to temporal.ConnectionConfig
	connConfig := temporal.ConnectionConfig{
		Address:       profileCfg.Address,
		Namespace:     profileCfg.Namespace,
		TLSCertPath:   profileCfg.TLS.Cert,
		TLSKeyPath:    profileCfg.TLS.Key,
		TLSCAPath:     profileCfg.TLS.CA,
		TLSServerName: profileCfg.TLS.ServerName,
		TLSSkipVerify: profileCfg.TLS.SkipVerify,
	}

	// Stop current views
	if current := a.ui.Pages().Current(); current != nil {
		current.Stop()
	}

	// Update UI to show connecting state
	a.ui.StatsBar().SetConnected(false)
	a.ui.StatsBar().SetProfile(name + " (connecting...)")

	// Reconnect in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := a.provider.ReconnectWithConfig(ctx, connConfig)
		cancel()

		a.ui.QueueUpdateDraw(func() {
			if err != nil {
				// Reconnection failed - restore previous state
				a.ui.StatsBar().SetProfile(a.activeProfile + " (failed)")
				a.ui.StatsBar().SetConnected(false)
				return
			}

			// Success - update state
			a.activeProfile = name
			a.currentNS = connConfig.Namespace
			a.config.SetActiveProfile(name)
			_ = a.config.Save()

			// Update UI
			a.ui.StatsBar().SetProfile(name)
			a.ui.StatsBar().SetConnected(true)
			a.ui.StatsBar().SetNamespace(connConfig.Namespace)

			// Reinitialize views - go back to namespace list
			a.reinitializeViews()
		})
	}()
}

// reinitializeViews resets the view stack after a profile switch.
func (a *App) reinitializeViews() {
	// Clear all views from the stack completely
	a.ui.Pages().Clear()

	// Create fresh namespace list and push it
	a.namespaceList = NewNamespaceList(a)
	a.ui.Pages().Push(a.namespaceList)
	a.ui.SetFocus(a.namespaceList)
}

// HandleCommand handles command bar commands like :profile.
func (a *App) HandleCommand(cmdType ui.CommandType, text string) {
	switch cmdType {
	case ui.CommandProfile:
		a.handleProfileCommand(text)
	case ui.CommandAction:
		// Handle general action commands
		if strings.HasPrefix(text, "profile") {
			args := strings.TrimPrefix(text, "profile")
			args = strings.TrimSpace(args)
			a.handleProfileCommand(args)
		}
	}
}

func (a *App) handleProfileCommand(args string) {
	args = strings.TrimSpace(args)

	if args == "" {
		// No args - show profile selector
		a.ShowProfileSelector()
		return
	}

	parts := strings.Fields(args)
	cmd := parts[0]

	switch cmd {
	case "new":
		a.showProfileForm("")
	case "edit":
		if len(parts) > 1 {
			a.showProfileForm(parts[1])
		} else {
			a.showProfileForm(a.activeProfile)
		}
	case "delete":
		if len(parts) > 1 {
			a.deleteProfile(parts[1])
		}
	case "save":
		// Save current connection as new profile
		a.showProfileForm("")
	default:
		// Treat as profile name - switch to it
		if a.config != nil && a.config.ProfileExists(cmd) {
			a.SwitchProfile(cmd)
		}
	}
}

// ActiveProfile returns the currently active profile name.
func (a *App) ActiveProfile() string {
	return a.activeProfile
}

// Config returns the app configuration.
func (a *App) Config() *config.Config {
	return a.config
}
