package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/theme/themes"
	"github.com/atterpac/jig/util"
	"github.com/galaxy-io/tempo/internal/commands/isbroken"
	"github.com/galaxy-io/tempo/internal/config"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/galaxy-io/tempo/internal/update"
	"github.com/galaxy-io/tempo/internal/view"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CLI flags
var (
	profileName   = flag.String("profile", "", "Connection profile name (from config)")
	address       = flag.String("address", "", "Temporal server address (overrides profile)")
	namespace     = flag.String("namespace", "", "Default namespace (overrides profile)")
	tlsCert       = flag.String("tls-cert", "", "Path to TLS certificate (overrides profile)")
	tlsKey        = flag.String("tls-key", "", "Path to TLS private key (overrides profile)")
	tlsCA         = flag.String("tls-ca", "", "Path to CA certificate (overrides profile)")
	tlsServerName = flag.String("tls-server-name", "", "Server name for TLS verification (overrides profile)")
	tlsSkipVerify = flag.Bool("tls-skip-verify", false, "Skip TLS verification (insecure)")
	themeNameFlag = flag.String("theme", "", "Theme name (overrides config file)")
	devMode       = flag.Bool("dev", false, "Development mode: test splash screen with theme cycling")
	versionFlag   = flag.Bool("version", false, "Print version information and exit")
)

const (
	maxRetries     = 5
	initialBackoff = 1 * time.Second
	maxBackoff     = 10 * time.Second
)

func main() {
	// Check for subcommands before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "isbroken" {
		isbroken.Run(os.Args[2:])
		return
	}

	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Println(update.GetVersionInfo())
		os.Exit(0)
	}

	// Load configuration from file
	cfg, err := config.Load()
	if err != nil {
		// Config load error is non-fatal, use defaults
		cfg = config.DefaultConfig()
	}

	// Determine theme: CLI flag overrides config file
	themeName := cfg.Theme
	if *themeNameFlag != "" {
		themeName = *themeNameFlag
	}

	// Initialize theme system before any UI using jig's built-in themes
	selectedTheme := themes.Get(themeName)
	if selectedTheme == nil {
		fmt.Fprintf(os.Stderr, "Warning: theme %q not found, using tokyonight-night\n", themeName)
		selectedTheme = themes.Default()
	}
	theme.SetProvider(selectedTheme)

	// Register Temporal-specific statuses with jig's theme system
	temporal.RegisterTemporalStatuses()

	// Determine which profile to use
	activeProfileName := cfg.ActiveProfile
	if *profileName != "" {
		// CLI flag overrides active profile
		if !cfg.ProfileExists(*profileName) {
			fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", *profileName)
			fmt.Fprintf(os.Stderr, "Available profiles: %v\n", cfg.ListProfiles())
			os.Exit(1)
		}
		activeProfileName = *profileName
		cfg.ActiveProfile = activeProfileName
	}

	// Get the profile's connection config
	profileConfig, _ := cfg.GetProfile(activeProfileName)

	// Build temporal connection config from profile
	connConfig := temporal.ConnectionConfig{
		Address:       profileConfig.Address,
		Namespace:     profileConfig.Namespace,
		TLSCertPath:   profileConfig.TLS.Cert,
		TLSKeyPath:    profileConfig.TLS.Key,
		TLSCAPath:     profileConfig.TLS.CA,
		TLSServerName: profileConfig.TLS.ServerName,
		TLSSkipVerify: profileConfig.TLS.SkipVerify,
	}

	// CLI flags override profile settings
	if *address != "" {
		connConfig.Address = *address
	}
	if *namespace != "" {
		connConfig.Namespace = *namespace
	}
	if *tlsCert != "" {
		connConfig.TLSCertPath = *tlsCert
	}
	if *tlsKey != "" {
		connConfig.TLSKeyPath = *tlsKey
	}
	if *tlsCA != "" {
		connConfig.TLSCAPath = *tlsCA
	}
	if *tlsServerName != "" {
		connConfig.TLSServerName = *tlsServerName
	}
	if *tlsSkipVerify {
		connConfig.TLSSkipVerify = true
	}

	// Run connection with UI
	provider, err := connectWithUI(connConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer provider.Close()

	// Launch main application with config for profile management
	app := view.NewAppWithProvider(provider, connConfig.Namespace, cfg, activeProfileName)
	app.SetDevMode(*devMode)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

const splashLogo = `
░▒▓████████▓▒░▒▓████████▓▒░▒▓██████████████▓▒░░▒▓███████▓▒░ ░▒▓██████▓▒░  
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓█▓▒░   ░▒▓██████▓▒░ ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓███████▓▒░░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░ 
   ░▒▓█▓▒░   ░▒▓████████▓▒░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░       ░▒▓██████▓▒░  
`

// const splashLogo = `
//                _                       __
//               /   \                  /      \
//              '      \              /          \
//             |       |Oo          o|            |
//             \    \  |OOOo......oOO|   /        |
//              \    \\OOOOOOOOOOOOOOO\//        /
//                \ _o\OOOOOOOOOOOOOOOO//. ___ /
//            ______OOOOOOOOOOOOOOOOOOOOOOOo.___
//             --- OO'* *OOOOOOOOOO'*   OOOOO--
//                 OO.   OOOOOOOOO'    .OOOOO o
//                 \OOOooOOOOOOOOOooooOOOOOO'OOOo
//               .OO "OOOOOOOOOOOOOOOOOOOO"OOOOOOOo
//           __ OOOOOOOOOOOOOOOOOOOOOO"OOOOOOOOOOOOo
//          ___OOOOOOOO_"OOOOOOOOOOO"_OOOOOOOOOOOOOOOO
//            OOOOO^OOOO0-(____)/OOOOOOOOOOOOO^OOOOOO
//            OOOOO OO000/00||00\000000OOOOOOOO OOOOOO
//            OOOOO O0000000000000000 ppppoooooOOOOOO
//             OOOOO 0000000000000000 QQQQ "OOOOOOO"
//             o"OOOO 000000000000000oooooOOoooooooO'
//             OOo"OOOO.00000000000000000000OOOOOOOO'
//            OOOOOO QQQQ 0000000000000000000OOOOOOO
//           OOOOOO00eeee00000000000000000000OOOOOOOO.
//          OOOOOOOO000000000000000000000000OOOOOOOOOO
//          OOOOOOOOO00000000000000000000000OOOOOOOOOO
//           OOOOOOOOO000000000000000000000OOOOOOOOOOO
//            "OOOOOOOO0000000000000000000OOOOOOOOOOO'
//              "OOOOOOO00000000000000000OOOOOOOOOO"
//   .ooooOOOOOOOo"OOOOOOO000000000000OOOOOOOOOOO"
// .OOO"""""""""".oOOOOOOOOOOOOOOOOOOOOOOOOOOOOo
// OOO         QQQQO"'                      "QQQQ
// OOO
//  OOo.
//   "OOOOOOOOOOOOoooooooo....
// `

// connectWithUI shows a connection UI while attempting to connect to Temporal.
// Returns the provider on success, or error if user quits or max retries exceeded.
func connectWithUI(config temporal.ConnectionConfig) (temporal.Provider, error) {
	app := tview.NewApplication()

	// Note: Global tview.Styles are already set by theme.SetProvider() in main()

	// Logo display - use left alignment to preserve internal spacing
	logoText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	logoText.SetBackgroundColor(theme.Bg())

	// Apply gradient effect to logo using theme colors
	gradientColors := util.DefaultGradientColors()
	gradientLogo := util.ApplyDiagonalGradient(splashLogo, gradientColors)
	logoText.SetText(gradientLogo)

	// Create spacer boxes with background color
	leftSpacer := tview.NewBox().SetBackgroundColor(theme.Bg())
	rightSpacer := tview.NewBox().SetBackgroundColor(theme.Bg())
	topSpacer := tview.NewBox().SetBackgroundColor(theme.Bg())
	midSpacer := tview.NewBox().SetBackgroundColor(theme.Bg())
	bottomSpacer := tview.NewBox().SetBackgroundColor(theme.Bg())

	// Wrap logo in horizontal flex to center it as a block
	logoContainer := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftSpacer, 0, 1, false).
		AddItem(logoText, 78, 0, false).
		AddItem(rightSpacer, 0, 1, false)
	logoContainer.SetBackgroundColor(theme.Bg())

	// Status display
	statusText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	statusText.SetBackgroundColor(theme.Bg())

	// Sponsor display (centered, subtle)
	sponsorText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	sponsorText.SetBackgroundColor(theme.Bg())
	sponsorText.SetText(fmt.Sprintf(
		"[%s]Made with %s  by getgalaxy.io[-]",
		theme.TagFgDim(), theme.IconHeart,
	))

	// Build layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topSpacer, 0, 1, false).
		AddItem(logoContainer, 9, 0, false).
		AddItem(statusText, 3, 0, false).
		AddItem(midSpacer, 1, 0, false).
		AddItem(sponsorText, 1, 0, false).
		AddItem(bottomSpacer, 0, 1, false)
	flex.SetBackgroundColor(theme.Bg())

	// Result channels and sync
	var provider temporal.Provider
	var connErr error
	var mu sync.Mutex
	quit := make(chan struct{})
	done := make(chan struct{})
	appRunning := make(chan struct{})

	// setStatusText sets the status text content
	setStatusText := func(msg string, isError bool) {
		color := theme.TagAccent()
		if isError {
			color = theme.TagError()
		}
		statusText.SetText(fmt.Sprintf(
			"[%s]%s[-]\n[%s]Press 'q' to quit[-]",
			color, msg,
			theme.TagFgDim(),
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

		// Show splash screen for a moment before connecting
		select {
		case <-quit:
			mu.Lock()
			connErr = fmt.Errorf("connection cancelled by user")
			mu.Unlock()
			return
		case <-time.After(1500 * time.Millisecond):
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
				time.Sleep(1 * time.Second) // Pause to show success
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
	setStatusText("Initializing...", false)

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
