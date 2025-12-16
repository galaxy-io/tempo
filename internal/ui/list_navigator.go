package ui

import "github.com/rivo/tview"

// ListNavigator provides a consistent interface for j/k navigation in list-based content.
type ListNavigator interface {
	// MoveUp moves selection up, returns true if moved
	MoveUp() bool
	// MoveDown moves selection down, returns true if moved
	MoveDown() bool
	// GetSelectedIndex returns the current selection index (0-based, excluding headers)
	GetSelectedIndex() int
	// GetItemCount returns the number of selectable items
	GetItemCount() int
}

// TableListNavigator wraps tview.Table for consistent j/k navigation.
type TableListNavigator struct {
	table     *tview.Table
	fixedRows int // Number of header rows to skip (usually 0 or 1)
}

// NewTableListNavigator creates a navigator for a tview.Table.
func NewTableListNavigator(table *tview.Table, fixedRows int) *TableListNavigator {
	return &TableListNavigator{
		table:     table,
		fixedRows: fixedRows,
	}
}

// MoveUp moves selection up one row.
func (n *TableListNavigator) MoveUp() bool {
	row, col := n.table.GetSelection()
	if row > n.fixedRows {
		n.table.Select(row-1, col)
		return true
	}
	return false
}

// MoveDown moves selection down one row.
func (n *TableListNavigator) MoveDown() bool {
	row, col := n.table.GetSelection()
	if row < n.table.GetRowCount()-1 {
		n.table.Select(row+1, col)
		return true
	}
	return false
}

// GetSelectedIndex returns the selected row index (0-based, excluding fixed rows).
func (n *TableListNavigator) GetSelectedIndex() int {
	row, _ := n.table.GetSelection()
	return row - n.fixedRows
}

// GetItemCount returns the number of selectable rows.
func (n *TableListNavigator) GetItemCount() int {
	return n.table.GetRowCount() - n.fixedRows
}

// SelectIndex selects a specific index (0-based, excluding fixed rows).
func (n *TableListNavigator) SelectIndex(index int) {
	n.table.Select(index+n.fixedRows, 0)
}

// TextViewNavigator wraps tview.TextView for scrolling navigation.
type TextViewNavigator struct {
	textView *tview.TextView
}

// NewTextViewNavigator creates a navigator for a tview.TextView.
func NewTextViewNavigator(tv *tview.TextView) *TextViewNavigator {
	return &TextViewNavigator{textView: tv}
}

// MoveUp scrolls up one line.
func (n *TextViewNavigator) MoveUp() bool {
	row, col := n.textView.GetScrollOffset()
	if row > 0 {
		n.textView.ScrollTo(row-1, col)
		return true
	}
	return false
}

// MoveDown scrolls down one line.
func (n *TextViewNavigator) MoveDown() bool {
	row, col := n.textView.GetScrollOffset()
	n.textView.ScrollTo(row+1, col)
	return true
}

// GetSelectedIndex returns the current scroll row.
func (n *TextViewNavigator) GetSelectedIndex() int {
	row, _ := n.textView.GetScrollOffset()
	return row
}

// GetItemCount returns -1 as text views have unknown item counts.
func (n *TextViewNavigator) GetItemCount() int {
	return -1
}

// ScrollTo scrolls to a specific row.
func (n *TextViewNavigator) ScrollTo(row int) {
	n.textView.ScrollTo(row, 0)
}
