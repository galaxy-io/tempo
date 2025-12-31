package view

import (
	"fmt"
	"strings"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/layout"
	"github.com/atterpac/jig/nav"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/theme/themes"
	"github.com/galaxy-io/tempo/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const pons = `
               _                       __
              /   \                  /      \
             '      \              /          \
            |       |Oo          o|            |
            \    \  |OOOo......oOO|   /        |
             \    \\OOOOOOOOOOOOOOO\//        /
               \ _o\OOOOOOOOOOOOOOOO//. ___ /
           ______OOOOOOOOOOOOOOOOOOOOOOOo.___
            --- OO'* *OOOOOOOOOO'*   OOOOO--
                OO.   OOOOOOOOO'    .OOOOO o
                \OOOooOOOOOOOOOooooOOOOOO'OOOo
              .OO "OOOOOOOOOOOOOOOOOOOO"OOOOOOOo
          __ OOOOOOOOOOOOOOOOOOOOOO"OOOOOOOOOOOOo
         ___OOOOOOOO_"OOOOOOOOOOO"_OOOOOOOOOOOOOOOO
           OOOOO^OOOO0-(____)/OOOOOOOOOOOOO^OOOOOO
           OOOOO OO000/00||00\000000OOOOOOOO OOOOOO
           OOOOO O0000000000000000 ppppoooooOOOOOO
            OOOOO 0000000000000000 QQQQ "OOOOOOO"
            o"OOOO 000000000000000oooooOOoooooooO'
            OOo"OOOO.00000000000000000000OOOOOOOO'
           OOOOOO QQQQ 0000000000000000000OOOOOOO
          OOOOOO00eeee00000000000000000000OOOOOOOO.
         OOOOOOOO000000000000000000000000OOOOOOOOOO
         OOOOOOOOO00000000000000000000000OOOOOOOOOO
          OOOOOOOOO000000000000000000000OOOOOOOOOOO
           "OOOOOOOO0000000000000000000OOOOOOOOOOO'
             "OOOOOOO00000000000000000OOOOOOOOOO"
  .ooooOOOOOOOo"OOOOOOO000000000000OOOOOOOOOOO"
.OOO"""""""""".oOOOOOOOOOOOOOOOOOOOOOOOOOOOOo
OOO         QQQQO"'                      "QQQQ
OOO
 OOo.
  "OOOOOOOOOOOOoooooooo....
`

// DebugData holds debug information passed from the main package.
type DebugData struct {
	Version       string
	Commit        string
	BuildDate     string
	OS            string
	Arch          string
	GoVersion     string
	TerminalCols  int
	TerminalRows  int
	Term          string
	ColorTerm     string
	TermProgram   string
	ColorSpace    string
	ConfigPath    string
	ThemeName     string
	ProfileName   string
	ServerAddress string
	Namespace     string
	TLSEnabled    bool
	TLSCertPath   string
	TLSKeyPath    string
	TLSCAPath     string
}

// DebugScreen displays environment and debug information.
type DebugScreen struct {
	*tview.Flex
	panel   *components.Panel
	content *tview.TextView
	pons     *tview.TextView
	inner   *tview.Flex
	data    DebugData
	app     *DebugApp // reference to app for toasts
}

// NewDebugScreen creates a new debug screen view.
func NewDebugScreen(data DebugData) *DebugScreen {
	ds := &DebugScreen{
		Flex:    tview.NewFlex().SetDirection(tview.FlexColumn),
		content: tview.NewTextView(),
		pons:     tview.NewTextView(),
		inner:   tview.NewFlex().SetDirection(tview.FlexColumn),
		data:    data,
	}
	ds.setup()
	return ds
}

func (ds *DebugScreen) setup() {
	ds.SetBackgroundColor(theme.Bg())

	// Configure content view (left side - debug info)
	ds.content.SetDynamicColors(true)
	ds.content.SetBackgroundColor(theme.Bg())
	ds.content.SetTextColor(theme.Fg())
	ds.content.SetWordWrap(true)
	ds.content.SetScrollable(true)

	// Configure rat view (right side)
	ds.pons.SetDynamicColors(true)
	ds.pons.SetBackgroundColor(theme.Bg())
	ds.pons.SetTextColor(theme.Fg())
	ds.pons.SetTextAlign(tview.AlignLeft)

	// Inner flex: debug info on left, rat on right
	ds.inner.SetBackgroundColor(theme.Bg())
	ds.inner.AddItem(ds.content, 0, 1, true)
	ds.inner.AddItem(ds.pons, 55, 0, false) // rat is ~50 chars wide

	// Create panel
	ds.panel = components.NewPanel().SetTitle(fmt.Sprintf("%s Debug Info", theme.IconInfo))
	ds.panel.SetContent(ds.inner)

	// Single panel layout
	ds.AddItem(ds.panel, 0, 1, true)

	// Render content
	ds.renderContent()
	ds.renderRat()
}

func (ds *DebugScreen) renderContent() {
	dim := theme.TagFgDim()
	fg := theme.TagFg()
	accent := theme.TagAccent()
	success := theme.TagSuccess()
	warn := theme.TagWarning()

	var text string

	// Version section
	text += fmt.Sprintf("[%s::b]VERSION[-:-:-]\n", accent)
	text += fmt.Sprintf("  [%s]tempo:[-]   [%s]%s[-]\n", dim, fg, ds.data.Version)
	text += fmt.Sprintf("  [%s]commit:[-]  [%s]%s[-]\n", dim, fg, ds.data.Commit)
	text += fmt.Sprintf("  [%s]built:[-]   [%s]%s[-]\n", dim, fg, ds.data.BuildDate)
	text += "\n"

	// System section
	text += fmt.Sprintf("[%s::b]SYSTEM[-:-:-]\n", accent)
	text += fmt.Sprintf("  [%s]os:[-]      [%s]%s[-]\n", dim, fg, ds.data.OS)
	text += fmt.Sprintf("  [%s]arch:[-]    [%s]%s[-]\n", dim, fg, ds.data.Arch)
	text += fmt.Sprintf("  [%s]go:[-]      [%s]%s[-]\n", dim, fg, ds.data.GoVersion)
	text += "\n"

	// Terminal section
	text += fmt.Sprintf("[%s::b]TERMINAL[-:-:-]\n", accent)
	text += fmt.Sprintf("  [%s]size:[-]        [%s]%dx%d[-]\n", dim, fg, ds.data.TerminalCols, ds.data.TerminalRows)
	text += fmt.Sprintf("  [%s]term:[-]        [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.Term))
	text += fmt.Sprintf("  [%s]colorterm:[-]   [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.ColorTerm))
	text += fmt.Sprintf("  [%s]term_program:[-][%s] %s[-]\n", dim, fg, valueOrDash(ds.data.TermProgram))
	text += fmt.Sprintf("  [%s]color_space:[-] [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.ColorSpace))
	text += "\n"

	// Config section
	text += fmt.Sprintf("[%s::b]CONFIG[-:-:-]\n", accent)
	text += fmt.Sprintf("  [%s]path:[-]   [%s]%s[-]\n", dim, fg, ds.data.ConfigPath)
	text += fmt.Sprintf("  [%s]theme:[-]  [%s]%s[-]\n", dim, fg, ds.data.ThemeName)
	text += "\n"

	// Profile section
	text += fmt.Sprintf("[%s::b]PROFILE[-:-:-]\n", accent)
	text += fmt.Sprintf("  [%s]active:[-]    [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.ProfileName))
	text += fmt.Sprintf("  [%s]address:[-]   [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.ServerAddress))
	text += fmt.Sprintf("  [%s]namespace:[-] [%s]%s[-]\n", dim, fg, valueOrDash(ds.data.Namespace))

	if ds.data.TLSEnabled {
		text += fmt.Sprintf("  [%s]tls:[-]       [%s]enabled[-]\n", dim, success)
		if ds.data.TLSCertPath != "" {
			text += fmt.Sprintf("  [%s]tls_cert:[-]  [%s]%s[-]\n", dim, fg, ds.data.TLSCertPath)
		}
		if ds.data.TLSKeyPath != "" {
			text += fmt.Sprintf("  [%s]tls_key:[-]   [%s]%s[-]\n", dim, fg, ds.data.TLSKeyPath)
		}
		if ds.data.TLSCAPath != "" {
			text += fmt.Sprintf("  [%s]tls_ca:[-]    [%s]%s[-]\n", dim, fg, ds.data.TLSCAPath)
		}
	} else {
		text += fmt.Sprintf("  [%s]tls:[-]       [%s]disabled[-]\n", dim, warn)
	}

	ds.content.SetText(text)
}

func (ds *DebugScreen) renderRat() {
	dim := theme.TagFgDim()
	accent := theme.TagAccent()

	var text string
	text += fmt.Sprintf("[%s]%s[-]\n", dim, pons)
	text += fmt.Sprintf("[%s::b]I'm sorry 󰋔[-:-:-]\n", accent)
	text += fmt.Sprintf("[%s]This information helps me debug and solve issues faster[-]\n\n", dim)
	text += fmt.Sprintf("[%s]y[-] yank report\n", accent)
	text += fmt.Sprintf("[%s]Y[-] yank issue template\n", accent)

	ds.pons.SetText(text)
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// RefreshTheme updates all component colors after a theme change.
func (ds *DebugScreen) RefreshTheme() {
	bg := theme.Bg()
	ds.SetBackgroundColor(bg)
	ds.inner.SetBackgroundColor(bg)
	ds.content.SetBackgroundColor(bg)
	ds.content.SetTextColor(theme.Fg())
	ds.pons.SetBackgroundColor(bg)
	ds.pons.SetTextColor(theme.Fg())
	ds.renderContent()
	ds.renderRat()
}

// GeneratePlainReport generates a plain text debug report.
func (ds *DebugScreen) GeneratePlainReport() string {
	var sb strings.Builder

	sb.WriteString("=== Tempo Debug Info ===\n\n")

	sb.WriteString("VERSION\n")
	sb.WriteString(fmt.Sprintf("  tempo: %s\n", ds.data.Version))
	sb.WriteString(fmt.Sprintf("  commit: %s\n", ds.data.Commit))
	sb.WriteString(fmt.Sprintf("  built: %s\n", ds.data.BuildDate))
	sb.WriteString("\n")

	sb.WriteString("SYSTEM\n")
	sb.WriteString(fmt.Sprintf("  os: %s\n", ds.data.OS))
	sb.WriteString(fmt.Sprintf("  arch: %s\n", ds.data.Arch))
	sb.WriteString(fmt.Sprintf("  go: %s\n", ds.data.GoVersion))
	sb.WriteString("\n")

	sb.WriteString("TERMINAL\n")
	sb.WriteString(fmt.Sprintf("  size: %dx%d\n", ds.data.TerminalCols, ds.data.TerminalRows))
	sb.WriteString(fmt.Sprintf("  term: %s\n", valueOrDash(ds.data.Term)))
	sb.WriteString(fmt.Sprintf("  colorterm: %s\n", valueOrDash(ds.data.ColorTerm)))
	sb.WriteString(fmt.Sprintf("  term_program: %s\n", valueOrDash(ds.data.TermProgram)))
	sb.WriteString(fmt.Sprintf("  color_space: %s\n", valueOrDash(ds.data.ColorSpace)))
	sb.WriteString("\n")

	sb.WriteString("CONFIG\n")
	sb.WriteString(fmt.Sprintf("  path: %s\n", ds.data.ConfigPath))
	sb.WriteString(fmt.Sprintf("  theme: %s\n", ds.data.ThemeName))
	sb.WriteString("\n")

	sb.WriteString("PROFILE\n")
	sb.WriteString(fmt.Sprintf("  active: %s\n", valueOrDash(ds.data.ProfileName)))
	sb.WriteString(fmt.Sprintf("  address: %s\n", valueOrDash(ds.data.ServerAddress)))
	sb.WriteString(fmt.Sprintf("  namespace: %s\n", valueOrDash(ds.data.Namespace)))
	if ds.data.TLSEnabled {
		sb.WriteString("  tls: enabled\n")
	} else {
		sb.WriteString("  tls: disabled\n")
	}

	return sb.String()
}

// GenerateIssueTemplate generates a GitHub issue template with debug info.
func (ds *DebugScreen) GenerateIssueTemplate() string {
	var sb strings.Builder

	sb.WriteString("## Bug Report\n\n")
	sb.WriteString("### Description\n")
	sb.WriteString("<!-- Describe what happened -->\n\n")
	sb.WriteString("### Expected Behavior\n")
	sb.WriteString("<!-- What did you expect to happen? -->\n\n")
	sb.WriteString("### Steps to Reproduce\n")
	sb.WriteString("1. \n2. \n3. \n\n")
	sb.WriteString("### Environment\n")
	sb.WriteString("```\n")
	sb.WriteString(ds.GeneratePlainReport())
	sb.WriteString("```\n")

	return sb.String()
}

// Name returns the view name.
func (ds *DebugScreen) Name() string {
	return "debug"
}

// Start is called when the view becomes active.
func (ds *DebugScreen) Start() {}

// Stop is called when the view is deactivated.
func (ds *DebugScreen) Stop() {}

// Hints returns keybinding hints for this view.
func (ds *DebugScreen) Hints() []KeyHint {
	return []KeyHint{
		{Key: "y", Description: "Yank report"},
		{Key: "Y", Description: "Yank issue"},
		{Key: "T", Description: "Theme"},
		{Key: "q", Description: "Quit"},
	}
}

// DebugApp is a simplified application for the debug screen.
type DebugApp struct {
	app       *layout.App
	statusBar *layout.StatusBar
	menu      *layout.Menu
	toasts    *components.ToastManager
	screen    *DebugScreen
	config    *config.Config
}

// NewDebugApp creates a new debug application.
func NewDebugApp(data DebugData, cfg *config.Config) *DebugApp {
	da := &DebugApp{
		config: cfg,
	}

	// Create status bar
	da.statusBar = layout.NewStatusBar()
	da.statusBar.SetTitle("tempo isbroken")
	da.statusBar.SetTitleAlign(components.AlignLeft)

	// Create menu
	da.menu = layout.NewMenu()

	// Create app with jig layout
	da.app = layout.NewApp(layout.AppConfig{
		TopBar:       da.statusBar,
		TopBarHeight: 3,
		ShowCrumbs:   false,
		BottomBar:    da.menu,
		OnComponentChange: func(c nav.Component) {
			if c != nil {
				da.menu.SetHints(c.Hints())
			}
		},
	})

	// Create toast manager for notifications
	da.toasts = components.NewToastManager(da.app.GetApplication())
	da.toasts.SetPosition(components.ToastBottomRight)

	// Wire up toast rendering as an overlay
	da.app.GetApplication().SetAfterDrawFunc(func(screen tcell.Screen) {
		w, h := screen.Size()
		da.toasts.Draw(screen, w, h)
	})

	// Create debug screen
	da.screen = NewDebugScreen(data)
	da.screen.app = da // give screen reference to app for toasts
	da.app.Pages().Push(da.screen)
	da.menu.SetHints(da.screen.Hints())

	// Global key handler
	da.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			da.app.Stop()
			return nil
		case 'T':
			da.showThemeSelector()
			return nil
		case 'y':
			da.yankReport()
			return nil
		case 'Y':
			da.yankIssueTemplate()
			return nil
		}

		// Escape also quits
		if event.Key() == tcell.KeyEscape {
			da.app.Stop()
			return nil
		}

		return event
	})

	return da
}

// yankReport copies the debug report to clipboard.
func (da *DebugApp) yankReport() {
	report := da.screen.GeneratePlainReport()
	if err := copyToClipboard(report); err != nil {
		da.toasts.Error("Failed to copy: " + err.Error())
	} else {
		da.toasts.Success("Report copied to clipboard!")
	}
}

// yankIssueTemplate copies the issue template to clipboard.
func (da *DebugApp) yankIssueTemplate() {
	template := da.screen.GenerateIssueTemplate()
	if err := copyToClipboard(template); err != nil {
		da.toasts.Error("Failed to copy: " + err.Error())
	} else {
		da.toasts.Success("Issue template copied to clipboard!")
	}
}

// Run starts the debug application.
func (da *DebugApp) Run() error {
	return da.app.Run()
}

// showThemeSelector displays the theme picker modal.
func (da *DebugApp) showThemeSelector() {
	bg := theme.Bg()
	list := tview.NewList()
	list.SetBackgroundColor(bg)
	list.SetMainTextColor(theme.Fg())
	list.SetMainTextStyle(tcell.StyleDefault.Background(bg).Foreground(theme.Fg()))
	list.SetSelectedBackgroundColor(theme.Accent())
	list.SetSelectedTextColor(bg)
	list.SetSelectedStyle(tcell.StyleDefault.Background(theme.Accent()).Foreground(bg))
	list.SetHighlightFullLine(true)
	list.ShowSecondaryText(false)

	// Get all available themes
	allThemes := themes.All()
	currentTheme := da.config.Theme

	// Categorize themes
	var darkThemes, lightThemes []string
	for name := range allThemes {
		if debugIsDarkTheme(name) {
			darkThemes = append(darkThemes, name)
		} else {
			lightThemes = append(lightThemes, name)
		}
	}

	// Sort themes alphabetically
	debugSortStrings(darkThemes)
	debugSortStrings(lightThemes)

	// Track mapping from list index to theme name
	listToTheme := make(map[int]string)
	listIdx := 0

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
				theme.SetProvider(newTheme)
				da.screen.data.ThemeName = name
				da.screen.RefreshTheme()
			}
			// Save theme to config
			da.config.Theme = name
			da.config.Save()
			da.app.Pages().RemovePage("theme-selector")
			da.app.SetFocus(da.screen)
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
				theme.SetProvider(newTheme)
				da.screen.data.ThemeName = name
				da.screen.RefreshTheme()
			}
			// Save theme to config
			da.config.Theme = name
			da.config.Save()
			da.app.Pages().RemovePage("theme-selector")
			da.app.SetFocus(da.screen)
		})
		listIdx++
	}

	// Find list index for current theme
	currentListIdx := 1 // Default to first dark theme
	for idx, name := range listToTheme {
		if name == currentTheme {
			currentListIdx = idx
			break
		}
	}
	list.SetCurrentItem(currentListIdx)

	// Live preview on change
	list.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if themeName, ok := listToTheme[index]; ok {
			if t := themes.Get(themeName); t != nil {
				theme.SetProvider(t)
				da.screen.data.ThemeName = themeName
				da.screen.RefreshTheme()
			}
		}
	})

	// Vim-style navigation and escape handling
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}

		if event.Key() == tcell.KeyEscape {
			// Restore original theme
			if t := themes.Get(currentTheme); t != nil {
				theme.SetProvider(t)
				da.screen.data.ThemeName = currentTheme
				da.screen.RefreshTheme()
			}
			da.app.Pages().RemovePage("theme-selector")
			da.app.SetFocus(da.screen)
			return nil
		}

		// Skip headers when navigating
		if event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp {
			current := list.GetCurrentItem()
			// If on header, skip to next valid item
			if current == 0 {
				list.SetCurrentItem(1)
				return nil
			}
			if current == lightHeaderIdx {
				if event.Key() == tcell.KeyDown {
					list.SetCurrentItem(lightHeaderIdx + 1)
				} else {
					list.SetCurrentItem(lightHeaderIdx - 1)
				}
				return nil
			}
		}

		return event
	})

	// Create modal
	modal := components.NewModal(components.ModalConfig{
		Title:    "Select Theme",
		Width:    30,
		Height:   22,
		Backdrop: false,
	})
	modal.SetContent(list)
	modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Navigate"},
		{Key: "Enter", Description: "Select"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		// Restore original theme
		if t := themes.Get(currentTheme); t != nil {
			theme.SetProvider(t)
			da.screen.data.ThemeName = currentTheme
			da.screen.RefreshTheme()
		}
		da.app.Pages().RemovePage("theme-selector")
		da.app.SetFocus(da.screen)
	})

	da.app.Pages().AddPage("theme-selector", modal, true, true)
	da.app.SetFocus(list)

	// Suppress unused variable warning
	_ = lightHeaderIdx
}

// debugIsDarkTheme checks if a theme name indicates a dark theme.
func debugIsDarkTheme(name string) bool {
	lightKeywords := []string{"light", "day", "latte"}
	for _, kw := range lightKeywords {
		if debugContainsSubstr(name, kw) {
			return false
		}
	}
	return true
}

func debugContainsSubstr(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func debugSortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
