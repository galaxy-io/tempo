package ui

import (
	"fmt"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FilterPicker provides a modal to select, save, and manage saved filters.
type FilterPicker struct {
	*tview.Box
	filters       []config.SavedFilter
	selectedIndex int
	mode          filterPickerMode

	onSelect   func(filter config.SavedFilter)
	onSave     func(name string, query string, isDefault bool)
	onDelete   func(name string)
	onSetDefault func(name string)
	onCancel   func()

	// For save mode
	saveNameInput string
	saveCursor    int
	currentQuery  string
	saveAsDefault bool
}

type filterPickerMode int

const (
	filterPickerModeSelect filterPickerMode = iota
	filterPickerModeSave
)

// NewFilterPicker creates a new filter picker.
func NewFilterPicker(filters []config.SavedFilter, currentQuery string) *FilterPicker {
	fp := &FilterPicker{
		Box:          tview.NewBox(),
		filters:      filters,
		currentQuery: currentQuery,
	}
	fp.SetBackgroundColor(ColorBg())

	OnThemeChange(func(_ *config.ParsedTheme) {
		fp.SetBackgroundColor(ColorBg())
	})

	return fp
}

// SetOnSelect sets the callback for when a filter is selected.
func (fp *FilterPicker) SetOnSelect(fn func(filter config.SavedFilter)) {
	fp.onSelect = fn
}

// SetOnSave sets the callback for when a new filter is saved.
func (fp *FilterPicker) SetOnSave(fn func(name string, query string, isDefault bool)) {
	fp.onSave = fn
}

// SetOnDelete sets the callback for when a filter is deleted.
func (fp *FilterPicker) SetOnDelete(fn func(name string)) {
	fp.onDelete = fn
}

// SetOnSetDefault sets the callback for when a filter is set as default.
func (fp *FilterPicker) SetOnSetDefault(fn func(name string)) {
	fp.onSetDefault = fn
}

// SetOnCancel sets the cancel callback.
func (fp *FilterPicker) SetOnCancel(fn func()) {
	fp.onCancel = fn
}

// GetHeight returns the preferred height for this component.
func (fp *FilterPicker) GetHeight() int {
	if fp.mode == filterPickerModeSave {
		return 8
	}
	rows := len(fp.filters)
	if rows == 0 {
		rows = 1 // "No saved filters" message
	}
	return rows + 5 // filters + border + title + hints + save hint
}

// Draw renders the filter picker.
func (fp *FilterPicker) Draw(screen tcell.Screen) {
	fp.Box.DrawForSubclass(screen, fp)

	x, y, width, height := fp.GetInnerRect()
	if width <= 0 || height < 3 {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(ColorPanelBorder()).Background(ColorBg())
	titleStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg()).Bold(true)
	itemStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())
	selectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())
	descStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	hintStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	defaultStyle := tcell.StyleDefault.Foreground(ColorCompleted()).Background(ColorBg())
	inputStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBgLight())

	// Draw border
	screen.SetContent(x, y, '╭', nil, borderStyle)
	screen.SetContent(x+width-1, y, '╮', nil, borderStyle)
	screen.SetContent(x, y+height-1, '╰', nil, borderStyle)
	screen.SetContent(x+width-1, y+height-1, '╯', nil, borderStyle)

	for i := x + 1; i < x+width-1; i++ {
		screen.SetContent(i, y, '─', nil, borderStyle)
		screen.SetContent(i, y+height-1, '─', nil, borderStyle)
	}
	for i := y + 1; i < y+height-1; i++ {
		screen.SetContent(x, i, '│', nil, borderStyle)
		screen.SetContent(x+width-1, i, '│', nil, borderStyle)
	}

	if fp.mode == filterPickerModeSave {
		fp.drawSaveMode(screen, x, y, width, height, titleStyle, itemStyle, inputStyle, hintStyle, borderStyle)
		return
	}

	// Title
	title := " Saved Filters "
	titleX := x + (width-len(title))/2
	for i, r := range []rune(title) {
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Draw filters or empty state
	if len(fp.filters) == 0 {
		emptyMsg := "No saved filters. Press 's' to save current query."
		msgX := x + (width-len(emptyMsg))/2
		msgY := y + height/2
		if msgY >= y+1 && msgY < y+height-1 {
			for i, r := range []rune(emptyMsg) {
				if msgX+i > x && msgX+i < x+width-1 {
					screen.SetContent(msgX+i, msgY, r, nil, descStyle)
				}
			}
		}
	} else {
		for i, filter := range fp.filters {
			rowY := y + 1 + i
			if rowY >= y+height-2 {
				break
			}

			style := itemStyle
			dStyle := descStyle
			if i == fp.selectedIndex {
				style = selectedStyle
				dStyle = selectedStyle
			}

			// Clear row
			for cx := x + 1; cx < x+width-1; cx++ {
				screen.SetContent(cx, rowY, ' ', nil, style)
			}

			// Draw marker
			marker := "  "
			if i == fp.selectedIndex {
				marker = IconArrowRight + " "
			}
			for mi, r := range []rune(marker) {
				if x+2+mi < x+width-1 {
					screen.SetContent(x+2+mi, rowY, r, nil, style)
				}
			}

			// Draw default indicator
			nameStart := x + 4
			if filter.IsDefault {
				defMarker := IconCompleted + " "
				defStyle := defaultStyle
				if i == fp.selectedIndex {
					defStyle = selectedStyle
				}
				for di, r := range []rune(defMarker) {
					if nameStart+di < x+width-1 {
						screen.SetContent(nameStart+di, rowY, r, nil, defStyle)
					}
				}
				nameStart += 2
			}

			// Draw name
			for ni, r := range []rune(filter.Name) {
				if nameStart+ni < x+width-1 {
					screen.SetContent(nameStart+ni, rowY, r, nil, style)
				}
			}

			// Draw query (truncated)
			queryX := x + 25
			if queryX < x+width-10 {
				query := filter.Query
				maxLen := width - 30
				if maxLen > 0 && len(query) > maxLen {
					query = query[:maxLen-3] + "..."
				}
				for qi, r := range []rune(query) {
					if queryX+qi < x+width-2 {
						screen.SetContent(queryX+qi, rowY, r, nil, dStyle)
					}
				}
			}
		}
	}

	// Draw hints at bottom
	hintY := y + height - 1
	var hint string
	if len(fp.filters) > 0 {
		hint = " [Enter] Apply  [d] Delete  [*] Set Default  [s] Save New  [Esc] Cancel "
	} else {
		hint = " [s] Save Current Query  [Esc] Cancel "
	}
	hintX := x + (width-len(hint))/2
	for i, r := range []rune(hint) {
		if hintX+i > x && hintX+i < x+width-1 {
			screen.SetContent(hintX+i, hintY, r, nil, hintStyle)
		}
	}
}

func (fp *FilterPicker) drawSaveMode(screen tcell.Screen, x, y, width, height int,
	titleStyle, itemStyle, inputStyle, hintStyle, borderStyle tcell.Style) {

	// Title
	title := " Save Filter "
	titleX := x + (width-len(title))/2
	for i, r := range []rune(title) {
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Name input
	labelY := y + 2
	label := "Name: "
	for li, r := range []rune(label) {
		if x+3+li < x+width-1 {
			screen.SetContent(x+3+li, labelY, r, nil, itemStyle)
		}
	}

	inputX := x + 3 + len(label)
	inputWidth := width - len(label) - 8
	for i := 0; i < inputWidth; i++ {
		ch := ' '
		if i < len(fp.saveNameInput) {
			ch = rune(fp.saveNameInput[i])
		}
		if inputX+i < x+width-2 {
			screen.SetContent(inputX+i, labelY, ch, nil, inputStyle)
		}
	}

	// Draw cursor
	cursorX := inputX + fp.saveCursor
	if cursorX < inputX+inputWidth && cursorX < x+width-2 {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorFg())
		ch := ' '
		if fp.saveCursor < len(fp.saveNameInput) {
			ch = rune(fp.saveNameInput[fp.saveCursor])
		}
		screen.SetContent(cursorX, labelY, ch, nil, cursorStyle)
	}

	// Query preview
	queryY := y + 3
	queryLabel := "Query: "
	for ql, r := range []rune(queryLabel) {
		if x+3+ql < x+width-1 {
			screen.SetContent(x+3+ql, queryY, r, nil, itemStyle)
		}
	}
	query := fp.currentQuery
	maxQueryLen := width - len(queryLabel) - 8
	if len(query) > maxQueryLen {
		query = query[:maxQueryLen-3] + "..."
	}
	queryX := x + 3 + len(queryLabel)
	for qi, r := range []rune(query) {
		if queryX+qi < x+width-2 {
			screen.SetContent(queryX+qi, queryY, r, nil, hintStyle)
		}
	}

	// Default checkbox
	defaultY := y + 4
	checkbox := "[ ] Set as default"
	if fp.saveAsDefault {
		checkbox = "[" + IconCompleted + "] Set as default"
	}
	for ci, r := range []rune(checkbox) {
		if x+3+ci < x+width-1 {
			screen.SetContent(x+3+ci, defaultY, r, nil, itemStyle)
		}
	}

	// Hints
	hintY := y + height - 1
	hint := " [Enter] Save  [Tab] Toggle Default  [Esc] Cancel "
	hintX := x + (width-len(hint))/2
	for i, r := range []rune(hint) {
		if hintX+i > x && hintX+i < x+width-1 {
			screen.SetContent(hintX+i, hintY, r, nil, hintStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (fp *FilterPicker) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return fp.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if fp.mode == filterPickerModeSave {
			fp.handleSaveMode(event)
			return
		}

		fp.handleSelectMode(event)
	})
}

func (fp *FilterPicker) handleSelectMode(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		if len(fp.filters) > 0 && fp.selectedIndex >= 0 && fp.selectedIndex < len(fp.filters) {
			if fp.onSelect != nil {
				fp.onSelect(fp.filters[fp.selectedIndex])
			}
		}
	case tcell.KeyEscape:
		if fp.onCancel != nil {
			fp.onCancel()
		}
	case tcell.KeyUp:
		if len(fp.filters) > 0 {
			fp.selectedIndex--
			if fp.selectedIndex < 0 {
				fp.selectedIndex = len(fp.filters) - 1
			}
		}
	case tcell.KeyDown:
		if len(fp.filters) > 0 {
			fp.selectedIndex++
			if fp.selectedIndex >= len(fp.filters) {
				fp.selectedIndex = 0
			}
		}
	case tcell.KeyRune:
		switch event.Rune() {
		case 'j':
			if len(fp.filters) > 0 {
				fp.selectedIndex++
				if fp.selectedIndex >= len(fp.filters) {
					fp.selectedIndex = 0
				}
			}
		case 'k':
			if len(fp.filters) > 0 {
				fp.selectedIndex--
				if fp.selectedIndex < 0 {
					fp.selectedIndex = len(fp.filters) - 1
				}
			}
		case 'd':
			// Delete selected filter
			if len(fp.filters) > 0 && fp.selectedIndex >= 0 && fp.selectedIndex < len(fp.filters) {
				if fp.onDelete != nil {
					fp.onDelete(fp.filters[fp.selectedIndex].Name)
				}
			}
		case '*':
			// Set as default
			if len(fp.filters) > 0 && fp.selectedIndex >= 0 && fp.selectedIndex < len(fp.filters) {
				if fp.onSetDefault != nil {
					fp.onSetDefault(fp.filters[fp.selectedIndex].Name)
				}
			}
		case 's':
			// Switch to save mode if there's a current query
			if fp.currentQuery != "" {
				fp.mode = filterPickerModeSave
				fp.saveNameInput = ""
				fp.saveCursor = 0
				fp.saveAsDefault = false
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// Quick select by number
			idx := int(event.Rune() - '1')
			if idx >= 0 && idx < len(fp.filters) {
				fp.selectedIndex = idx
				if fp.onSelect != nil {
					fp.onSelect(fp.filters[fp.selectedIndex])
				}
			}
		}
	}
}

func (fp *FilterPicker) handleSaveMode(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		if fp.saveNameInput != "" && fp.onSave != nil {
			fp.onSave(fp.saveNameInput, fp.currentQuery, fp.saveAsDefault)
		}
	case tcell.KeyEscape:
		fp.mode = filterPickerModeSelect
	case tcell.KeyTab:
		fp.saveAsDefault = !fp.saveAsDefault
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if fp.saveCursor > 0 {
			fp.saveNameInput = fp.saveNameInput[:fp.saveCursor-1] + fp.saveNameInput[fp.saveCursor:]
			fp.saveCursor--
		}
	case tcell.KeyDelete:
		if fp.saveCursor < len(fp.saveNameInput) {
			fp.saveNameInput = fp.saveNameInput[:fp.saveCursor] + fp.saveNameInput[fp.saveCursor+1:]
		}
	case tcell.KeyLeft:
		if fp.saveCursor > 0 {
			fp.saveCursor--
		}
	case tcell.KeyRight:
		if fp.saveCursor < len(fp.saveNameInput) {
			fp.saveCursor++
		}
	case tcell.KeyRune:
		r := event.Rune()
		// Allow alphanumeric, spaces, dashes, underscores
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == ' ' || r == '-' || r == '_' {
			fp.saveNameInput = fp.saveNameInput[:fp.saveCursor] + string(r) + fp.saveNameInput[fp.saveCursor:]
			fp.saveCursor++
		}
	}
}

// Focus sets focus to this picker.
func (fp *FilterPicker) Focus(delegate func(p tview.Primitive)) {
	fp.Box.Focus(delegate)
}

// UpdateFilters updates the filters list.
func (fp *FilterPicker) UpdateFilters(filters []config.SavedFilter) {
	fp.filters = filters
	if fp.selectedIndex >= len(filters) {
		fp.selectedIndex = len(filters) - 1
		if fp.selectedIndex < 0 {
			fp.selectedIndex = 0
		}
	}
}

// ShowSaveMode switches to save mode.
func (fp *FilterPicker) ShowSaveMode() {
	fp.mode = filterPickerModeSave
	fp.saveNameInput = ""
	fp.saveCursor = 0
	fp.saveAsDefault = false
}

// HasFilters returns true if there are saved filters.
func (fp *FilterPicker) HasFilters() bool {
	return len(fp.filters) > 0
}

// QuickSaveFilter creates a new filter without showing the full picker.
type QuickSaveDialog struct {
	*Modal
	nameInput    string
	cursorPos    int
	setDefault   bool
	currentQuery string
	onSave       func(name string, isDefault bool)
	onCancel     func()
}

// NewQuickSaveDialog creates a quick save dialog.
func NewQuickSaveDialog(query string) *QuickSaveDialog {
	qsd := &QuickSaveDialog{
		Modal: NewModal(ModalConfig{
			Title:    "Save Filter",
			Width:    50,
			Height:   8,
			Backdrop: true,
		}),
		currentQuery: query,
	}
	qsd.setup()
	return qsd
}

func (qsd *QuickSaveDialog) setup() {
	content := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]Query:[-] [%s]%s[-]", TagFgDim(), TagFg(), truncateQuery(qsd.currentQuery, 40)))
	content.SetBackgroundColor(ColorBg())
	qsd.SetContent(content)
	qsd.SetHints([]KeyHint{
		{Key: "Enter", Description: "Save"},
		{Key: "Tab", Description: "Toggle Default"},
		{Key: "Esc", Description: "Cancel"},
	})
}

// SetOnSave sets the save callback.
func (qsd *QuickSaveDialog) SetOnSave(fn func(name string, isDefault bool)) {
	qsd.onSave = fn
}

// SetOnCancel sets the cancel callback.
func (qsd *QuickSaveDialog) SetOnCancel(fn func()) {
	qsd.onCancel = fn
	qsd.Modal.SetOnClose(fn)
}

// Draw renders the quick save dialog.
func (qsd *QuickSaveDialog) Draw(screen tcell.Screen) {
	qsd.Modal.Draw(screen)

	x, y, width, height := qsd.GetInnerRect()
	if width <= 0 || height < 4 {
		return
	}

	inputStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBgLight())
	labelStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	checkStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())

	// Name input (in the middle of the modal)
	labelY := y + height/2
	label := "Name: "
	for li, r := range []rune(label) {
		if x+4+li < x+width-1 {
			screen.SetContent(x+4+li, labelY, r, nil, labelStyle)
		}
	}

	inputX := x + 4 + len(label)
	inputWidth := width - len(label) - 10
	for i := 0; i < inputWidth; i++ {
		ch := ' '
		if i < len(qsd.nameInput) {
			ch = rune(qsd.nameInput[i])
		}
		if inputX+i < x+width-2 {
			screen.SetContent(inputX+i, labelY, ch, nil, inputStyle)
		}
	}

	// Cursor
	cursorX := inputX + qsd.cursorPos
	if cursorX < inputX+inputWidth && cursorX < x+width-2 {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorFg())
		ch := ' '
		if qsd.cursorPos < len(qsd.nameInput) {
			ch = rune(qsd.nameInput[qsd.cursorPos])
		}
		screen.SetContent(cursorX, labelY, ch, nil, cursorStyle)
	}

	// Default checkbox
	checkY := labelY + 1
	checkbox := "[ ] Set as default"
	if qsd.setDefault {
		checkbox = "[" + IconCompleted + "] Set as default"
	}
	for ci, r := range []rune(checkbox) {
		if x+4+ci < x+width-1 {
			screen.SetContent(x+4+ci, checkY, r, nil, checkStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (qsd *QuickSaveDialog) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return qsd.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if qsd.nameInput != "" && qsd.onSave != nil {
				qsd.onSave(qsd.nameInput, qsd.setDefault)
			}
		case tcell.KeyEscape:
			if qsd.onCancel != nil {
				qsd.onCancel()
			}
		case tcell.KeyTab:
			qsd.setDefault = !qsd.setDefault
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if qsd.cursorPos > 0 {
				qsd.nameInput = qsd.nameInput[:qsd.cursorPos-1] + qsd.nameInput[qsd.cursorPos:]
				qsd.cursorPos--
			}
		case tcell.KeyDelete:
			if qsd.cursorPos < len(qsd.nameInput) {
				qsd.nameInput = qsd.nameInput[:qsd.cursorPos] + qsd.nameInput[qsd.cursorPos+1:]
			}
		case tcell.KeyLeft:
			if qsd.cursorPos > 0 {
				qsd.cursorPos--
			}
		case tcell.KeyRight:
			if qsd.cursorPos < len(qsd.nameInput) {
				qsd.cursorPos++
			}
		case tcell.KeyRune:
			r := event.Rune()
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
				r == ' ' || r == '-' || r == '_' {
				qsd.nameInput = qsd.nameInput[:qsd.cursorPos] + string(r) + qsd.nameInput[qsd.cursorPos:]
				qsd.cursorPos++
			}
		}
	})
}

// Focus focuses the dialog.
func (qsd *QuickSaveDialog) Focus(delegate func(p tview.Primitive)) {
	qsd.Flex.Focus(delegate)
}

func truncateQuery(q string, max int) string {
	if len(q) <= max {
		return q
	}
	return q[:max-3] + "..."
}
