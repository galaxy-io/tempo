package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DateRangePreset represents a predefined time range.
type DateRangePreset struct {
	Name        string
	Description string
	Duration    time.Duration
	Query       string // Temporal visibility query fragment
}

// DefaultDatePresets contains common time range presets.
// Note: Query field uses ${TIME:duration} placeholders that will be resolved at query time.
var DefaultDatePresets = []DateRangePreset{
	{Name: "1h", Description: "Last hour", Duration: time.Hour, Query: "StartTime > '${TIME:1h}'"},
	{Name: "24h", Description: "Last 24 hours", Duration: 24 * time.Hour, Query: "StartTime > '${TIME:24h}'"},
	{Name: "7d", Description: "Last 7 days", Duration: 7 * 24 * time.Hour, Query: "StartTime > '${TIME:7d}'"},
	{Name: "30d", Description: "Last 30 days", Duration: 30 * 24 * time.Hour, Query: "StartTime > '${TIME:30d}'"},
	{Name: "All", Description: "All time", Duration: 0, Query: ""},
}

// DateRangeField specifies which time field to filter on.
type DateRangeField int

const (
	DateRangeStartTime DateRangeField = iota
	DateRangeCloseTime
)

// DateRangePicker provides a date range selection UI.
type DateRangePicker struct {
	*tview.Box
	presets       []DateRangePreset
	selectedIndex int
	field         DateRangeField
	customInput   string
	showCustom    bool
	cursorPos     int

	onSelect func(query string) // Returns visibility query fragment
	onCancel func()
}

// NewDateRangePicker creates a new date range picker.
func NewDateRangePicker() *DateRangePicker {
	drp := &DateRangePicker{
		Box:     tview.NewBox(),
		presets: DefaultDatePresets,
		field:   DateRangeStartTime,
	}
	drp.SetBackgroundColor(ColorBg())

	OnThemeChange(func(_ *config.ParsedTheme) {
		drp.SetBackgroundColor(ColorBg())
	})

	return drp
}

// SetField sets which time field to filter on (StartTime or CloseTime).
func (drp *DateRangePicker) SetField(field DateRangeField) {
	drp.field = field
}

// SetOnSelect sets the selection callback.
func (drp *DateRangePicker) SetOnSelect(fn func(query string)) {
	drp.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (drp *DateRangePicker) SetOnCancel(fn func()) {
	drp.onCancel = fn
}

// getFieldName returns the visibility query field name.
func (drp *DateRangePicker) getFieldName() string {
	if drp.field == DateRangeCloseTime {
		return "CloseTime"
	}
	return "StartTime"
}

// parseCustomDuration parses a duration string like "3d", "2w", "4h".
func parseCustomDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Match patterns like: 30m, 4h, 3d, 2w
	re := regexp.MustCompile(`^(\d+)([mhdw])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

// buildQuery builds the visibility query for the current selection.
// Returns a query with ${TIME:...} placeholders that need to be resolved.
func (drp *DateRangePicker) buildQuery() string {
	if drp.showCustom {
		// Parse custom input
		dur, err := parseCustomDuration(drp.customInput)
		if err != nil || dur == 0 {
			return "" // Invalid input or "all time"
		}
		// Convert duration to placeholder format
		field := drp.getFieldName()
		durStr := formatDurationForPlaceholder(dur)
		return fmt.Sprintf("%s > '${TIME:%s}'", field, durStr)
	}

	if drp.selectedIndex >= 0 && drp.selectedIndex < len(drp.presets) {
		preset := drp.presets[drp.selectedIndex]
		if preset.Duration == 0 {
			return "" // All time
		}
		// Replace StartTime with the configured field
		query := preset.Query
		if drp.field == DateRangeCloseTime {
			query = strings.Replace(query, "StartTime", "CloseTime", 1)
		}
		return query
	}
	return ""
}

// formatDurationForPlaceholder converts a duration to placeholder format (e.g., "24h", "7d").
func formatDurationForPlaceholder(d time.Duration) string {
	if d >= 24*time.Hour {
		days := d / (24 * time.Hour)
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		hours := d / time.Hour
		return fmt.Sprintf("%dh", hours)
	}
	minutes := d / time.Minute
	return fmt.Sprintf("%dm", minutes)
}

// Draw renders the date range picker.
func (drp *DateRangePicker) Draw(screen tcell.Screen) {
	drp.Box.DrawForSubclass(screen, drp)

	x, y, width, height := drp.GetInnerRect()
	if width <= 0 || height < 5 {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(ColorPanelBorder()).Background(ColorBg())
	titleStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg()).Bold(true)
	itemStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())
	selectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())
	descStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	hintStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	customStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBgLight())
	customSelectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())

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

	// Title
	fieldName := drp.getFieldName()
	title := fmt.Sprintf(" Date Range (%s) ", fieldName)
	titleX := x + (width-len(title))/2
	for i, r := range []rune(title) {
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Draw presets
	for i, preset := range drp.presets {
		rowY := y + 1 + i
		if rowY >= y+height-3 {
			break
		}

		style := itemStyle
		dStyle := descStyle
		if !drp.showCustom && i == drp.selectedIndex {
			style = selectedStyle
			dStyle = selectedStyle
		}

		// Clear row
		for cx := x + 1; cx < x+width-1; cx++ {
			screen.SetContent(cx, rowY, ' ', nil, style)
		}

		// Draw preset name with marker
		marker := "  "
		if !drp.showCustom && i == drp.selectedIndex {
			marker = IconArrowRight + " "
		}
		text := marker + preset.Name
		for ti, r := range []rune(text) {
			if x+2+ti < x+width-1 {
				screen.SetContent(x+2+ti, rowY, r, nil, style)
			}
		}

		// Draw description
		descX := x + 12
		if descX < x+width-20 {
			desc := "- " + preset.Description
			for di, r := range []rune(desc) {
				if descX+di < x+width-2 {
					screen.SetContent(descX+di, rowY, r, nil, dStyle)
				}
			}
		}
	}

	// Draw custom input section
	customY := y + 1 + len(drp.presets)
	if customY < y+height-2 {
		// Separator line
		screen.SetContent(x, customY, '├', nil, borderStyle)
		screen.SetContent(x+width-1, customY, '┤', nil, borderStyle)
		for i := x + 1; i < x+width-1; i++ {
			screen.SetContent(i, customY, '─', nil, borderStyle)
		}

		// Custom input row
		inputY := customY + 1
		if inputY < y+height-1 {
			style := itemStyle
			if drp.showCustom {
				style = selectedStyle
			}

			// Clear row
			for cx := x + 1; cx < x+width-1; cx++ {
				screen.SetContent(cx, inputY, ' ', nil, style)
			}

			// Draw custom label
			marker := "  "
			if drp.showCustom {
				marker = IconArrowRight + " "
			}
			label := marker + "Custom: "
			for li, r := range []rune(label) {
				if x+2+li < x+width-1 {
					screen.SetContent(x+2+li, inputY, r, nil, style)
				}
			}

			// Draw input field
			inputX := x + 2 + len(label)
			inputWidth := 15
			inputStyle := customStyle
			if drp.showCustom {
				inputStyle = customSelectedStyle
			}

			for i := 0; i < inputWidth; i++ {
				if inputX+i < x+width-2 {
					ch := ' '
					if i < len(drp.customInput) {
						ch = rune(drp.customInput[i])
					}
					screen.SetContent(inputX+i, inputY, ch, nil, inputStyle)
				}
			}

			// Draw cursor when in custom mode
			if drp.showCustom {
				cursorX := inputX + drp.cursorPos
				if cursorX < inputX+inputWidth && cursorX < x+width-2 {
					cursorStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorFg())
					ch := ' '
					if drp.cursorPos < len(drp.customInput) {
						ch = rune(drp.customInput[drp.cursorPos])
					}
					screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
				}
			}

			// Draw hint for custom input
			hintX := inputX + inputWidth + 2
			hint := "(e.g., 3d, 2w, 4h)"
			for hi, r := range []rune(hint) {
				if hintX+hi < x+width-2 {
					screen.SetContent(hintX+hi, inputY, r, nil, hintStyle)
				}
			}
		}
	}

	// Draw bottom hints
	hintY := y + height - 1
	hint := " [↑↓] Select  [Tab] Custom  [Enter] Apply  [Esc] Cancel "
	hintX := x + (width-len(hint))/2
	for i, r := range []rune(hint) {
		if hintX+i > x && hintX+i < x+width-1 {
			screen.SetContent(hintX+i, hintY, r, nil, hintStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (drp *DateRangePicker) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return drp.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if drp.showCustom {
			// Handle custom input mode
			switch event.Key() {
			case tcell.KeyEnter:
				if drp.onSelect != nil {
					drp.onSelect(drp.buildQuery())
				}
			case tcell.KeyEscape:
				if drp.onCancel != nil {
					drp.onCancel()
				}
			case tcell.KeyTab:
				drp.showCustom = false
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if drp.cursorPos > 0 {
					drp.customInput = drp.customInput[:drp.cursorPos-1] + drp.customInput[drp.cursorPos:]
					drp.cursorPos--
				}
			case tcell.KeyDelete:
				if drp.cursorPos < len(drp.customInput) {
					drp.customInput = drp.customInput[:drp.cursorPos] + drp.customInput[drp.cursorPos+1:]
				}
			case tcell.KeyLeft:
				if drp.cursorPos > 0 {
					drp.cursorPos--
				}
			case tcell.KeyRight:
				if drp.cursorPos < len(drp.customInput) {
					drp.cursorPos++
				}
			case tcell.KeyRune:
				r := event.Rune()
				// Only allow valid characters for duration input
				if (r >= '0' && r <= '9') || r == 'm' || r == 'h' || r == 'd' || r == 'w' {
					drp.customInput = drp.customInput[:drp.cursorPos] + string(r) + drp.customInput[drp.cursorPos:]
					drp.cursorPos++
				}
			}
			return
		}

		// Handle preset selection mode
		switch event.Key() {
		case tcell.KeyEnter:
			if drp.onSelect != nil {
				drp.onSelect(drp.buildQuery())
			}
		case tcell.KeyEscape:
			if drp.onCancel != nil {
				drp.onCancel()
			}
		case tcell.KeyTab:
			drp.showCustom = true
		case tcell.KeyUp:
			drp.selectedIndex--
			if drp.selectedIndex < 0 {
				drp.selectedIndex = len(drp.presets) - 1
			}
		case tcell.KeyDown:
			drp.selectedIndex++
			if drp.selectedIndex >= len(drp.presets) {
				drp.selectedIndex = 0
			}
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				drp.selectedIndex++
				if drp.selectedIndex >= len(drp.presets) {
					drp.selectedIndex = 0
				}
			case 'k':
				drp.selectedIndex--
				if drp.selectedIndex < 0 {
					drp.selectedIndex = len(drp.presets) - 1
				}
			case '1', '2', '3', '4', '5':
				// Quick select by number
				idx := int(event.Rune() - '1')
				if idx < len(drp.presets) {
					drp.selectedIndex = idx
					if drp.onSelect != nil {
						drp.onSelect(drp.buildQuery())
					}
				}
			}
		}
	})
}

// Focus sets focus to this picker.
func (drp *DateRangePicker) Focus(delegate func(p tview.Primitive)) {
	drp.Box.Focus(delegate)
}

// GetHeight returns the preferred height for this component.
func (drp *DateRangePicker) GetHeight() int {
	return len(drp.presets) + 5 // presets + border + separator + custom + hints
}
