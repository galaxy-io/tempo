package view

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/layout"
	"github.com/atterpac/jig/nav"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/theme/themes"
	"github.com/galaxy-io/tempo/internal/config"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/galaxy-io/tempo/internal/update"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	connectionCheckInterval = 10 * time.Second
	reconnectInitialBackoff = 2 * time.Second
	reconnectMaxBackoff     = 30 * time.Second
	connectionCheckTimeout  = 5 * time.Second
)

// App is the main application controller.
type App struct {
	app           *layout.App
	statusBar     *layout.StatusBar
	menu          *layout.Menu
	toasts        *components.ToastManager
	namespaceList *NamespaceList

	// Protected by mu - accessed from multiple goroutines
	mu            sync.RWMutex
	provider      temporal.Provider
	currentNS     string
	activeProfile string
	reconnecting  bool

	// Connection monitor
	stopMonitor chan struct{}

	// Profile management
	config *config.Config

	// Dev mode
	devMode bool
}

// NewApp creates a new application controller with no provider (uses mock data).
func NewApp() *App {
	a := &App{
		currentNS: "default",
	}
	a.buildApp()
	a.setup()
	return a
}

// NewAppWithProvider creates a new application controller with a Temporal provider.
func NewAppWithProvider(provider temporal.Provider, defaultNamespace string, cfg *config.Config, activeProfile string) *App {
	a := &App{
		provider:      provider,
		currentNS:     defaultNamespace,
		stopMonitor:   make(chan struct{}),
		config:        cfg,
		activeProfile: activeProfile,
	}
	a.buildApp()
	a.setup()

	// Set initial profile name in stats bar (must be first - clears sections)
	a.setProfile(activeProfile)
	// Set initial connection status based on provider (adds section 2)
	if provider != nil {
		a.setConnected(provider.IsConnected())
	}
	return a
}

func (a *App) buildApp() {
	// Create status bar with left-aligned title and content
	a.statusBar = layout.NewStatusBar()
	a.statusBar.SetTitle("tempo")
	a.statusBar.SetTitleAlign(components.AlignLeft)
	a.statusBar.SetContentAlign(components.AlignLeft)

	// Create menu
	a.menu = layout.NewMenu()

	// Create app with jig layout
	a.app = layout.NewApp(layout.AppConfig{
		TopBar:       a.statusBar,
		TopBarHeight: 3,
		ShowCrumbs:   true,
		BottomBar:    a.menu,
		OnComponentChange: func(c nav.Component) {
			if c != nil {
				a.menu.SetHints(c.Hints())
			}
			a.updateCrumbs()
		},
	})

	// Create toast manager for notifications
	a.toasts = components.NewToastManager(a.app.GetApplication())
	a.toasts.SetPosition(components.ToastBottomRight)

	// Wire up toast rendering as an overlay
	a.app.GetApplication().SetAfterDrawFunc(func(screen tcell.Screen) {
		w, h := screen.Size()
		a.toasts.Draw(screen, w, h)
	})
}

func (a *App) setup() {
	// Set up command bar callbacks
	a.statusBar.SetOnCommandSubmit(func(text string) {
		a.statusBar.ExitCommandMode()
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "profile") {
			args := strings.TrimPrefix(text, "profile")
			a.handleProfileCommand(strings.TrimSpace(args))
		}
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})

	a.statusBar.SetOnCommandCancel(func() {
		a.statusBar.ExitCommandMode()
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})

	// Global key handler
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Skip global handling when command bar is active
		if a.statusBar.IsCommandMode() {
			return event
		}

		// Check if we're on a modal page that should handle its own escape
		isModalPage := a.app.Pages().CurrentIsModal()
		// Fallback for views using AddPage that don't implement nav.Component
		if !isModalPage {
			if frontPage, _ := a.app.Pages().GetFrontPage(); frontPage == "splash-test" {
				isModalPage = true
			}
		}

		// Global quit (only on root view, not in modals)
		if event.Rune() == 'q' && !isModalPage {
			if a.app.Pages().StackDepth() <= 1 {
				a.Stop()
				return nil
			}
		}

		// Global back navigation (skip for modals - they handle their own escape)
		if !isModalPage {
			if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
				// Check if current view wants to handle escape first
				if current := a.app.Pages().Current(); current != nil {
					if handler, ok := current.(EscapeHandler); ok {
						if handler.HandleEscape() {
							return nil
						}
					}
				}
				if a.app.Pages().CanPop() {
					a.app.Pages().Pop()
					if current := a.app.Pages().Current(); current != nil {
						a.app.SetFocus(current)
					}
					return nil
				}
			}
		}

		// Help (works everywhere except modals)
		if event.Rune() == '?' && !isModalPage {
			a.showHelp()
			return nil
		}

		// Theme selector (capital T) - works everywhere except modals
		if event.Rune() == 'T' && !isModalPage {
			a.showThemeSelector()
			return nil
		}

		// Profile selector (capital P) - works everywhere except modals
		if event.Rune() == 'P' && !isModalPage {
			a.ShowProfileSelector()
			return nil
		}

		// Command bar (: key) - works everywhere except modals
		if event.Rune() == ':' && !isModalPage {
			a.showCommandBar()
			return nil
		}

		// Dev mode: splash screen test (capital S)
		if a.devMode && event.Rune() == 'S' {
			a.showSplashTest()
			return nil
		}

		// Debug screen (!) - works everywhere except modals
		if event.Rune() == '!' && !isModalPage {
			a.showDebugScreen()
			return nil
		}

		return event
	})

	// Create and push the home view
	// If using API key auth, skip namespace list and go directly to workflows
	// (API keys are typically namespace-scoped and can't list all namespaces)
	if a.provider != nil && a.provider.Config().APIKey != "" {
		wl := NewWorkflowList(a, a.currentNS)
		a.app.Pages().Push(wl)
	} else {
		a.namespaceList = NewNamespaceList(a)
		a.app.Pages().Push(a.namespaceList)
	}
}

func (a *App) updateCrumbs() {
	current := a.app.Pages().Current()
	if current == nil || a.app.Crumbs() == nil {
		return
	}

	var path []string
	if named, ok := current.(interface{ Name() string }); ok {
		switch named.Name() {
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
	}
	a.app.Crumbs().SetPath(path)
}

// Status bar helpers
// Section layout: [0] profile, [1] namespace, [2] connection status

func (a *App) setConnected(connected bool) {
	icon := theme.IconDisconnected
	text := "disconnected"
	colorFunc := theme.Error
	if connected {
		icon = theme.IconConnected
		text = "connected"
		colorFunc = theme.Success
	}

	section := layout.StatusSection{
		Icon:      icon,
		Text:      text,
		ColorFunc: colorFunc,
	}

	// Connection status is section 2
	if a.statusBar.SectionCount() >= 3 {
		a.statusBar.UpdateSection(2, section)
	} else {
		a.statusBar.AddSection(section)
	}
}

func (a *App) setProfile(name string) {
	a.statusBar.ClearSections()
	// Section 0: profile (accent color, no icon)
	a.statusBar.AddSection(layout.StatusSection{
		Text:      name,
		ColorFunc: theme.Accent,
	})
	// Section 1: namespace (no icon)
	a.statusBar.AddSection(layout.StatusSection{
		Text: a.currentNS,
	})
	// Section 2: connection status (will be set by setConnected)
}

func (a *App) setNamespace(ns string) {
	// Namespace is section 1 (no icon)
	a.statusBar.UpdateSection(1, layout.StatusSection{
		Text: ns,
	})
}

// WorkflowStats holds workflow count statistics.
type WorkflowStats struct {
	Running   int
	Completed int
	Failed    int
}

// SetWorkflowStats updates the workflow statistics in the status bar (right-aligned).
func (a *App) SetWorkflowStats(stats WorkflowStats) {
	// Clear existing right sections and add new stats
	a.statusBar.ClearRightSections()

	// Format: dimmed label, colored number
	dimTag := theme.TagFgDim()
	runningColor := theme.TagInfo()
	completedColor := theme.TagSuccess()
	failedColor := theme.TagError()

	a.statusBar.AddRightSection(layout.StatusSection{
		Text: fmt.Sprintf("[%s]Running:[-] [%s]%d[-]", dimTag, runningColor, stats.Running),
	})
	a.statusBar.AddRightSection(layout.StatusSection{
		Text: fmt.Sprintf("[%s]Completed:[-] [%s]%d[-]", dimTag, completedColor, stats.Completed),
	})
	a.statusBar.AddRightSection(layout.StatusSection{
		Text: fmt.Sprintf("[%s]Failed:[-] [%s]%d[-]", dimTag, failedColor, stats.Failed),
	})
}

// ClearWorkflowStats removes workflow statistics from the status bar.
func (a *App) ClearWorkflowStats() {
	a.statusBar.ClearRightSections()
}

// App returns the underlying jig layout.App.
func (a *App) JigApp() *layout.App {
	return a.app
}

// Provider returns the Temporal provider.
// Thread-safe: can be called from any goroutine.
func (a *App) Provider() temporal.Provider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.provider
}

// SetNamespace sets the current namespace context.
// Thread-safe: can be called from any goroutine.
func (a *App) SetNamespace(ns string) {
	a.mu.Lock()
	a.currentNS = ns
	a.mu.Unlock()
	a.setNamespace(ns)
}

// CurrentNamespace returns the current namespace.
// Thread-safe: can be called from any goroutine.
func (a *App) CurrentNamespace() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentNS
}

// NavigateToWorkflows pushes the workflow list view.
func (a *App) NavigateToWorkflows(namespace string) {
	a.SetNamespace(namespace)
	wl := NewWorkflowList(a, namespace)
	a.app.Pages().Push(wl)
}

// NavigateToWorkflowDetail pushes the workflow detail view.
func (a *App) NavigateToWorkflowDetail(workflowID, runID string) {
	wd := NewWorkflowDetail(a, workflowID, runID)
	a.app.Pages().Push(wd)
}

// NavigateToEvents pushes the event history view.
func (a *App) NavigateToEvents(workflowID, runID string) {
	ev := NewEventHistory(a, workflowID, runID)
	a.app.Pages().Push(ev)
}

// NavigateToTaskQueues pushes the task queue view.
func (a *App) NavigateToTaskQueues() {
	tq := NewTaskQueueView(a)
	a.app.Pages().Push(tq)
}

// NavigateToSchedules pushes the schedule list view.
func (a *App) NavigateToSchedules() {
	sl := NewScheduleList(a, a.CurrentNamespace())
	a.app.Pages().Push(sl)
}

// NavigateToNamespaceDetail pushes the namespace detail view.
func (a *App) NavigateToNamespaceDetail(namespace string) {
	nd := NewNamespaceDetail(a, namespace)
	a.app.Pages().Push(nd)
}

// NavigateToWorkflowDiff pushes the workflow diff view.
func (a *App) NavigateToWorkflowDiff(workflowA, workflowB *temporal.Workflow) {
	wd := NewWorkflowDiffWithWorkflows(a, a.CurrentNamespace(), workflowA, workflowB)
	a.app.Pages().Push(wd)
}

// NavigateToWorkflowDiffEmpty pushes an empty workflow diff view.
func (a *App) NavigateToWorkflowDiffEmpty() {
	wd := NewWorkflowDiff(a, a.CurrentNamespace())
	a.app.Pages().Push(wd)
}

// NavigateToWorkflowGraph pushes the workflow graph view.
func (a *App) NavigateToWorkflowGraph(workflow *temporal.Workflow) {
	wg := NewWorkflowGraphView(a, a.CurrentNamespace(), workflow)
	a.app.Pages().Push(wg)
}

// Run starts the application.
func (a *App) Run() error {
	// Start connection monitor if we have a provider
	a.mu.RLock()
	hasProvider := a.provider != nil
	a.mu.RUnlock()

	if hasProvider && a.stopMonitor != nil {
		go a.connectionMonitor()
	}

	// Check for updates if enabled
	if a.config != nil && a.config.ShouldCheckUpdates() {
		go a.checkForUpdates()
	}

	return a.app.Run()
}

// checkForUpdates checks for updates and automatically applies them.
func (a *App) checkForUpdates() {
	// Skip auto-update for Homebrew installs - use `brew upgrade` instead
	if update.IsHomebrewInstall() {
		return
	}

	updater := update.NewUpdater()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := updater.CheckForUpdate(ctx)
	if err != nil {
		// Silent failure - don't bother user with update check errors
		return
	}

	if !info.NeedsUpdate {
		return
	}

	// Apply update automatically
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer updateCancel()

	if err := updater.ApplyUpdate(updateCtx, info); err != nil {
		return
	}

	a.app.QueueUpdateDraw(func() {
		a.toasts.Success("Updated, restart plz " + theme.IconHeart)
	})
}

// ShowToastError displays an error toast notification.
func (a *App) ShowToastError(message string) {
	a.app.QueueUpdateDraw(func() {
		a.toasts.Error(message)
	})
}

// ShowToastWarning displays a warning toast notification.
func (a *App) ShowToastWarning(message string) {
	a.app.QueueUpdateDraw(func() {
		a.toasts.Warning(message)
	})
}

// ShowToastSuccess displays a success toast notification.
// Use ToastSuccess instead if already inside QueueUpdateDraw.
func (a *App) ShowToastSuccess(message string) {
	a.app.QueueUpdateDraw(func() {
		a.toasts.Success(message)
	})
}

// ToastSuccess displays a success toast (call from within QueueUpdateDraw).
func (a *App) ToastSuccess(message string) {
	a.toasts.Success(message)
}

// ToastError displays an error toast (call from within QueueUpdateDraw).
func (a *App) ToastError(message string) {
	a.toasts.Error(message)
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
			// Get provider with lock
			a.mu.RLock()
			provider := a.provider
			a.mu.RUnlock()

			if provider == nil {
				continue
			}

			// Check connection
			ctx, cancel := context.WithTimeout(context.Background(), connectionCheckTimeout)
			err := provider.CheckConnection(ctx)
			cancel()

			if err != nil {
				// Connection lost - update UI
				a.app.QueueUpdateDraw(func() {
					a.setConnected(false)
				})

				// Attempt reconnection with backoff
				a.mu.Lock()
				shouldReconnect := !a.reconnecting
				if shouldReconnect {
					a.reconnecting = true
				}
				a.mu.Unlock()

				if shouldReconnect {
					go a.attemptReconnect(backoff)
					backoff = backoff * 2
					if backoff > reconnectMaxBackoff {
						backoff = reconnectMaxBackoff
					}
				}
			} else {
				// Connection is good - reset backoff
				backoff = reconnectInitialBackoff
				a.mu.Lock()
				a.reconnecting = false
				a.mu.Unlock()

				// Ensure UI shows connected
				a.app.QueueUpdateDraw(func() {
					a.setConnected(true)
				})
			}
		}
	}
}

// attemptReconnect tries to reconnect to the Temporal server.
func (a *App) attemptReconnect(backoff time.Duration) {
	select {
	case <-a.stopMonitor:
		return
	case <-time.After(backoff):
	}

	// Get provider with lock
	a.mu.RLock()
	provider := a.provider
	a.mu.RUnlock()

	if provider == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := provider.Reconnect(ctx)
	cancel()

	a.app.QueueUpdateDraw(func() {
		if err == nil {
			a.setConnected(true)
			a.mu.Lock()
			a.reconnecting = false
			a.mu.Unlock()
		}
	})
}

// Stop stops the application and connection monitor.
func (a *App) Stop() {
	if a.stopMonitor != nil {
		select {
		case <-a.stopMonitor:
		default:
			close(a.stopMonitor)
		}
	}
	a.app.Stop()
}

// SetDevMode enables or disables development mode.
func (a *App) SetDevMode(enabled bool) {
	a.devMode = enabled
}

// showSplashTest shows the splash screen for testing gradients and themes.
func (a *App) showSplashTest() {
	currentTheme := "tokyonight-night"
	if a.config != nil && a.config.Theme != "" {
		currentTheme = a.config.Theme
	}

	splash := NewSplashTestView(currentTheme)
	splash.SetOnClose(func() {
		a.closeSplashTest()
	})
	splash.SetOnThemeChange(func(themeName string) {
		// Update config with new theme
		if a.config != nil {
			a.config.Theme = themeName
		}
		// Refresh theme colors across the app
		a.app.RefreshTheme()
	})

	a.app.Pages().AddPage("splash-test", splash, true, true)
	a.app.SetFocus(splash)
}

func (a *App) closeSplashTest() {
	a.app.Pages().RemovePage("splash-test")
	if current := a.app.Pages().Current(); current != nil {
		a.app.SetFocus(current)
	}
}

func (a *App) showHelp() {
	helpModal := NewHelpModal()

	// Get current view's hints
	current := a.app.Pages().Current()
	if current != nil {
		if named, ok := current.(interface{ Name() string }); ok {
			helpModal.SetViewHints(named.Name(), current.Hints())
		}
	}

	helpModal.SetOnClose(func() {
		a.closeHelp()
	})

	a.app.Pages().Push(helpModal)
	a.app.SetFocus(helpModal)
}

func (a *App) closeHelp() {
	a.app.Pages().DismissModal()
}

func (a *App) closeThemeSelector() {
	a.app.Pages().DismissModal()
}

func (a *App) showDebugScreen() {
	// Build debug data from current app state
	data := DebugData{
		Version:     update.Version,
		Commit:      update.Commit,
		BuildDate:   update.BuildDate,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoVersion:   runtime.Version(),
		Term:        os.Getenv("TERM"),
		ColorTerm:   os.Getenv("COLORTERM"),
		TermProgram: os.Getenv("TERM_PROGRAM"),
		ConfigPath:  config.ConfigPath(),
		ThemeName:   a.config.Theme,
		ProfileName: a.activeProfile,
	}

	// Get profile connection details
	if profile, ok := a.config.GetProfile(a.activeProfile); ok {
		data.ServerAddress = profile.Address
		data.Namespace = profile.Namespace
		data.TLSEnabled = profile.TLS.Cert != "" || profile.TLS.CA != ""
		data.TLSCertPath = profile.TLS.Cert
		data.TLSKeyPath = profile.TLS.Key
		data.TLSCAPath = profile.TLS.CA
	}

	// Detect color space from environment
	colorTerm := os.Getenv("COLORTERM")
	term := os.Getenv("TERM")
	switch {
	case colorTerm == "truecolor" || colorTerm == "24bit":
		data.ColorSpace = "truecolor (24-bit)"
	case strings.Contains(term, "256color"):
		data.ColorSpace = "256 colors"
	default:
		data.ColorSpace = "unknown"
	}

	// Create and push debug screen
	debugScreen := NewDebugScreen(data)
	a.app.Pages().Push(debugScreen)
}

func (a *App) showCommandBar() {
	a.statusBar.SetCommandPrompt(": ")
	a.statusBar.SetCommandPlaceholder("command...")
	a.statusBar.EnterCommandMode()
	a.app.SetFocus(a.statusBar.GetCommandInput())
}

func (a *App) showThemeSelector() {
	// Get current theme name from config
	currentTheme := "tokyonight-night"
	if a.config != nil && a.config.Theme != "" {
		currentTheme = a.config.Theme
	}
	originalTheme := currentTheme

	// Separate themes into dark and light categories
	allThemes := config.ThemeNames()
	var darkThemes, lightThemes []string
	for _, name := range allThemes {
		if t, ok := config.BuiltinThemes[name]; ok {
			if t.Type == "light" {
				lightThemes = append(lightThemes, name)
			} else {
				darkThemes = append(darkThemes, name)
			}
		}
	}

	// Create modal with backdrop disabled so dashboard is visible for live preview
	modal := components.NewModal(components.ModalConfig{
		Title:    "Select Theme",
		Width:    30,
		Height:   22,
		Backdrop: false,
	})

	// Create a list for theme selection
	list := tview.NewList()
	bg := theme.Bg()
	list.SetBackgroundColor(bg)
	list.SetMainTextColor(theme.Fg())
	list.SetMainTextStyle(tcell.StyleDefault.Background(bg).Foreground(theme.Fg()))
	list.SetSelectedBackgroundColor(theme.Accent())
	list.SetSelectedTextColor(bg)
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Accent()).Foreground(bg))
	list.SetHighlightFullLine(true)
	list.ShowSecondaryText(false)

	// Track mapping from list index to theme name (for headers)
	listToTheme := make(map[int]string)
	listIdx := 0

	// Find current theme index for marker
	currentIdx := -1
	for i, name := range allThemes {
		if name == currentTheme {
			currentIdx = i
			break
		}
	}

	// Add dark themes header
	list.AddItem("[::d]─── Dark ───[-::-]", "", 0, nil)
	listIdx++

	// Add dark themes
	for _, themeName := range darkThemes {
		name := themeName // capture for closure
		prefix := "  "
		if name == currentTheme {
			prefix = "● "
		}
		listToTheme[listIdx] = name
		list.AddItem(prefix+name, "", 0, func() {
			newTheme := themes.Get(name)
			if newTheme != nil {
				theme.SetProvider(newTheme) // Auto-refreshes all registered views
			}
			// Save theme to config
			go func() {
				cfg, _ := config.Load()
				if cfg == nil {
					cfg = config.DefaultConfig()
				}
				cfg.Theme = name
				_ = config.Save(cfg)
			}()
			a.closeThemeSelector()
		})
		listIdx++
	}

	// Add light themes header
	list.AddItem("[::d]─── Light ───[-::-]", "", 0, nil)
	lightHeaderIdx := listIdx
	listIdx++

	// Add light themes
	for _, themeName := range lightThemes {
		name := themeName // capture for closure
		prefix := "  "
		if name == currentTheme {
			prefix = "● "
		}
		listToTheme[listIdx] = name
		list.AddItem(prefix+name, "", 0, func() {
			newTheme := themes.Get(name)
			if newTheme != nil {
				theme.SetProvider(newTheme) // Auto-refreshes all registered views
			}
			// Save theme to config
			go func() {
				cfg, _ := config.Load()
				if cfg == nil {
					cfg = config.DefaultConfig()
				}
				cfg.Theme = name
				_ = config.Save(cfg)
			}()
			a.closeThemeSelector()
		})
		listIdx++
	}

	// Find list index for current theme
	currentListIdx := 1 // Start after dark header
	if currentIdx >= 0 {
		// Find it in the correct section
		for idx, themeName := range listToTheme {
			if themeName == currentTheme {
				currentListIdx = idx
				break
			}
		}
	}
	list.SetCurrentItem(currentListIdx)

	// Live preview on navigation
	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if themeName, ok := listToTheme[index]; ok {
			newTheme := themes.Get(themeName)
			if newTheme != nil {
				theme.SetProvider(newTheme) // Auto-refreshes all registered views
				// Update list colors for new theme
				newBg := theme.Bg()
				list.SetBackgroundColor(newBg)
				list.SetMainTextColor(theme.Fg())
				list.SetMainTextStyle(tcell.StyleDefault.Background(newBg).Foreground(theme.Fg()))
				list.SetSelectedBackgroundColor(theme.Accent())
				list.SetSelectedTextColor(newBg)
				list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Accent()).Foreground(newBg))
			}
		}
	})

	modal.SetContent(list).
		SetHints([]components.KeyHint{
			{Key: "j/k", Description: "Navigate"},
			{Key: "Enter", Description: "Select"},
			{Key: "Esc", Description: "Cancel"},
		}).
		SetOnCancel(func() {
			// Restore original theme on cancel
			origTheme := themes.Get(originalTheme)
			if origTheme != nil {
				theme.SetProvider(origTheme) // Auto-refreshes all registered views
			}
			a.closeThemeSelector()
		})

	// Handle vim navigation and escape in the list
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		current := list.GetCurrentItem()

		// Handle Escape and q to cancel
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Restore original theme on cancel
			origTheme := themes.Get(originalTheme)
			if origTheme != nil {
				theme.SetProvider(origTheme) // Auto-refreshes all registered views
			}
			a.closeThemeSelector()
			return nil
		}

		switch event.Rune() {
		case 'j':
			next := current + 1
			// Skip light header
			if next == lightHeaderIdx {
				next++
			}
			if next < list.GetItemCount() {
				list.SetCurrentItem(next)
			}
			return nil
		case 'k':
			prev := current - 1
			// Skip headers
			if prev == lightHeaderIdx {
				prev--
			}
			if prev == 0 { // dark header
				prev-- // Will be -1, handled below
			}
			if prev >= 1 { // Don't go above first theme (index 1)
				list.SetCurrentItem(prev)
			}
			return nil
		}
		return event
	})

	// Use AddPage with explicit name so global InputCapture knows to skip Escape handling
	a.app.Pages().Push(modal)
	a.app.SetFocus(list)
}

// refreshCurrentView calls RefreshTheme on the current view if it supports it.
//
// Deprecated: As of jig v0.0.6, theme.SetProvider() automatically calls RefreshTheme()
// on all registered Refreshable components and triggers a redraw. This method is no
// longer needed for theme switching. Kept for backwards compatibility.
func (a *App) refreshCurrentView() {
	if current := a.app.Pages().Current(); current != nil {
		if refreshable, ok := current.(interface{ RefreshTheme() }); ok {
			refreshable.RefreshTheme()
		}
	}
}

// Profile management methods

// ShowProfileSelector opens the profile selector modal.
func (a *App) ShowProfileSelector() {
	if a.config == nil {
		return
	}

	modal := NewProfileModal()
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
		modal.SetProfiles(a.config.ListProfiles(), a.activeProfile)
	})
	modal.SetOnClose(func() {
		a.closeProfileSelector()
	})

	a.app.Pages().Push(modal)
	a.app.SetFocus(modal)
}

func (a *App) closeProfileSelector() {
	a.app.Pages().DismissModal()
}

func (a *App) showProfileForm(editName string) {
	form := NewProfileForm()

	if editName != "" {
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
		a.SwitchProfile(name)
	})
	form.SetOnCancel(func() {
		a.closeProfileForm()
	})

	a.app.Pages().Push(form)
	a.app.SetFocus(form)
}

func (a *App) closeProfileForm() {
	a.app.Pages().DismissModal()
}

func (a *App) deleteProfile(name string) {
	if a.config == nil {
		return
	}
	if err := a.config.DeleteProfile(name); err != nil {
		return
	}
	_ = a.config.Save()
}

// SwitchProfile switches to a different connection profile.
func (a *App) SwitchProfile(name string) {
	a.mu.RLock()
	provider := a.provider
	currentProfile := a.activeProfile
	a.mu.RUnlock()

	if a.config == nil || provider == nil {
		return
	}

	profileCfg, ok := a.config.GetProfile(name)
	if !ok {
		return
	}
	profileCfg = profileCfg.ExpandEnv()

	connConfig := temporal.ConnectionConfig{
		Address:       profileCfg.Address,
		Namespace:     profileCfg.Namespace,
		TLSCertPath:   profileCfg.TLS.Cert,
		TLSKeyPath:    profileCfg.TLS.Key,
		TLSCAPath:     profileCfg.TLS.CA,
		TLSServerName: profileCfg.TLS.ServerName,
		TLSSkipVerify: profileCfg.TLS.SkipVerify,
		APIKey:        profileCfg.APIKey,
	}

	// Stop current views
	if current := a.app.Pages().Current(); current != nil {
		current.Stop()
	}

	// Update UI to show connecting state (setProfile must be first - clears sections)
	a.setProfile(name + " (connecting...)")
	a.setConnected(false)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := provider.ReconnectWithConfig(ctx, connConfig)
		cancel()

		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.setProfile(currentProfile + " (failed)")
				a.setConnected(false)
				return
			}

			a.mu.Lock()
			a.activeProfile = name
			a.currentNS = connConfig.Namespace
			a.mu.Unlock()

			a.config.SetActiveProfile(name)
			_ = a.config.Save()

			a.setProfile(name)
			a.setConnected(true)
			a.setNamespace(connConfig.Namespace)

			a.reinitializeViews()
		})
	}()
}

// reinitializeViews resets the view stack after a profile switch.
func (a *App) reinitializeViews() {
	a.app.Pages().Clear()

	// If using API key auth, skip namespace list and go directly to workflows
	// (API keys are typically namespace-scoped and can't list all namespaces)
	if a.provider != nil && a.provider.Config().APIKey != "" {
		wl := NewWorkflowList(a, a.currentNS)
		a.app.Pages().Push(wl)
		a.app.SetFocus(wl)
	} else {
		a.namespaceList = NewNamespaceList(a)
		a.app.Pages().Push(a.namespaceList)
		a.app.SetFocus(a.namespaceList)
	}
}

func (a *App) handleProfileCommand(args string) {
	args = strings.TrimSpace(args)

	if args == "" {
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
		a.showProfileForm("")
	default:
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

// FilterModeCallbacks holds callbacks for filter mode.
type FilterModeCallbacks struct {
	OnSubmit func(text string)
	OnCancel func()
	OnChange func(text string)
}

// filterModeActive tracks if we're in filter mode with custom callbacks.
var filterModeCallbacks *FilterModeCallbacks

// ShowFilterMode enters filter mode with custom callbacks.
// The filter input replaces the status bar content with a "/" prompt.
func (a *App) ShowFilterMode(initialText string, callbacks FilterModeCallbacks) {
	filterModeCallbacks = &callbacks

	a.statusBar.SetCommandPrompt("/ ")
	a.statusBar.SetCommandPlaceholder("Filter workflows...")

	// Set up the callbacks
	a.statusBar.SetOnCommandSubmit(func(text string) {
		a.statusBar.ExitCommandMode()
		filterModeCallbacks = nil
		// Restore default callbacks
		a.restoreDefaultCommandCallbacks()
		if callbacks.OnSubmit != nil {
			callbacks.OnSubmit(text)
		}
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})

	a.statusBar.SetOnCommandCancel(func() {
		a.statusBar.ExitCommandMode()
		filterModeCallbacks = nil
		// Restore default callbacks
		a.restoreDefaultCommandCallbacks()
		if callbacks.OnCancel != nil {
			callbacks.OnCancel()
		}
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})

	a.statusBar.EnterCommandMode()

	// Set initial text if provided
	if initialText != "" {
		a.statusBar.GetCommandInput().SetText(initialText)
	}

	a.app.SetFocus(a.statusBar.GetCommandInput())

	// Set up change handler via input field's changed func
	if callbacks.OnChange != nil {
		a.statusBar.GetCommandInput().SetChangedFunc(func(text string) {
			callbacks.OnChange(text)
		})
	}
}

// ExitFilterMode exits filter mode and restores default command bar behavior.
func (a *App) ExitFilterMode() {
	if a.statusBar.IsCommandMode() {
		a.statusBar.ClearSuggestion()
		a.statusBar.ExitCommandMode()
	}
	filterModeCallbacks = nil
	a.restoreDefaultCommandCallbacks()
	// Restore focus to current view
	if current := a.app.Pages().Current(); current != nil {
		a.app.SetFocus(current)
	}
}

// SetFilterSuggestion sets the inline ghost text suggestion for the filter input.
// The suggestion should be the full text (what the user typed + completion).
func (a *App) SetFilterSuggestion(suggestion string) {
	a.statusBar.SetSuggestion(suggestion)
}

// IsFilterMode returns whether filter mode is active.
func (a *App) IsFilterMode() bool {
	return filterModeCallbacks != nil && a.statusBar.IsCommandMode()
}

// restoreDefaultCommandCallbacks restores the default command bar callbacks.
func (a *App) restoreDefaultCommandCallbacks() {
	a.statusBar.SetCommandPrompt(": ")
	a.statusBar.SetCommandPlaceholder("command...")
	a.statusBar.GetCommandInput().SetChangedFunc(nil)

	a.statusBar.SetOnCommandSubmit(func(text string) {
		a.statusBar.ExitCommandMode()
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "profile") {
			args := strings.TrimPrefix(text, "profile")
			a.handleProfileCommand(strings.TrimSpace(args))
		}
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})

	a.statusBar.SetOnCommandCancel(func() {
		a.statusBar.ExitCommandMode()
		// Restore focus to current view
		if current := a.app.Pages().Current(); current != nil {
			a.app.SetFocus(current)
		}
	})
}

// EscapeHandler is implemented by views that want to handle escape key.
type EscapeHandler interface {
	HandleEscape() bool
}

// KeyHint re-exports jig's KeyHint for convenience.
type KeyHint = components.KeyHint
