package isbroken

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/theme/themes"
	"github.com/galaxy-io/tempo/internal/config"
	"github.com/galaxy-io/tempo/internal/update"
	"github.com/galaxy-io/tempo/internal/view"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// DebugInfo holds all collected environment information.
type DebugInfo struct {
	// Version
	Version   string
	Commit    string
	BuildDate string

	// System
	OS        string
	Arch      string
	GoVersion string

	// Terminal
	TerminalCols int
	TerminalRows int
	Term         string
	ColorTerm    string
	TermProgram  string
	ColorSpace   string

	// Config
	ConfigPath string
	ThemeName  string

	// Profile
	ProfileName   string
	ServerAddress string
	Namespace     string
	TLSEnabled    bool
	TLSCertPath   string
	TLSKeyPath    string
	TLSCAPath     string
}

// Run handles the 'isbroken' subcommand.
func Run(args []string) {
	fs := flag.NewFlagSet("isbroken", flag.ExitOnError)
	plain := fs.Bool("plain", false, "Output plain text instead of TUI")
	themeFlag := fs.String("theme", "", "Theme name (for TUI mode)")
	fs.Parse(args)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Collect debug info
	info := collectDebugInfo(cfg)

	if *plain {
		printPlainDebugInfo(info)
		return
	}

	// TUI mode
	themeName := cfg.Theme
	if *themeFlag != "" {
		themeName = *themeFlag
	}

	selectedTheme := themes.Get(themeName)
	if selectedTheme == nil {
		selectedTheme = themes.Default()
		themeName = "tokyonight-night"
	}
	theme.SetProvider(selectedTheme)
	info.ThemeName = themeName

	runTUI(info, cfg)
}

// collectDebugInfo gathers all environment information.
func collectDebugInfo(cfg *config.Config) DebugInfo {
	info := DebugInfo{
		// Version info
		Version:   update.Version,
		Commit:    update.Commit,
		BuildDate: update.BuildDate,

		// System info
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),

		// Terminal environment
		Term:        os.Getenv("TERM"),
		ColorTerm:   os.Getenv("COLORTERM"),
		TermProgram: os.Getenv("TERM_PROGRAM"),

		// Config
		ConfigPath: config.ConfigPath(),
		ThemeName:  cfg.Theme,

		// Profile (if exists)
		ProfileName: cfg.ActiveProfile,
	}

	// Get terminal size - try multiple methods
	// Method 1: golang.org/x/term (most reliable for non-TUI contexts)
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		info.TerminalCols = w
		info.TerminalRows = h
	}

	// Method 2: Environment variables (fallback)
	if info.TerminalCols == 0 {
		if cols := os.Getenv("COLUMNS"); cols != "" {
			if c, err := strconv.Atoi(cols); err == nil {
				info.TerminalCols = c
			}
		}
	}
	if info.TerminalRows == 0 {
		if rows := os.Getenv("LINES"); rows != "" {
			if r, err := strconv.Atoi(rows); err == nil {
				info.TerminalRows = r
			}
		}
	}

	// Detect color space from environment and tcell
	info.ColorSpace = detectColorSpaceFromEnv(info.Term, info.ColorTerm)

	// Try to get more accurate color info from tcell if possible
	screen, err := tcell.NewScreen()
	if err == nil {
		if err := screen.Init(); err == nil {
			tcellColorSpace := detectColorSpace(screen)
			if tcellColorSpace != "" {
				info.ColorSpace = tcellColorSpace
			}
			// Also try to get size from tcell as another source
			if info.TerminalCols == 0 || info.TerminalRows == 0 {
				info.TerminalCols, info.TerminalRows = screen.Size()
			}
			screen.Fini()
		}
	}

	// Get profile connection details
	if profile, ok := cfg.GetProfile(cfg.ActiveProfile); ok {
		info.ServerAddress = profile.Address
		info.Namespace = profile.Namespace
		info.TLSEnabled = profile.TLS.Cert != "" || profile.TLS.CA != ""
		info.TLSCertPath = profile.TLS.Cert
		info.TLSKeyPath = profile.TLS.Key
		info.TLSCAPath = profile.TLS.CA
	}

	return info
}

// detectColorSpace determines the terminal's color capabilities from tcell.
func detectColorSpace(screen tcell.Screen) string {
	colors := screen.Colors()
	switch {
	case colors >= 16777216:
		return "truecolor (16M colors)"
	case colors >= 256:
		return "256 colors"
	case colors >= 88:
		return "88 colors"
	case colors >= 16:
		return "16 colors"
	case colors >= 8:
		return "8 colors"
	case colors > 0:
		return fmt.Sprintf("%d colors", colors)
	default:
		return ""
	}
}

// detectColorSpaceFromEnv determines color capabilities from environment variables.
func detectColorSpaceFromEnv(termEnv, colorTerm string) string {
	// Check COLORTERM first (most explicit indicator)
	switch colorTerm {
	case "truecolor", "24bit":
		return "truecolor (24-bit)"
	}

	// Check TERM for color hints
	if strings.Contains(termEnv, "256color") {
		return "256 colors"
	}
	if strings.Contains(termEnv, "truecolor") || strings.Contains(termEnv, "24bit") {
		return "truecolor (24-bit)"
	}
	if strings.Contains(termEnv, "color") || strings.Contains(termEnv, "xterm") {
		return "16+ colors (from TERM)"
	}

	return "unknown"
}

// printPlainDebugInfo outputs debug info as plain text.
func printPlainDebugInfo(info DebugInfo) {
	var sb strings.Builder

	sb.WriteString("=== Tempo Debug Info ===\n\n")

	sb.WriteString("VERSION\n")
	sb.WriteString(fmt.Sprintf("  tempo: %s\n", info.Version))
	sb.WriteString(fmt.Sprintf("  commit: %s\n", info.Commit))
	sb.WriteString(fmt.Sprintf("  built: %s\n", info.BuildDate))
	sb.WriteString("\n")

	sb.WriteString("SYSTEM\n")
	sb.WriteString(fmt.Sprintf("  os: %s\n", info.OS))
	sb.WriteString(fmt.Sprintf("  arch: %s\n", info.Arch))
	sb.WriteString(fmt.Sprintf("  go: %s\n", info.GoVersion))
	sb.WriteString("\n")

	sb.WriteString("TERMINAL\n")
	sb.WriteString(fmt.Sprintf("  size: %dx%d\n", info.TerminalCols, info.TerminalRows))
	sb.WriteString(fmt.Sprintf("  term: %s\n", valueOrNA(info.Term)))
	sb.WriteString(fmt.Sprintf("  colorterm: %s\n", valueOrNA(info.ColorTerm)))
	sb.WriteString(fmt.Sprintf("  term_program: %s\n", valueOrNA(info.TermProgram)))
	sb.WriteString(fmt.Sprintf("  color_space: %s\n", valueOrNA(info.ColorSpace)))
	sb.WriteString("\n")

	sb.WriteString("CONFIG\n")
	sb.WriteString(fmt.Sprintf("  path: %s\n", info.ConfigPath))
	sb.WriteString(fmt.Sprintf("  theme: %s\n", info.ThemeName))
	sb.WriteString("\n")

	sb.WriteString("PROFILE\n")
	sb.WriteString(fmt.Sprintf("  active: %s\n", valueOrNA(info.ProfileName)))
	sb.WriteString(fmt.Sprintf("  address: %s\n", valueOrNA(info.ServerAddress)))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", valueOrNA(info.Namespace)))
	if info.TLSEnabled {
		sb.WriteString("  tls: enabled\n")
		if info.TLSCertPath != "" {
			sb.WriteString(fmt.Sprintf("  tls_cert: %s\n", info.TLSCertPath))
		}
		if info.TLSKeyPath != "" {
			sb.WriteString(fmt.Sprintf("  tls_key: %s\n", info.TLSKeyPath))
		}
		if info.TLSCAPath != "" {
			sb.WriteString(fmt.Sprintf("  tls_ca: %s\n", info.TLSCAPath))
		}
	} else {
		sb.WriteString("  tls: disabled\n")
	}

	fmt.Print(sb.String())
}

// valueOrNA returns the value or "n/a" if empty.
func valueOrNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

// runTUI launches the debug TUI.
func runTUI(info DebugInfo, cfg *config.Config) {
	app := view.NewDebugApp(info.ToDebugData(), cfg)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ToDebugData converts DebugInfo to the view package's DebugData type.
func (d DebugInfo) ToDebugData() view.DebugData {
	return view.DebugData{
		Version:       d.Version,
		Commit:        d.Commit,
		BuildDate:     d.BuildDate,
		OS:            d.OS,
		Arch:          d.Arch,
		GoVersion:     d.GoVersion,
		TerminalCols:  d.TerminalCols,
		TerminalRows:  d.TerminalRows,
		Term:          d.Term,
		ColorTerm:     d.ColorTerm,
		TermProgram:   d.TermProgram,
		ColorSpace:    d.ColorSpace,
		ConfigPath:    d.ConfigPath,
		ThemeName:     d.ThemeName,
		ProfileName:   d.ProfileName,
		ServerAddress: d.ServerAddress,
		Namespace:     d.Namespace,
		TLSEnabled:    d.TLSEnabled,
		TLSCertPath:   d.TLSCertPath,
		TLSKeyPath:    d.TLSKeyPath,
		TLSCAPath:     d.TLSCAPath,
	}
}
