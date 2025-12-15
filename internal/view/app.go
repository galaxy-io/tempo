package view

import (
	"context"
	"time"

	"github.com/atterpac/temportui/internal/temporal"
	"github.com/atterpac/temportui/internal/ui"
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
func NewAppWithProvider(provider temporal.Provider, defaultNamespace string) *App {
	a := &App{
		ui:          ui.NewApp(),
		provider:    provider,
		currentNS:   defaultNamespace,
		stopMonitor: make(chan struct{}),
	}
	a.setup()
	// Set initial connection status based on provider
	if provider != nil {
		a.ui.StatsBar().SetConnected(provider.IsConnected())
	}
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
				return nil
			}
		}

		// Help
		if event.Rune() == '?' {
			a.showHelp()
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

func (a *App) showHelp() {
	// TODO: Implement help modal
	// For now, the key hints in the menu bar serve as help
}
