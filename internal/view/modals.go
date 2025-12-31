package view

import (
	"fmt"
	"strconv"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/layout"
	"github.com/atterpac/jig/theme"
	"github.com/atterpac/jig/theme/themes"
	"github.com/atterpac/jig/util"
	"github.com/atterpac/jig/validators"
	"github.com/galaxy-io/tempo/internal/config"
	"github.com/galaxy-io/tempo/internal/update"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SplashModal displays a splash screen with app info.
type SplashModal struct {
	*components.Modal
	content *tview.TextView
	onClose func()
}

func NewSplashModal() *SplashModal {
	m := &SplashModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    "Tempo",
			Width:    60,
			Height:   18,
			Backdrop: true,
		}),
	}
	m.setup()
	return m
}

func (m *SplashModal) setup() {
	m.content = tview.NewTextView().SetDynamicColors(true)
	m.content.SetBackgroundColor(theme.Bg())
	m.content.SetTextAlign(tview.AlignCenter)

	splashText := fmt.Sprintf(`
[%s::b]   __                      [-:-:-]
[%s::b]  / /   ___   ___  _ __ ___ [-:-:-]
[%s::b] / /   / _ \ / _ \| '_ ' _ \[-:-:-]
[%s::b]/ /___| (_) | (_) | | | | | |[-:-:-]
[%s::b]\_____/\___/ \___/|_| |_| |_|[-:-:-]

[%s]Temporal Workflow Explorer[-]

[%s]Version: %s[-]

[%s]Navigate workflows, schedules, and task queues
with a keyboard-driven interface.[-]

[%s]Press any key to continue...[-]`,
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagFg(),
		theme.TagFgDim(),
		update.Version,
		theme.TagFg(),
		theme.TagFgDim())

	m.content.SetText(splashText)
	m.Modal.SetContent(m.content)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "any key", Description: "Continue"},
	})
	m.Modal.SetOnCancel(func() {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

func (m *SplashModal) SetOnClose(fn func()) { m.onClose = fn }

func (m *SplashModal) Start() {}
func (m *SplashModal) Stop()  {}
func (m *SplashModal) Hints() []KeyHint {
	return []KeyHint{{Key: "any key", Description: "Close"}}
}

func (m *SplashModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		// Any key closes the splash
		if m.onClose != nil {
			m.onClose()
		}
	})
}

// HelpModal displays help information with view-specific keybindings.
type HelpModal struct {
	*components.Modal
	viewName  string
	viewHints []KeyHint
	content   *tview.TextView
}

func NewHelpModal() *HelpModal {
	m := &HelpModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s Help", theme.IconInfo),
			Width:    65,
			Height:   25,
			Backdrop: true,
		}),
	}
	m.setup()
	return m
}

func (m *HelpModal) setup() {
	m.content = tview.NewTextView().SetDynamicColors(true)
	m.content.SetBackgroundColor(theme.Bg())
	m.content.SetScrollable(true)
	m.Modal.SetContent(m.content)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Scroll"},
		{Key: "Esc", Description: "Close"},
	})
}

func (m *HelpModal) SetViewHints(name string, hints []KeyHint) {
	m.viewName = name
	m.viewHints = hints
	m.updateContent()
}

func (m *HelpModal) updateContent() {
	var text string

	// Global keybindings
	text = fmt.Sprintf(`[%s::b]Global Keybindings[-:-:-]

[%s]?[-]          Show help
[%s]T[-]          Change theme
[%s]P[-]          Switch profile
[%s]esc[-]        Go back / Close modal
[%s]q[-]          Quit application

`, theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent())

	// View-specific hints
	if len(m.viewHints) > 0 {
		text += fmt.Sprintf(`[%s::b]%s Keybindings[-:-:-]

`, theme.TagAccent(), m.viewName)

		for _, hint := range m.viewHints {
			text += fmt.Sprintf("[%s]%-12s[-] %s\n", theme.TagAccent(), hint.Key, hint.Description)
		}
	}

	// Navigation tips
	text += fmt.Sprintf(`
[%s::b]Navigation[-:-:-]

[%s]j/↓[-]        Move down
[%s]k/↑[-]        Move up
[%s]g[-]          Go to top
[%s]G[-]          Go to bottom
[%s]Enter[-]      Select / Open
[%s]Tab[-]        Switch panel (where applicable)
`, theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent(),
		theme.TagAccent())

	m.content.SetText(text)
}

func (m *HelpModal) SetOnClose(fn func()) {
	m.Modal.SetOnClose(fn)
	m.Modal.SetOnCancel(fn)
}

// ThemeSelectorModal allows selecting themes.
type ThemeSelectorModal struct {
	*components.Modal
	table       *components.Table
	themes      []string
	currentIdx  int
	onSelect    func(string)
	onCancel    func()
	onPreview   func(string)
	originalIdx int
}

func NewThemeSelectorModal() *ThemeSelectorModal {
	m := &ThemeSelectorModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s Select Theme", theme.IconInfo),
			Width:    50,
			Height:   20,
			Backdrop: true,
		}),
	}
	m.setup()
	return m
}

func (m *ThemeSelectorModal) setup() {
	m.table = components.NewTable()
	m.table.SetHeaders("", "THEME")
	m.table.SetBorder(false)

	m.themes = config.ThemeNames()

	m.table.SetSelectionChangedFunc(func(row, col int) {
		if row >= 0 && row < len(m.themes) {
			m.currentIdx = row
			if m.onPreview != nil {
				m.onPreview(m.themes[row])
			}
		}
	})

	m.table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(m.themes) {
			if m.onSelect != nil {
				m.onSelect(m.themes[row])
			}
		}
	})

	m.Modal.SetContent(m.table)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "j/k", Description: "Navigate"},
		{Key: "Enter", Description: "Select"},
		{Key: "Esc", Description: "Cancel"},
	})
	m.Modal.SetOnCancel(func() {
		// Restore original theme on cancel
		if m.onPreview != nil && m.originalIdx >= 0 && m.originalIdx < len(m.themes) {
			m.onPreview(m.themes[m.originalIdx])
		}
		if m.onCancel != nil {
			m.onCancel()
		}
	})
}

func (m *ThemeSelectorModal) SetThemes(themes []string, currentTheme string) {
	m.themes = themes
	m.table.ClearRows()

	for i, t := range themes {
		marker := " "
		if t == currentTheme {
			marker = "●"
			m.currentIdx = i
			m.originalIdx = i
		}
		m.table.AddRow(marker, t)
	}

	if m.currentIdx < len(themes) {
		m.table.SelectRow(m.currentIdx)
	}
}

func (m *ThemeSelectorModal) SetOnSelect(fn func(string))  { m.onSelect = fn }
func (m *ThemeSelectorModal) SetOnCancel(fn func())        { m.onCancel = fn }
func (m *ThemeSelectorModal) SetOnPreview(fn func(string)) { m.onPreview = fn }

func (m *ThemeSelectorModal) Focus(delegate func(p tview.Primitive)) {
	delegate(m.table)
}

// ProfileModal manages connection profiles.
type ProfileModal struct {
	*components.Modal
	table    *components.Table
	profiles []string
	active   string
	onSelect func(string)
	onNew    func()
	onEdit   func(string)
	onDelete func(string)
	onClose  func()
}

func NewProfileModal() *ProfileModal {
	m := &ProfileModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s Connection Profiles", theme.IconInfo),
			Width:    55,
			Height:   20,
			Backdrop: true,
		}),
	}
	m.setup()
	return m
}

func (m *ProfileModal) setup() {
	m.table = components.NewTable()
	m.table.SetHeaders("", "PROFILE", "ADDRESS")
	m.table.SetBorder(false)

	m.table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(m.profiles) {
			if m.onSelect != nil {
				m.onSelect(m.profiles[row])
			}
		}
	})

	m.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'n':
			if m.onNew != nil {
				m.onNew()
			}
			return nil
		case 'e':
			row := m.table.SelectedRow()
			if row >= 0 && row < len(m.profiles) && m.onEdit != nil {
				m.onEdit(m.profiles[row])
			}
			return nil
		case 'd':
			row := m.table.SelectedRow()
			if row >= 0 && row < len(m.profiles) && m.profiles[row] != m.active && m.onDelete != nil {
				m.onDelete(m.profiles[row])
			}
			return nil
		}
		return event
	})

	m.Modal.SetContent(m.table)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Switch"},
		{Key: "n", Description: "New"},
		{Key: "e", Description: "Edit"},
		{Key: "d", Description: "Delete"},
		{Key: "Esc", Description: "Close"},
	})
	m.Modal.SetOnCancel(func() {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

func (m *ProfileModal) SetProfiles(profiles []string, active string) {
	m.profiles = profiles
	m.active = active
	m.table.ClearRows()

	cfg, _ := config.Load()
	currentIdx := 0

	for i, name := range profiles {
		marker := " "
		if name == active {
			marker = "●"
			currentIdx = i
		}
		address := ""
		if cfg != nil {
			if profile, ok := cfg.GetProfile(name); ok {
				address = profile.Address
			}
		}
		m.table.AddRow(marker, name, truncateMiddle(address, 25))
	}

	if len(profiles) > 0 {
		m.table.SelectRow(currentIdx)
	}
}

func (m *ProfileModal) SetOnSelect(fn func(string)) { m.onSelect = fn }
func (m *ProfileModal) SetOnNew(fn func())          { m.onNew = fn }
func (m *ProfileModal) SetOnEdit(fn func(string))   { m.onEdit = fn }
func (m *ProfileModal) SetOnDelete(fn func(string)) { m.onDelete = fn }
func (m *ProfileModal) SetOnClose(fn func())        { m.onClose = fn }

func (m *ProfileModal) Focus(delegate func(p tview.Primitive)) {
	delegate(m.table)
}

// ProfileForm for creating/editing profiles.
type ProfileForm struct {
	*components.Modal
	form     *components.Form
	isEdit   bool
	editName string
	onSave   func(string, config.ConnectionConfig)
	onCancel func()
}

func NewProfileForm() *ProfileForm {
	f := &ProfileForm{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s New Profile", theme.IconInfo),
			Width:    60,
			Height:   22,
			Backdrop: true,
		}),
	}
	f.setup()
	return f
}

func (f *ProfileForm) setup() {
	f.form = f.buildForm("", config.ConnectionConfig{
		Address:   "localhost:7233",
		Namespace: "default",
	}, false)

	f.Modal.SetContent(f.form)
	f.Modal.SetHints([]components.KeyHint{
		{Key: "Tab", Description: "Next field"},
		{Key: "Ctrl+S", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})
}

func (f *ProfileForm) buildForm(name string, cfg config.ConnectionConfig, isEdit bool) *components.Form {
	builder := components.NewFormBuilder()

	// Name field - required for new profiles
	if isEdit {
		builder.Text("name", "Profile Name").
			Value(name).
			Done()
	} else {
		builder.Text("name", "Profile Name").
			Placeholder("Enter profile name").
			Validate(validators.Required(), validators.MinLength(1)).
			Done()
	}

	// Connection settings
	builder.Text("address", "Server Address").
		Placeholder("localhost:7233").
		Value(cfg.Address).
		Validate(validators.Required()).
		Done()

	builder.Text("namespace", "Default Namespace").
		Placeholder("default").
		Value(cfg.Namespace).
		Done()

	// TLS settings (optional)
	builder.Text("tlsCert", "TLS Cert Path (optional)").
		Value(cfg.TLS.Cert).
		Done()

	builder.Text("tlsKey", "TLS Key Path (optional)").
		Value(cfg.TLS.Key).
		Done()

	builder.Text("tlsCA", "TLS CA Path (optional)").
		Value(cfg.TLS.CA).
		Done()

	builder.Text("tlsServerName", "TLS Server Name (optional)").
		Value(cfg.TLS.ServerName).
		Done()

	// Skip TLS verify
	skipVerifyDefault := "No"
	if cfg.TLS.SkipVerify {
		skipVerifyDefault = "Yes"
	}
	builder.Select("tlsSkipVerify", "Skip TLS Verify", []string{"No", "Yes"}).
		Default(skipVerifyDefault).
		Done()

	// Set callbacks
	builder.OnSubmit(func(values map[string]any) {
		saveName := name
		if !isEdit {
			saveName = values["name"].(string)
		}
		if saveName == "" {
			return
		}

		skipVerify := values["tlsSkipVerify"].(string) == "Yes"

		newCfg := config.ConnectionConfig{
			Address:   values["address"].(string),
			Namespace: values["namespace"].(string),
			TLS: config.TLSConfig{
				Cert:       values["tlsCert"].(string),
				Key:        values["tlsKey"].(string),
				CA:         values["tlsCA"].(string),
				ServerName: values["tlsServerName"].(string),
				SkipVerify: skipVerify,
			},
		}

		if f.onSave != nil {
			f.onSave(saveName, newCfg)
		}
	})

	builder.OnCancel(func() {
		if f.onCancel != nil {
			f.onCancel()
		}
	})

	return builder.Build()
}

func (f *ProfileForm) SetProfile(name string, cfg config.ConnectionConfig) {
	f.isEdit = name != ""
	f.editName = name

	if f.isEdit {
		f.Modal.SetTitle(fmt.Sprintf("%s Edit Profile: %s", theme.IconInfo, name))
	} else {
		f.Modal.SetTitle(fmt.Sprintf("%s New Profile", theme.IconInfo))
	}

	f.form = f.buildForm(name, cfg, f.isEdit)
	f.Modal.SetContent(f.form)
}

func (f *ProfileForm) SetOnSave(fn func(string, config.ConnectionConfig)) { f.onSave = fn }
func (f *ProfileForm) SetOnCancel(fn func())                              { f.onCancel = fn }

func (f *ProfileForm) Focus(delegate func(p tview.Primitive)) {
	f.form.Focus(delegate)
}

// Helper function to truncate string in the middle
func truncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 5 {
		return s[:maxLen]
	}
	half := (maxLen - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}

// DeleteConfirmModal shows a confirmation dialog for deletion.
type DeleteConfirmModal struct {
	*components.Modal
	itemName  string
	itemType  string
	onConfirm func()
	onCancel  func()
}

func NewDeleteConfirmModal(itemType, itemName string) *DeleteConfirmModal {
	m := &DeleteConfirmModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s Delete %s", theme.IconError, itemType),
			Width:    50,
			Height:   10,
			Backdrop: true,
		}),
		itemName: itemName,
		itemType: itemType,
	}
	m.setup()
	return m
}

func (m *DeleteConfirmModal) setup() {
	content := tview.NewTextView().SetDynamicColors(true)
	content.SetBackgroundColor(theme.Bg())
	content.SetTextAlign(tview.AlignCenter)
	content.SetText(fmt.Sprintf(`[%s]Are you sure you want to delete[-]
[%s::b]%s[-:-:-]?

[%s]This action cannot be undone.[-]`,
		theme.TagFg(),
		theme.TagAccent(), m.itemName,
		theme.TagError()))

	m.Modal.SetContent(content)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "y", Description: "Yes, delete"},
		{Key: "n/Esc", Description: "Cancel"},
	})
	m.Modal.SetOnCancel(func() {
		if m.onCancel != nil {
			m.onCancel()
		}
	})
}

func (m *DeleteConfirmModal) SetOnConfirm(fn func()) { m.onConfirm = fn }
func (m *DeleteConfirmModal) SetOnCancel(fn func())  { m.onCancel = fn }

func (m *DeleteConfirmModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Rune() {
		case 'y', 'Y':
			if m.onConfirm != nil {
				m.onConfirm()
			}
		case 'n', 'N':
			if m.onCancel != nil {
				m.onCancel()
			}
		}
		if event.Key() == tcell.KeyEscape {
			if m.onCancel != nil {
				m.onCancel()
			}
		}
	})
}

// ErrorModal displays an error message.
type ErrorModal struct {
	*components.Modal
	message string
	onClose func()
}

func NewErrorModal(title, message string) *ErrorModal {
	m := &ErrorModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s %s", theme.IconError, title),
			Width:    55,
			Height:   12,
			Backdrop: true,
		}),
		message: message,
	}
	m.setup()
	return m
}

func (m *ErrorModal) setup() {
	content := tview.NewTextView().SetDynamicColors(true)
	content.SetBackgroundColor(theme.Bg())
	content.SetTextAlign(tview.AlignCenter)
	content.SetText(fmt.Sprintf(`[%s]%s[-]

[%s]Press any key to close.[-]`,
		theme.TagError(), m.message,
		theme.TagFgDim()))

	m.Modal.SetContent(content)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "any key", Description: "Close"},
	})
	m.Modal.SetOnCancel(func() {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

func (m *ErrorModal) SetOnClose(fn func()) { m.onClose = fn }

func (m *ErrorModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

// InfoModal displays an informational message.
type InfoModal struct {
	*components.Modal
	onClose func()
}

func NewInfoModal(title, message string) *InfoModal {
	m := &InfoModal{
		Modal: components.NewModal(components.ModalConfig{
			Title:    fmt.Sprintf("%s %s", theme.IconInfo, title),
			Width:    55,
			Height:   12,
			Backdrop: true,
		}),
	}
	m.setup(message)
	return m
}

func (m *InfoModal) setup(message string) {
	content := tview.NewTextView().SetDynamicColors(true)
	content.SetBackgroundColor(theme.Bg())
	content.SetTextAlign(tview.AlignCenter)
	content.SetText(fmt.Sprintf(`[%s]%s[-]

[%s]Press any key to close.[-]`,
		theme.TagFg(), message,
		theme.TagFgDim()))

	m.Modal.SetContent(content)
	m.Modal.SetHints([]components.KeyHint{
		{Key: "any key", Description: "Close"},
	})
	m.Modal.SetOnCancel(func() {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

func (m *InfoModal) SetOnClose(fn func()) { m.onClose = fn }

func (m *InfoModal) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		if m.onClose != nil {
			m.onClose()
		}
	})
}

// ShowErrorModal displays an error modal and handles cleanup on close.
func ShowErrorModal(app *layout.App, title, message string) {
	modal := NewErrorModal(title, message)
	modal.SetOnClose(func() {
		app.Pages().DismissModal()
	})
	app.Pages().Push(modal)
	app.SetFocus(modal)
}

// ShowInfoModal displays an info modal and handles cleanup on close.
func ShowInfoModal(app *layout.App, title, message string) {
	modal := NewInfoModal(title, message)
	modal.SetOnClose(func() {
		app.Pages().DismissModal()
	})
	app.Pages().Push(modal)
	app.SetFocus(modal)
}

// SplashTestView displays a full-screen splash for testing themes and gradients.
type SplashTestView struct {
	*tview.Box
	flex          *tview.Flex
	logoContainer *tview.Flex
	logoView      *tview.TextView
	statusView    *tview.TextView
	hintsView     *tview.TextView
	sponsorView   *tview.TextView
	leftSpacer    *tview.Box
	rightSpacer   *tview.Box
	topSpacer     *tview.Box
	midSpacer     *tview.Box
	bottomSpacer  *tview.Box
	themes        []string
	currentTheme  int
	gradientType  int // 0=diagonal, 1=reverse diagonal, 2=horizontal, 3=vertical
	onClose       func()
	onThemeChange func(string)
}

// Logo for splash screen testing (same as main splash)
const splashTestLogo = `
░▒▓████████▓▒░▒▓████████▓▒░▒▓██████████████▓▒░░▒▓███████▓▒░ ░▒▓██████▓▒░
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
   ░▒▓█▓▒░   ░▒▓██████▓▒░ ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓███████▓▒░░▒▓█▓▒░░▒▓█▓▒░
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░
   ░▒▓█▓▒░   ░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░
   ░▒▓█▓▒░   ░▒▓████████▓▒░▒▓█▓▒░░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░       ░▒▓██████▓▒░
`

func NewSplashTestView(currentThemeName string) *SplashTestView {
	v := &SplashTestView{
		Box:          tview.NewBox().SetBackgroundColor(theme.Bg()),
		themes:       themes.Names(),
		gradientType: 0,
	}

	// Find current theme index
	for i, name := range v.themes {
		if name == currentThemeName {
			v.currentTheme = i
			break
		}
	}

	v.setup()
	return v
}

func (v *SplashTestView) setup() {
	// Logo display - left aligned to preserve internal spacing
	v.logoView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.logoView.SetBackgroundColor(theme.Bg())

	// Status display (theme/gradient info)
	v.statusView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	v.statusView.SetBackgroundColor(theme.Bg())

	// Hints display
	v.hintsView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	v.hintsView.SetBackgroundColor(theme.Bg())

	// Sponsor display
	v.sponsorView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	v.sponsorView.SetBackgroundColor(theme.Bg())

	// Create spacers
	v.leftSpacer = tview.NewBox().SetBackgroundColor(theme.Bg())
	v.rightSpacer = tview.NewBox().SetBackgroundColor(theme.Bg())
	v.topSpacer = tview.NewBox().SetBackgroundColor(theme.Bg())
	v.midSpacer = tview.NewBox().SetBackgroundColor(theme.Bg())
	v.bottomSpacer = tview.NewBox().SetBackgroundColor(theme.Bg())

	// Center the logo horizontally
	v.logoContainer = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(v.leftSpacer, 0, 1, false).
		AddItem(v.logoView, 78, 0, false).
		AddItem(v.rightSpacer, 0, 1, false)
	v.logoContainer.SetBackgroundColor(theme.Bg())

	// Build layout matching main splash screen
	v.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.topSpacer, 0, 1, false).
		AddItem(v.logoContainer, 9, 0, false).
		AddItem(v.statusView, 3, 0, false).
		AddItem(v.hintsView, 1, 0, false).
		AddItem(v.midSpacer, 1, 0, false).
		AddItem(v.sponsorView, 1, 0, false).
		AddItem(v.bottomSpacer, 0, 1, false)
	v.flex.SetBackgroundColor(theme.Bg())

	v.updateDisplay()
}

func (v *SplashTestView) updateDisplay() {
	// Get gradient colors from current theme
	gradientColors := util.DefaultGradientColors()

	// Apply gradient based on type
	var gradientLogo string
	var gradientName string
	switch v.gradientType {
	case 0:
		gradientLogo = util.ApplyDiagonalGradient(splashTestLogo, gradientColors)
		gradientName = "Diagonal"
	case 1:
		gradientLogo = util.ApplyReverseDiagonalGradient(splashTestLogo, gradientColors)
		gradientName = "Reverse Diagonal"
	case 2:
		gradientLogo = util.ApplyHorizontalGradient(splashTestLogo, gradientColors)
		gradientName = "Horizontal"
	case 3:
		gradientLogo = util.ApplyVerticalGradient(splashTestLogo, gradientColors)
		gradientName = "Vertical"
	}

	v.logoView.SetText(gradientLogo)

	// Update status
	themeName := ""
	if v.currentTheme >= 0 && v.currentTheme < len(v.themes) {
		themeName = v.themes[v.currentTheme]
	}
	v.statusView.SetText(fmt.Sprintf(
		"[%s]Theme: [%s::b]%s[-:-:-] [%s](%d/%d)[-]  [%s]Gradient: [%s::b]%s[-:-:-]",
		theme.TagFgDim(),
		theme.TagAccent(), themeName,
		theme.TagFgDim(), v.currentTheme+1, len(v.themes),
		theme.TagFgDim(),
		theme.TagAccent(), gradientName,
	))

	// Update hints
	v.hintsView.SetText(fmt.Sprintf(
		"[%s]n/p[-] Next/Prev theme  [%s]g[-] Cycle gradient  [%s]Esc[-] Close",
		theme.TagAccent(), theme.TagAccent(), theme.TagAccent(),
	))

	// Update sponsor
	v.sponsorView.SetText(fmt.Sprintf(
		"[%s]Made with %s  by getgalaxy.io[-]",
		theme.TagFgDim(), theme.IconHeart,
	))

	// Update all backgrounds
	v.Box.SetBackgroundColor(theme.Bg())
	v.logoView.SetBackgroundColor(theme.Bg())
	v.statusView.SetBackgroundColor(theme.Bg())
	v.hintsView.SetBackgroundColor(theme.Bg())
	v.sponsorView.SetBackgroundColor(theme.Bg())
	v.flex.SetBackgroundColor(theme.Bg())
	v.logoContainer.SetBackgroundColor(theme.Bg())
	v.leftSpacer.SetBackgroundColor(theme.Bg())
	v.rightSpacer.SetBackgroundColor(theme.Bg())
	v.topSpacer.SetBackgroundColor(theme.Bg())
	v.midSpacer.SetBackgroundColor(theme.Bg())
	v.bottomSpacer.SetBackgroundColor(theme.Bg())
}

func (v *SplashTestView) SetOnClose(fn func())             { v.onClose = fn }
func (v *SplashTestView) SetOnThemeChange(fn func(string)) { v.onThemeChange = fn }

func (v *SplashTestView) Draw(screen tcell.Screen) {
	v.Box.DrawForSubclass(screen, v)
	x, y, width, height := v.GetInnerRect()
	v.flex.SetRect(x, y, width, height)
	v.flex.Draw(screen)
}

func (v *SplashTestView) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return v.WrapInputHandler(func(event *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch event.Rune() {
		case 'n': // Next theme
			if len(v.themes) > 0 {
				v.currentTheme = (v.currentTheme + 1) % len(v.themes)
				v.applyTheme()
			}
		case 'p': // Previous theme
			if len(v.themes) > 0 {
				v.currentTheme = (v.currentTheme - 1 + len(v.themes)) % len(v.themes)
				v.applyTheme()
			}
		case 'g': // Cycle gradient type
			v.gradientType = (v.gradientType + 1) % 4
			v.updateDisplay()
		}

		if event.Key() == tcell.KeyEscape {
			if v.onClose != nil {
				v.onClose()
			}
		}
	})
}

func (v *SplashTestView) applyTheme() {
	if v.currentTheme >= 0 && v.currentTheme < len(v.themes) {
		themeName := v.themes[v.currentTheme]
		newTheme := themes.Get(themeName)
		if newTheme != nil {
			theme.SetProvider(newTheme)
			if v.onThemeChange != nil {
				v.onThemeChange(themeName)
			}
		}
	}
	v.updateDisplay()
}

// Unused but prevents import errors
var _ = strconv.Itoa
