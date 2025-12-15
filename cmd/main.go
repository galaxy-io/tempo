package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/atterpac/temportui/internal/config"
	"github.com/atterpac/temportui/internal/temporal"
	"github.com/atterpac/temportui/internal/ui"
	"github.com/atterpac/temportui/internal/view"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CLI flags
var (
	address       = flag.String("address", "localhost:7233", "Temporal server address")
	namespace     = flag.String("namespace", "default", "Default namespace")
	tlsCert       = flag.String("tls-cert", "", "Path to TLS certificate")
	tlsKey        = flag.String("tls-key", "", "Path to TLS private key")
	tlsCA         = flag.String("tls-ca", "", "Path to CA certificate")
	tlsServerName = flag.String("tls-server-name", "", "Server name for TLS verification")
	tlsSkipVerify = flag.Bool("tls-skip-verify", false, "Skip TLS verification (insecure)")
	themeName     = flag.String("theme", "", "Theme name (overrides config file)")
)

const (
	maxRetries     = 5
	initialBackoff = 1 * time.Second
	maxBackoff     = 10 * time.Second
)

func main() {
	flag.Parse()

	// Load configuration from file
	cfg, err := config.Load()
	if err != nil {
		// Config load error is non-fatal, use defaults
		cfg = config.DefaultConfig()
	}

	// Determine theme: CLI flag overrides config file
	theme := cfg.Theme
	if *themeName != "" {
		theme = *themeName
	}

	// Initialize theme system before any UI
	if err := ui.InitTheme(theme); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load theme %q: %v, using catppuccin-mocha\n", theme, err)
		if err := ui.InitTheme("catppuccin-mocha"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize theme: %v\n", err)
			os.Exit(1)
		}
	}

	connConfig := temporal.ConnectionConfig{
		Address:       *address,
		Namespace:     *namespace,
		TLSCertPath:   *tlsCert,
		TLSKeyPath:    *tlsKey,
		TLSCAPath:     *tlsCA,
		TLSServerName: *tlsServerName,
		TLSSkipVerify: *tlsSkipVerify,
	}

	// Run connection with UI
	provider, err := connectWithUI(connConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer provider.Close()

	// Launch main application
	app := view.NewAppWithProvider(provider, *namespace)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// connectWithUI shows a connection UI while attempting to connect to Temporal.
// Returns the provider on success, or error if user quits or max retries exceeded.
func connectWithUI(config temporal.ConnectionConfig) (temporal.Provider, error) {
	app := tview.NewApplication()

	// Note: Global tview.Styles are already set by ui.InitTheme() in main()

	// Status display
	statusText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusText.SetBackgroundColor(ui.ColorBg())

	// Build layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false). // Top spacer
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false). // Left spacer
			AddItem(statusText, 60, 0, false).
			AddItem(nil, 0, 1, false), // Right spacer
			5, 0, false).
		AddItem(nil, 0, 1, false) // Bottom spacer
	flex.SetBackgroundColor(ui.ColorBg())

	// Result channels and sync
	var provider temporal.Provider
	var connErr error
	var mu sync.Mutex
	quit := make(chan struct{})
	done := make(chan struct{})
	appRunning := make(chan struct{})

	// setStatusText sets the status text content
	setStatusText := func(msg string, isError bool) {
		color := ui.TagAccent()
		if isError {
			color = ui.TagFailed()
		}
		statusText.SetText(fmt.Sprintf(
			"\n[%s]%s temporal-tui[-]\n\n[%s]%s[-]\n\n[%s]Press 'q' to quit[-]",
			ui.TagAccent(), ui.IconWorkflow,
			color, msg,
			ui.TagFgDim(),
		))
	}

	// Update status display (for use from goroutines after app is running)
	updateStatus := func(msg string, isError bool) {
		app.QueueUpdateDraw(func() {
			setStatusText(msg, isError)
		})
	}

	// Connection goroutine
	go func() {
		defer close(done)

		// Wait for app to be running before updating UI
		select {
		case <-appRunning:
		case <-quit:
			mu.Lock()
			connErr = fmt.Errorf("connection cancelled by user")
			mu.Unlock()
			return
		}

		backoff := initialBackoff
		for attempt := 1; attempt <= maxRetries; attempt++ {
			select {
			case <-quit:
				mu.Lock()
				connErr = fmt.Errorf("connection cancelled by user")
				mu.Unlock()
				return
			default:
			}

			updateStatus(fmt.Sprintf("Connecting to %s... (attempt %d/%d)", config.Address, attempt, maxRetries), false)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := temporal.NewClient(ctx, config)
			cancel()

			if err == nil {
				mu.Lock()
				provider = client
				mu.Unlock()
				updateStatus("Connected!", false)
				time.Sleep(500 * time.Millisecond) // Brief pause to show success
				app.Stop()
				return
			}

			// Connection failed
			if attempt < maxRetries {
				updateStatus(fmt.Sprintf("Connection failed: %v\nRetrying in %v...", err, backoff), true)

				select {
				case <-quit:
					mu.Lock()
					connErr = fmt.Errorf("connection cancelled by user")
					mu.Unlock()
					return
				case <-time.After(backoff):
				}

				// Exponential backoff with cap
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			} else {
				mu.Lock()
				connErr = fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
				mu.Unlock()
				updateStatus(fmt.Sprintf("Connection failed: %v\n\nMax retries exceeded. Press 'q' to exit.", err), true)
			}
		}

		// Wait for user to quit after max retries
		<-quit
	}()

	// Handle quit key
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyCtrlC {
			select {
			case <-quit:
				// Already closed
			default:
				close(quit)
			}
			app.Stop()
			return nil
		}
		return event
	})

	// Show initial status (set directly, not via QueueUpdateDraw since app isn't running yet)
	setStatusText(fmt.Sprintf("Connecting to %s...", config.Address), false)

	// Signal when app is running (after first draw)
	var appStartOnce sync.Once
	app.SetAfterDrawFunc(func(screen tcell.Screen) {
		appStartOnce.Do(func() {
			close(appRunning)
		})
	})

	// Run the connection UI
	app.SetRoot(flex, true)
	if err := app.Run(); err != nil {
		return nil, fmt.Errorf("UI error: %w", err)
	}

	// Wait for connection goroutine to finish
	<-done

	mu.Lock()
	defer mu.Unlock()

	if connErr != nil {
		return nil, connErr
	}

	return provider, nil
}
