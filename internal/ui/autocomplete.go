package ui

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ResolveTimePlaceholders replaces ${TIME:duration} placeholders with actual ISO timestamps.
func ResolveTimePlaceholders(query string) string {
	// Pattern: ${TIME:1h}, ${TIME:24h}, ${TIME:7d}, etc.
	re := regexp.MustCompile(`\$\{TIME:(\d+)([mhdw])\}`)

	return re.ReplaceAllStringFunc(query, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) != 3 {
			return match
		}

		value, _ := strconv.Atoi(matches[1])
		unit := matches[2]

		var duration time.Duration
		switch unit {
		case "m":
			duration = time.Duration(value) * time.Minute
		case "h":
			duration = time.Duration(value) * time.Hour
		case "d":
			duration = time.Duration(value) * 24 * time.Hour
		case "w":
			duration = time.Duration(value) * 7 * 24 * time.Hour
		default:
			return match
		}

		// Calculate the timestamp and format as ISO 8601
		t := time.Now().UTC().Add(-duration)
		return t.Format(time.RFC3339)
	})
}

// Suggestion represents an autocomplete suggestion.
type Suggestion struct {
	Text        string // Display text
	InsertText  string // Text to insert when selected
	Description string // Optional description
	Category    string // Category for grouping (e.g., "Field", "Operator", "Value")
}

// QueryTemplate represents a pre-defined query template.
type QueryTemplate struct {
	Name        string
	Description string
	Query       string
}

// TemporalVisibilityFields contains all Temporal visibility query fields.
var TemporalVisibilityFields = []Suggestion{
	{Text: "WorkflowId", InsertText: "WorkflowId", Description: "Workflow identifier", Category: "Field"},
	{Text: "WorkflowType", InsertText: "WorkflowType", Description: "Workflow type name", Category: "Field"},
	{Text: "ExecutionStatus", InsertText: "ExecutionStatus", Description: "Workflow status", Category: "Field"},
	{Text: "StartTime", InsertText: "StartTime", Description: "When workflow started", Category: "Field"},
	{Text: "CloseTime", InsertText: "CloseTime", Description: "When workflow closed", Category: "Field"},
	{Text: "ExecutionDuration", InsertText: "ExecutionDuration", Description: "Workflow duration", Category: "Field"},
	{Text: "TaskQueue", InsertText: "TaskQueue", Description: "Task queue name", Category: "Field"},
	{Text: "RunId", InsertText: "RunId", Description: "Run identifier", Category: "Field"},
}

// TemporalOperators contains visibility query operators.
var TemporalOperators = []Suggestion{
	{Text: "=", InsertText: "=", Description: "Equals", Category: "Operator"},
	{Text: "!=", InsertText: "!=", Description: "Not equals", Category: "Operator"},
	{Text: ">", InsertText: ">", Description: "Greater than", Category: "Operator"},
	{Text: ">=", InsertText: ">=", Description: "Greater or equal", Category: "Operator"},
	{Text: "<", InsertText: "<", Description: "Less than", Category: "Operator"},
	{Text: "<=", InsertText: "<=", Description: "Less or equal", Category: "Operator"},
	{Text: "BETWEEN", InsertText: "BETWEEN", Description: "Between two values", Category: "Operator"},
	{Text: "AND", InsertText: "AND", Description: "Logical AND", Category: "Operator"},
	{Text: "OR", InsertText: "OR", Description: "Logical OR", Category: "Operator"},
	{Text: "ORDER BY", InsertText: "ORDER BY", Description: "Sort results", Category: "Operator"},
}

// TemporalStatusValues contains possible ExecutionStatus values.
var TemporalStatusValues = []Suggestion{
	{Text: "Running", InsertText: "'Running'", Description: "Currently executing", Category: "Value"},
	{Text: "Completed", InsertText: "'Completed'", Description: "Finished successfully", Category: "Value"},
	{Text: "Failed", InsertText: "'Failed'", Description: "Execution failed", Category: "Value"},
	{Text: "Canceled", InsertText: "'Canceled'", Description: "Was canceled", Category: "Value"},
	{Text: "Terminated", InsertText: "'Terminated'", Description: "Was terminated", Category: "Value"},
	{Text: "TimedOut", InsertText: "'TimedOut'", Description: "Execution timed out", Category: "Value"},
	{Text: "ContinuedAsNew", InsertText: "'ContinuedAsNew'", Description: "Continued as new run", Category: "Value"},
}

// TemporalTimeExpressions contains common time expressions.
// Note: These are placeholders - actual timestamps will be calculated at query time.
var TemporalTimeExpressions = []Suggestion{
	{Text: "1 hour ago", InsertText: "${TIME:1h}", Description: "1 hour ago", Category: "Time"},
	{Text: "24 hours ago", InsertText: "${TIME:24h}", Description: "24 hours ago", Category: "Time"},
	{Text: "7 days ago", InsertText: "${TIME:7d}", Description: "7 days ago", Category: "Time"},
	{Text: "30 days ago", InsertText: "${TIME:30d}", Description: "30 days ago", Category: "Time"},
}

// DefaultQueryTemplates contains pre-defined query templates.
// Time placeholders like ${TIME:24h} will be resolved at query time.
var DefaultQueryTemplates = []QueryTemplate{
	{
		Name:        "Running",
		Description: "All running workflows",
		Query:       "ExecutionStatus = 'Running'",
	},
	{
		Name:        "Failed (24h)",
		Description: "Failed workflows in last 24 hours",
		Query:       "ExecutionStatus = 'Failed' AND CloseTime > '${TIME:24h}'",
	},
	{
		Name:        "Timed Out",
		Description: "All timed out workflows",
		Query:       "ExecutionStatus = 'TimedOut'",
	},
	{
		Name:        "Long Running",
		Description: "Running workflows started over 1 hour ago",
		Query:       "ExecutionStatus = 'Running' AND StartTime < '${TIME:1h}'",
	},
	{
		Name:        "Recently Completed",
		Description: "Completed in last hour",
		Query:       "ExecutionStatus = 'Completed' AND CloseTime > '${TIME:1h}'",
	},
	{
		Name:        "By Type",
		Description: "Filter by workflow type",
		Query:       "WorkflowType = '${type}'",
	},
}

// AutocompleteInput is an input field with autocomplete support.
type AutocompleteInput struct {
	*tview.Box
	text           string
	cursorPos      int
	suggestions    []Suggestion
	filteredSuggs  []Suggestion
	selectedIndex  int
	showSuggestion bool
	maxSuggestions int

	// Callbacks
	onSubmit func(text string)
	onCancel func()
	onChange func(text string)
	onSelect func(suggestion Suggestion)

	// Suggestion providers
	suggestionFn func(text string, cursorPos int) []Suggestion

	// History navigation
	historyFn func(direction int) string // -1 = previous, +1 = next
}

// NewAutocompleteInput creates a new autocomplete input field.
func NewAutocompleteInput() *AutocompleteInput {
	ai := &AutocompleteInput{
		Box:            tview.NewBox(),
		maxSuggestions: 8,
		suggestionFn:   DefaultVisibilitySuggestions,
	}
	ai.SetBackgroundColor(ColorBg())

	OnThemeChange(func(_ *config.ParsedTheme) {
		ai.SetBackgroundColor(ColorBg())
	})

	return ai
}

// SetText sets the input text.
func (ai *AutocompleteInput) SetText(text string) {
	ai.text = text
	ai.cursorPos = len(text)
	ai.updateSuggestions()
	if ai.onChange != nil {
		ai.onChange(text)
	}
}

// GetText returns the current input text.
func (ai *AutocompleteInput) GetText() string {
	return ai.text
}

// SetOnSubmit sets the submit callback.
func (ai *AutocompleteInput) SetOnSubmit(fn func(text string)) {
	ai.onSubmit = fn
}

// SetOnCancel sets the cancel callback.
func (ai *AutocompleteInput) SetOnCancel(fn func()) {
	ai.onCancel = fn
}

// SetOnChange sets the change callback.
func (ai *AutocompleteInput) SetOnChange(fn func(text string)) {
	ai.onChange = fn
}

// SetOnSelect sets the selection callback for when a suggestion is chosen.
func (ai *AutocompleteInput) SetOnSelect(fn func(suggestion Suggestion)) {
	ai.onSelect = fn
}

// SetSuggestionProvider sets a custom function to provide suggestions.
func (ai *AutocompleteInput) SetSuggestionProvider(fn func(text string, cursorPos int) []Suggestion) {
	ai.suggestionFn = fn
}

// SetHistoryProvider sets a function to provide history entries.
// The function receives direction (-1 for previous, +1 for next) and returns the query.
func (ai *AutocompleteInput) SetHistoryProvider(fn func(direction int) string) {
	ai.historyFn = fn
}

// updateSuggestions updates the filtered suggestions based on current input.
func (ai *AutocompleteInput) updateSuggestions() {
	if ai.suggestionFn != nil {
		ai.filteredSuggs = ai.suggestionFn(ai.text, ai.cursorPos)
	} else {
		ai.filteredSuggs = nil
	}
	ai.selectedIndex = 0
	ai.showSuggestion = len(ai.filteredSuggs) > 0
}

// DefaultVisibilitySuggestions provides context-aware suggestions for visibility queries.
func DefaultVisibilitySuggestions(text string, cursorPos int) []Suggestion {
	// Get the word being typed at cursor position
	textUpToCursor := text
	if cursorPos < len(text) {
		textUpToCursor = text[:cursorPos]
	}

	// Find the current token being typed
	lastSpace := strings.LastIndexAny(textUpToCursor, " ()")
	currentToken := textUpToCursor
	if lastSpace >= 0 {
		currentToken = textUpToCursor[lastSpace+1:]
	}
	currentTokenLower := strings.ToLower(currentToken)

	// Determine context
	trimmed := strings.TrimSpace(textUpToCursor)
	words := strings.Fields(trimmed)

	var suggestions []Suggestion

	// Context-aware suggestions
	if len(words) == 0 || (len(words) == 1 && !strings.ContainsAny(trimmed, "=<>!")) {
		// Beginning of query - suggest fields
		for _, s := range TemporalVisibilityFields {
			if currentToken == "" || strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
				suggestions = append(suggestions, s)
			}
		}
	} else if len(words) >= 1 {
		lastWord := words[len(words)-1]
		lastWordLower := strings.ToLower(lastWord)

		// Check if we just typed a field name (need operator)
		isField := false
		for _, f := range TemporalVisibilityFields {
			if strings.ToLower(f.Text) == lastWordLower && !strings.ContainsAny(trimmed, "=<>!") {
				isField = true
				break
			}
		}

		if isField || strings.HasSuffix(trimmed, " ") && containsField(words) && !containsOperator(words) {
			// After a field - suggest operators
			for _, s := range TemporalOperators {
				if s.Text == "AND" || s.Text == "OR" || s.Text == "ORDER BY" {
					continue // Skip logical operators here
				}
				if currentToken == "" || strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
					suggestions = append(suggestions, s)
				}
			}
		} else if strings.HasSuffix(lastWord, "=") || lastWord == ">" || lastWord == "<" ||
			lastWord == ">=" || lastWord == "<=" || lastWord == "!=" {
			// After an operator - suggest values based on field

			// Check if it's ExecutionStatus
			if len(words) >= 2 && strings.ToLower(words[len(words)-2]) == "executionstatus" {
				for _, s := range TemporalStatusValues {
					suggestions = append(suggestions, s)
				}
			} else if len(words) >= 2 && (strings.ToLower(words[len(words)-2]) == "starttime" ||
				strings.ToLower(words[len(words)-2]) == "closetime") {
				// Time field - suggest time expressions
				for _, s := range TemporalTimeExpressions {
					suggestions = append(suggestions, s)
				}
			}
		} else if (lastWord == "AND" || lastWord == "OR") ||
			(strings.HasSuffix(trimmed, " ") && hasCompleteCondition(trimmed)) {
			// After a complete condition - suggest AND/OR or new field
			for _, s := range []Suggestion{
				{Text: "AND", InsertText: "AND", Description: "Logical AND", Category: "Operator"},
				{Text: "OR", InsertText: "OR", Description: "Logical OR", Category: "Operator"},
			} {
				if currentToken == "" || strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
					suggestions = append(suggestions, s)
				}
			}
			// Also suggest fields after AND/OR
			if lastWord == "AND" || lastWord == "OR" {
				for _, s := range TemporalVisibilityFields {
					if currentToken == "" || strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
						suggestions = append(suggestions, s)
					}
				}
			}
		} else {
			// Default: filter by what's being typed
			for _, s := range TemporalVisibilityFields {
				if strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
					suggestions = append(suggestions, s)
				}
			}
			for _, s := range TemporalOperators {
				if strings.HasPrefix(strings.ToLower(s.Text), currentTokenLower) {
					suggestions = append(suggestions, s)
				}
			}
		}
	}

	// Limit suggestions
	if len(suggestions) > 8 {
		suggestions = suggestions[:8]
	}

	return suggestions
}

// Helper functions for query parsing
func containsField(words []string) bool {
	for _, w := range words {
		wLower := strings.ToLower(w)
		for _, f := range TemporalVisibilityFields {
			if strings.ToLower(f.Text) == wLower {
				return true
			}
		}
	}
	return false
}

func containsOperator(words []string) bool {
	for _, w := range words {
		if w == "=" || w == "!=" || w == ">" || w == "<" || w == ">=" || w == "<=" ||
			strings.HasSuffix(w, "=") {
			return true
		}
	}
	return false
}

func hasCompleteCondition(text string) bool {
	// Simple heuristic: contains a quoted value or ends with a time expression
	return strings.Contains(text, "'") && strings.Count(text, "'")%2 == 0 ||
		strings.HasSuffix(text, ")") ||
		strings.HasSuffix(text, "h") ||
		strings.HasSuffix(text, "d")
}

// acceptSuggestion inserts the selected suggestion.
func (ai *AutocompleteInput) acceptSuggestion() {
	if ai.selectedIndex >= 0 && ai.selectedIndex < len(ai.filteredSuggs) {
		sugg := ai.filteredSuggs[ai.selectedIndex]

		// Find where to insert (replace current token)
		textUpToCursor := ai.text
		if ai.cursorPos < len(ai.text) {
			textUpToCursor = ai.text[:ai.cursorPos]
		}

		lastSpace := strings.LastIndexAny(textUpToCursor, " ()")
		prefix := ""
		if lastSpace >= 0 {
			prefix = textUpToCursor[:lastSpace+1]
		}

		suffix := ""
		if ai.cursorPos < len(ai.text) {
			suffix = ai.text[ai.cursorPos:]
		}

		// Build new text
		ai.text = prefix + sugg.InsertText + suffix
		ai.cursorPos = len(prefix) + len(sugg.InsertText)

		ai.showSuggestion = false
		ai.filteredSuggs = nil

		if ai.onSelect != nil {
			ai.onSelect(sugg)
		}
		if ai.onChange != nil {
			ai.onChange(ai.text)
		}
	}
}

// Draw renders the autocomplete input.
func (ai *AutocompleteInput) Draw(screen tcell.Screen) {
	ai.Box.DrawForSubclass(screen, ai)

	x, y, width, height := ai.GetInnerRect()
	if width <= 0 || height < 1 {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(ColorPanelBorder()).Background(ColorBg())
	titleStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg()).Bold(true)
	textStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())
	promptStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg())
	hintStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	suggestionStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBgLight())
	selectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())
	categoryStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBgLight())

	// Calculate content area (inside border)
	// We need at least 3 rows for input, and more for suggestions
	inputRows := 3
	suggestionRows := 0
	if ai.showSuggestion && len(ai.filteredSuggs) > 0 {
		suggestionRows = min(len(ai.filteredSuggs), ai.maxSuggestions) + 2 // +2 for border
	}

	totalHeight := inputRows + suggestionRows
	if totalHeight > height {
		totalHeight = height
	}

	// Draw input box border
	screen.SetContent(x, y, '╭', nil, borderStyle)
	screen.SetContent(x+width-1, y, '╮', nil, borderStyle)
	screen.SetContent(x, y+2, '╰', nil, borderStyle)
	screen.SetContent(x+width-1, y+2, '╯', nil, borderStyle)

	for i := x + 1; i < x+width-1; i++ {
		screen.SetContent(i, y, '─', nil, borderStyle)
		screen.SetContent(i, y+2, '─', nil, borderStyle)
	}
	screen.SetContent(x, y+1, '│', nil, borderStyle)
	screen.SetContent(x+width-1, y+1, '│', nil, borderStyle)

	// Draw title
	title := " Query "
	titleRunes := []rune(title)
	titleX := x + 2
	for i, r := range titleRunes {
		if titleX+i >= x+width-1 {
			break
		}
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Draw prompt and input
	contentY := y + 1
	contentX := x + 2

	prompt := IconArrowRight + " /"
	for _, r := range []rune(prompt) {
		if contentX >= x+width-2 {
			break
		}
		screen.SetContent(contentX, contentY, r, nil, promptStyle)
		contentX++
	}

	// Draw input text
	inputRunes := []rune(ai.text)
	for i, r := range inputRunes {
		if contentX+i >= x+width-2 {
			break
		}
		screen.SetContent(contentX+i, contentY, r, nil, textStyle)
	}

	// Draw cursor
	cursorX := contentX + ai.cursorPos
	if cursorX < x+width-2 {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorFg())
		if ai.cursorPos < len(ai.text) {
			r := inputRunes[ai.cursorPos]
			screen.SetContent(cursorX, contentY, r, nil, cursorStyle)
		} else {
			screen.SetContent(cursorX, contentY, ' ', nil, cursorStyle)
		}
	}

	// Draw hint on right side
	hint := "[Tab] Complete  [↑↓] Navigate"
	hintX := x + width - len(hint) - 3
	if hintX > contentX+len(ai.text)+2 {
		for i, r := range []rune(hint) {
			screen.SetContent(hintX+i, contentY, r, nil, hintStyle)
		}
	}

	// Draw suggestions dropdown below
	if ai.showSuggestion && len(ai.filteredSuggs) > 0 && height > 3 {
		suggY := y + 3
		suggWidth := width - 4
		if suggWidth < 30 {
			suggWidth = min(width-2, 40)
		}
		suggX := x + 2

		// Draw suggestion box border
		numSuggs := min(len(ai.filteredSuggs), ai.maxSuggestions)
		if suggY+numSuggs+1 <= y+height {
			// Top border
			screen.SetContent(suggX, suggY, '┌', nil, borderStyle)
			screen.SetContent(suggX+suggWidth-1, suggY, '┐', nil, borderStyle)
			for i := suggX + 1; i < suggX+suggWidth-1; i++ {
				screen.SetContent(i, suggY, '─', nil, borderStyle)
			}

			// Suggestions
			for i, sugg := range ai.filteredSuggs {
				if i >= numSuggs {
					break
				}
				rowY := suggY + 1 + i
				if rowY >= y+height-1 {
					break
				}

				// Left border
				screen.SetContent(suggX, rowY, '│', nil, borderStyle)

				// Content
				style := suggestionStyle
				if i == ai.selectedIndex {
					style = selectedStyle
				}

				// Clear row
				for cx := suggX + 1; cx < suggX+suggWidth-1; cx++ {
					screen.SetContent(cx, rowY, ' ', nil, style)
				}

				// Draw category tag
				catStyle := categoryStyle
				if i == ai.selectedIndex {
					catStyle = selectedStyle
				}
				catTag := "[" + sugg.Category + "]"
				for ci, r := range []rune(catTag) {
					if suggX+2+ci < suggX+suggWidth-1 {
						screen.SetContent(suggX+2+ci, rowY, r, nil, catStyle)
					}
				}

				// Draw suggestion text
				textOffset := suggX + 2 + len(catTag) + 1
				for ti, r := range []rune(sugg.Text) {
					if textOffset+ti < suggX+suggWidth-1 {
						screen.SetContent(textOffset+ti, rowY, r, nil, style)
					}
				}

				// Draw description if space permits
				descOffset := textOffset + len(sugg.Text) + 2
				if descOffset < suggX+suggWidth-10 && sugg.Description != "" {
					desc := "- " + sugg.Description
					descStyle := hintStyle
					if i == ai.selectedIndex {
						descStyle = selectedStyle
					}
					for di, r := range []rune(desc) {
						if descOffset+di < suggX+suggWidth-2 {
							screen.SetContent(descOffset+di, rowY, r, nil, descStyle)
						}
					}
				}

				// Right border
				screen.SetContent(suggX+suggWidth-1, rowY, '│', nil, borderStyle)
			}

			// Bottom border
			bottomY := suggY + 1 + numSuggs
			if bottomY < y+height {
				screen.SetContent(suggX, bottomY, '└', nil, borderStyle)
				screen.SetContent(suggX+suggWidth-1, bottomY, '┘', nil, borderStyle)
				for i := suggX + 1; i < suggX+suggWidth-1; i++ {
					screen.SetContent(i, bottomY, '─', nil, borderStyle)
				}
			}
		}
	}
}

// InputHandler handles keyboard input.
func (ai *AutocompleteInput) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return ai.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if ai.showSuggestion && ai.selectedIndex >= 0 && ai.selectedIndex < len(ai.filteredSuggs) {
				// Accept suggestion on Enter if dropdown is visible
				ai.acceptSuggestion()
			} else if ai.onSubmit != nil {
				ai.onSubmit(ai.text)
			}
		case tcell.KeyEscape:
			if ai.showSuggestion {
				// Close suggestions first
				ai.showSuggestion = false
			} else if ai.onCancel != nil {
				ai.onCancel()
			}
		case tcell.KeyTab:
			// Accept current suggestion
			if ai.showSuggestion && len(ai.filteredSuggs) > 0 {
				ai.acceptSuggestion()
				ai.updateSuggestions() // Show new suggestions
			}
		case tcell.KeyUp:
			if ai.showSuggestion && len(ai.filteredSuggs) > 0 {
				ai.selectedIndex--
				if ai.selectedIndex < 0 {
					ai.selectedIndex = len(ai.filteredSuggs) - 1
				}
			} else if ai.historyFn != nil {
				// Navigate to previous history entry
				if historyQuery := ai.historyFn(-1); historyQuery != "" {
					ai.text = historyQuery
					ai.cursorPos = len(historyQuery)
					ai.updateSuggestions()
					if ai.onChange != nil {
						ai.onChange(ai.text)
					}
				}
			}
		case tcell.KeyDown:
			if ai.showSuggestion && len(ai.filteredSuggs) > 0 {
				ai.selectedIndex++
				if ai.selectedIndex >= len(ai.filteredSuggs) {
					ai.selectedIndex = 0
				}
			} else if ai.historyFn != nil {
				// Navigate to next history entry
				historyQuery := ai.historyFn(+1)
				ai.text = historyQuery
				ai.cursorPos = len(historyQuery)
				ai.updateSuggestions()
				if ai.onChange != nil {
					ai.onChange(ai.text)
				}
			}
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if ai.cursorPos > 0 {
				runes := []rune(ai.text)
				ai.text = string(runes[:ai.cursorPos-1]) + string(runes[ai.cursorPos:])
				ai.cursorPos--
				ai.updateSuggestions()
				if ai.onChange != nil {
					ai.onChange(ai.text)
				}
			}
		case tcell.KeyDelete:
			runes := []rune(ai.text)
			if ai.cursorPos < len(runes) {
				ai.text = string(runes[:ai.cursorPos]) + string(runes[ai.cursorPos+1:])
				ai.updateSuggestions()
				if ai.onChange != nil {
					ai.onChange(ai.text)
				}
			}
		case tcell.KeyLeft:
			if ai.cursorPos > 0 {
				ai.cursorPos--
			}
		case tcell.KeyRight:
			if ai.cursorPos < len(ai.text) {
				ai.cursorPos++
			}
		case tcell.KeyHome, tcell.KeyCtrlA:
			ai.cursorPos = 0
		case tcell.KeyEnd, tcell.KeyCtrlE:
			ai.cursorPos = len(ai.text)
		case tcell.KeyCtrlU:
			// Clear line
			ai.text = ""
			ai.cursorPos = 0
			ai.updateSuggestions()
			if ai.onChange != nil {
				ai.onChange(ai.text)
			}
		case tcell.KeyRune:
			r := event.Rune()
			runes := []rune(ai.text)
			ai.text = string(runes[:ai.cursorPos]) + string(r) + string(runes[ai.cursorPos:])
			ai.cursorPos++
			ai.updateSuggestions()
			if ai.onChange != nil {
				ai.onChange(ai.text)
			}
		}
	})
}

// Focus sets focus to this input.
func (ai *AutocompleteInput) Focus(delegate func(p tview.Primitive)) {
	ai.Box.Focus(delegate)
}

// HasFocus returns whether this input has focus.
func (ai *AutocompleteInput) HasFocus() bool {
	return ai.Box.HasFocus()
}

// GetSuggestionHeight returns the height needed to display suggestions.
func (ai *AutocompleteInput) GetSuggestionHeight() int {
	if !ai.showSuggestion || len(ai.filteredSuggs) == 0 {
		return 3 // Just the input box
	}
	return 3 + min(len(ai.filteredSuggs), ai.maxSuggestions) + 2
}

// QueryTemplateSelector provides a quick-select for query templates.
type QueryTemplateSelector struct {
	*tview.Box
	templates     []QueryTemplate
	selectedIndex int
	onSelect      func(template QueryTemplate)
	onCancel      func()
}

// NewQueryTemplateSelector creates a new template selector.
func NewQueryTemplateSelector(templates []QueryTemplate) *QueryTemplateSelector {
	qts := &QueryTemplateSelector{
		Box:       tview.NewBox(),
		templates: templates,
	}
	qts.SetBackgroundColor(ColorBg())

	OnThemeChange(func(_ *config.ParsedTheme) {
		qts.SetBackgroundColor(ColorBg())
	})

	return qts
}

// SetOnSelect sets the selection callback.
func (qts *QueryTemplateSelector) SetOnSelect(fn func(template QueryTemplate)) {
	qts.onSelect = fn
}

// SetOnCancel sets the cancel callback.
func (qts *QueryTemplateSelector) SetOnCancel(fn func()) {
	qts.onCancel = fn
}

// Draw renders the template selector.
func (qts *QueryTemplateSelector) Draw(screen tcell.Screen) {
	qts.Box.DrawForSubclass(screen, qts)

	x, y, width, height := qts.GetInnerRect()
	if width <= 0 || height < 3 {
		return
	}

	borderStyle := tcell.StyleDefault.Foreground(ColorPanelBorder()).Background(ColorBg())
	titleStyle := tcell.StyleDefault.Foreground(ColorAccent()).Background(ColorBg()).Bold(true)
	itemStyle := tcell.StyleDefault.Foreground(ColorFg()).Background(ColorBg())
	selectedStyle := tcell.StyleDefault.Foreground(ColorBg()).Background(ColorAccent())
	descStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())
	hintStyle := tcell.StyleDefault.Foreground(ColorFgDim()).Background(ColorBg())

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
	title := " Query Templates "
	titleX := x + (width-len(title))/2
	for i, r := range []rune(title) {
		screen.SetContent(titleX+i, y, r, nil, titleStyle)
	}

	// Draw templates
	for i, tmpl := range qts.templates {
		rowY := y + 1 + i
		if rowY >= y+height-2 {
			break
		}

		style := itemStyle
		dStyle := descStyle
		if i == qts.selectedIndex {
			style = selectedStyle
			dStyle = selectedStyle
		}

		// Clear row
		for cx := x + 1; cx < x+width-1; cx++ {
			screen.SetContent(cx, rowY, ' ', nil, style)
		}

		// Name
		name := "  " + tmpl.Name
		for ni, r := range []rune(name) {
			if x+1+ni < x+width-1 {
				screen.SetContent(x+1+ni, rowY, r, nil, style)
			}
		}

		// Description
		descX := x + 20
		if descX < x+width-20 {
			desc := "- " + tmpl.Description
			for di, r := range []rune(desc) {
				if descX+di < x+width-2 {
					screen.SetContent(descX+di, rowY, r, nil, dStyle)
				}
			}
		}
	}

	// Hints at bottom
	hintY := y + height - 1
	hint := " [↑↓] Select  [Enter] Apply  [Esc] Cancel "
	hintX := x + (width-len(hint))/2
	for i, r := range []rune(hint) {
		if hintX+i > x && hintX+i < x+width-1 {
			screen.SetContent(hintX+i, hintY, r, nil, hintStyle)
		}
	}
}

// InputHandler handles keyboard input.
func (qts *QueryTemplateSelector) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return qts.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEnter:
			if qts.selectedIndex >= 0 && qts.selectedIndex < len(qts.templates) {
				if qts.onSelect != nil {
					qts.onSelect(qts.templates[qts.selectedIndex])
				}
			}
		case tcell.KeyEscape:
			if qts.onCancel != nil {
				qts.onCancel()
			}
		case tcell.KeyUp:
			qts.selectedIndex--
			if qts.selectedIndex < 0 {
				qts.selectedIndex = len(qts.templates) - 1
			}
		case tcell.KeyDown:
			qts.selectedIndex++
			if qts.selectedIndex >= len(qts.templates) {
				qts.selectedIndex = 0
			}
		case tcell.KeyRune:
			// Quick select by number
			r := event.Rune()
			if r >= '1' && r <= '9' {
				idx := int(r - '1')
				if idx < len(qts.templates) {
					qts.selectedIndex = idx
					if qts.onSelect != nil {
						qts.onSelect(qts.templates[qts.selectedIndex])
					}
				}
			}
		}
	})
}

// Focus sets focus to this selector.
func (qts *QueryTemplateSelector) Focus(delegate func(p tview.Primitive)) {
	qts.Box.Focus(delegate)
}
