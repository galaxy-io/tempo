package ui

import (
	"fmt"
	"strings"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// EventOption represents a selectable event for reset.
type EventOption struct {
	EventID int64
	Type    string
	Time    string
	Details string
}

// EventSelectorModal displays a list of events to select from for workflow reset.
type EventSelectorModal struct {
	*Modal
	title        string
	message      string
	events       []EventOption
	filteredRows []int64 // EventIDs of filtered (displayable) events
	onSelect     func(eventID int64)
	onCancel     func()

	// Internal components
	table *tview.Table
	nav   *TableListNavigator
}

// NewEventSelectorModal creates an event selector modal.
func NewEventSelectorModal(title, message string, events []EventOption) *EventSelectorModal {
	em := &EventSelectorModal{
		Modal: NewModal(ModalConfig{
			Title:     title,
			Width:     70,
			Height:    12,
			MinHeight: 8,
			MaxHeight: 20,
			Backdrop:  true,
		}),
		title:   title,
		message: message,
		events:  events,
	}
	em.setup()
	return em
}

// SetOnSelect sets the selection callback.
func (em *EventSelectorModal) SetOnSelect(fn func(eventID int64)) *EventSelectorModal {
	em.onSelect = fn
	return em
}

// SetOnCancel sets the cancel callback.
func (em *EventSelectorModal) SetOnCancel(fn func()) *EventSelectorModal {
	em.onCancel = fn
	em.Modal.SetOnClose(fn)
	return em
}

func (em *EventSelectorModal) setup() {
	em.table = tview.NewTable()
	em.table.SetBackgroundColor(ColorBg())
	em.table.SetSelectable(true, false)
	em.table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(ColorBg()).
		Background(ColorAccent()))

	em.buildTable()

	// 1 header row
	em.nav = NewTableListNavigator(em.table, 1)

	em.SetContent(em.table)
	em.SetHints([]KeyHint{
		{Key: "j/k", Description: "Navigate"},
		{Key: "Enter", Description: "Select"},
		{Key: "Esc", Description: "Cancel"},
	})

	// Adjust height based on filtered event count
	height := len(em.filteredRows) + 4
	if height < 8 {
		height = 8
	}
	if height > 18 {
		height = 18
	}
	em.SetSize(70, height)

	// Register for theme changes
	OnThemeChange(func(_ *config.ParsedTheme) {
		em.table.SetBackgroundColor(ColorBg())
		em.table.SetSelectedStyle(tcell.StyleDefault.
			Foreground(ColorBg()).
			Background(ColorAccent()))
		em.buildTable()
	})
}

func (em *EventSelectorModal) buildTable() {
	em.table.Clear()
	em.filteredRows = nil

	// Add header row
	headers := []string{"ID", "TYPE", "TIME"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(ColorTableHdr()).
			SetBackgroundColor(ColorBg()).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		em.table.SetCell(0, col, cell)
	}

	// Add events (only WorkflowTaskCompleted events are valid reset points)
	row := 1
	for _, ev := range em.events {
		// Filter to only show valid reset points (WorkflowTaskCompleted events)
		if !strings.Contains(ev.Type, "WorkflowTaskCompleted") {
			continue
		}

		em.table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%d", ev.EventID)).
			SetTextColor(ColorFg()).
			SetBackgroundColor(ColorBg()))
		em.table.SetCell(row, 1, tview.NewTableCell(ev.Type).
			SetTextColor(ColorFg()).
			SetBackgroundColor(ColorBg()))
		em.table.SetCell(row, 2, tview.NewTableCell(ev.Time).
			SetTextColor(ColorFgDim()).
			SetBackgroundColor(ColorBg()))

		em.filteredRows = append(em.filteredRows, ev.EventID)
		row++
	}

	// If no valid events, show a message
	if row == 1 {
		em.table.SetCell(1, 0, tview.NewTableCell("No valid reset points found").
			SetTextColor(ColorFgDim()).
			SetBackgroundColor(ColorBg()).
			SetSelectable(false))
	}

	em.table.SetFixed(1, 0) // Fix header row

	// Select first data row
	if row > 1 {
		em.table.Select(1, 0)
	}
}

func (em *EventSelectorModal) getSelectedEventID() int64 {
	idx := em.nav.GetSelectedIndex()
	if idx >= 0 && idx < len(em.filteredRows) {
		return em.filteredRows[idx]
	}
	return 0
}

// InputHandler handles keyboard input.
func (em *EventSelectorModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return em.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEscape:
			if em.onCancel != nil {
				em.onCancel()
			}
		case tcell.KeyEnter:
			eventID := em.getSelectedEventID()
			if eventID > 0 && em.onSelect != nil {
				em.onSelect(eventID)
			}
		case tcell.KeyUp:
			em.nav.MoveUp()
		case tcell.KeyDown:
			em.nav.MoveDown()
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				em.nav.MoveDown()
			case 'k':
				em.nav.MoveUp()
			case 'q':
				if em.onCancel != nil {
					em.onCancel()
				}
			}
		}
	})
}

// Focus delegates focus to the table.
func (em *EventSelectorModal) Focus(delegate func(p tview.Primitive)) {
	delegate(em.table)
}
