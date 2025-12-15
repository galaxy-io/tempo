package ui

import (
	"github.com/rivo/tview"
)

// App wraps tview.Application with additional functionality.
type App struct {
	*tview.Application
	main     *tview.Flex
	statsBar *StatsBar
	crumbs   *Crumbs
	menu     *Menu
	pages    *Pages
	content  tview.Primitive
}

// NewApp creates a new application wrapper.
func NewApp() *App {
	app := &App{
		Application: tview.NewApplication(),
		statsBar:    NewStatsBar(),
		crumbs:      NewCrumbs(),
		menu:        NewMenu(),
		pages:       NewPages(),
	}
	app.buildLayout()
	return app
}

func (a *App) buildLayout() {
	// Set global background
	tview.Styles.PrimitiveBackgroundColor = ColorBg
	tview.Styles.ContrastBackgroundColor = ColorBgLight
	tview.Styles.MoreContrastBackgroundColor = ColorBgDark
	tview.Styles.BorderColor = ColorBorder
	tview.Styles.TitleColor = ColorAccent
	tview.Styles.PrimaryTextColor = ColorFg
	tview.Styles.SecondaryTextColor = ColorFgDim

	a.main = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statsBar, 3, 0, false).
		AddItem(a.crumbs, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.menu, 1, 0, false)

	a.main.SetBackgroundColor(ColorBg)
	a.SetRoot(a.main, true)
}

// StatsBar returns the stats bar component.
func (a *App) StatsBar() *StatsBar {
	return a.statsBar
}

// Crumbs returns the breadcrumb component.
func (a *App) Crumbs() *Crumbs {
	return a.crumbs
}

// Menu returns the menu component.
func (a *App) Menu() *Menu {
	return a.menu
}

// Pages returns the pages component.
func (a *App) Pages() *Pages {
	return a.pages
}

// SetContent sets the main content area (used by views).
func (a *App) SetContent(p tview.Primitive) {
	a.content = p
	a.pages.SetContent(p)
}

