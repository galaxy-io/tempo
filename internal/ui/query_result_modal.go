package ui

import (
	"fmt"
	"strings"

	"github.com/atterpac/loom/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// QueryResultModal displays the result of a workflow query.
type QueryResultModal struct {
	*Modal
	textView  *tview.TextView
	nav       *TextViewNavigator
	queryType string
	result    string
	errorMsg  string
}

// NewQueryResultModal creates a new query result modal.
func NewQueryResultModal() *QueryResultModal {
	qm := &QueryResultModal{
		Modal: NewModal(ModalConfig{
			Title:     "Query Result",
			Width:     70,
			Height:    20,
			MinHeight: 10,
			MaxHeight: 30,
			Backdrop:  true,
		}),
		textView: tview.NewTextView(),
	}
	qm.setup()
	return qm
}

// SetResult sets the query result to display.
func (qm *QueryResultModal) SetResult(queryType string, result string) *QueryResultModal {
	qm.queryType = queryType
	qm.result = result
	qm.errorMsg = ""
	qm.rebuildContent()
	return qm
}

// SetError sets an error message to display.
func (qm *QueryResultModal) SetError(queryType string, err string) *QueryResultModal {
	qm.queryType = queryType
	qm.result = ""
	qm.errorMsg = err
	qm.rebuildContent()
	return qm
}

// SetOnClose sets the callback when the modal is closed.
func (qm *QueryResultModal) SetOnClose(fn func()) *QueryResultModal {
	qm.Modal.SetOnClose(fn)
	return qm
}

func (qm *QueryResultModal) setup() {
	qm.textView.SetDynamicColors(true)
	qm.textView.SetBackgroundColor(ColorBg())
	qm.textView.SetScrollable(true)
	qm.textView.SetWordWrap(true)

	qm.nav = NewTextViewNavigator(qm.textView)

	qm.rebuildContent()

	qm.SetContent(qm.textView)
	qm.SetHints([]KeyHint{
		{Key: "j/k", Description: "Scroll"},
		{Key: "y", Description: "Copy result"},
		{Key: "Esc", Description: "Close"},
	})

	// Register for theme changes
	OnThemeChange(func(_ *config.ParsedTheme) {
		qm.textView.SetBackgroundColor(ColorBg())
		qm.rebuildContent()
	})
}

func (qm *QueryResultModal) rebuildContent() {
	var sb strings.Builder

	// Title with query type
	title := "Query Result"
	if qm.queryType != "" {
		title = fmt.Sprintf("Query: %s", qm.queryType)
	}
	qm.SetTitle(title)

	sb.WriteString("\n")

	if qm.errorMsg != "" {
		// Display error
		sb.WriteString(fmt.Sprintf("[%s::b]Error:[-:-:-]\n\n", TagFailed()))
		sb.WriteString(fmt.Sprintf("[%s]%s[-]\n", TagFg(), qm.errorMsg))
	} else if qm.result != "" {
		// Display result
		sb.WriteString(fmt.Sprintf("[%s::b]Result:[-:-:-]\n\n", TagAccent()))
		// Format the result with proper indentation
		lines := strings.Split(qm.result, "\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("[%s]%s[-]\n", TagFg(), line))
		}
	} else {
		sb.WriteString(fmt.Sprintf("[%s]No result[-]\n", TagFgDim()))
	}

	qm.textView.SetText(sb.String())

	// Adjust height based on content
	lineCount := strings.Count(sb.String(), "\n")
	height := min(lineCount+4, 25)
	if height < 10 {
		height = 10
	}
	qm.SetSize(70, height)
}

// GetResult returns the current result text for copying.
func (qm *QueryResultModal) GetResult() string {
	return qm.result
}

// InputHandler handles keyboard input.
func (qm *QueryResultModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return qm.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyEscape:
			qm.Close()
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				qm.Close()
			case 'j':
				qm.nav.MoveDown()
			case 'k':
				qm.nav.MoveUp()
			case 'y':
				// Copy result to clipboard
				if qm.result != "" {
					CopyToClipboard(qm.result)
				}
			}
		case tcell.KeyDown:
			qm.nav.MoveDown()
		case tcell.KeyUp:
			qm.nav.MoveUp()
		}
	})
}

// Focus delegates focus to the text view.
func (qm *QueryResultModal) Focus(delegate func(p tview.Primitive)) {
	delegate(qm.textView)
}
