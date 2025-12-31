package view

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/rivo/tview"
)

func (wl *WorkflowList) showVisibilityQuery() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Visibility Query", theme.IconSearch),
		Width:    70,
		Height:   16,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("query", "Query", wl.visibilityQuery)

	helpText := tview.NewTextView().SetDynamicColors(true)
	helpText.SetBackgroundColor(theme.Bg())
	helpText.SetText(fmt.Sprintf(`[%s]Examples:[-]
  WorkflowType = 'OrderWorkflow'
  ExecutionStatus = 'Running'
  StartTime > '2024-01-01T00:00:00Z'
  WorkflowId STARTS_WITH 'order-'`,
		theme.TagFgDim()))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 3, 0, true).
		AddItem(helpText, 0, 1, false)
	content.SetBackgroundColor(theme.Bg())

	form.SetOnSubmit(func(values map[string]any) {
		query := values["query"].(string)
		wl.closeModal()
		wl.applyVisibilityQuery(query)
	})
	form.SetOnCancel(func() {
		wl.closeModal()
	})

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		query := values["query"].(string)
		wl.closeModal()
		wl.applyVisibilityQuery(query)
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) applyVisibilityQuery(query string) {
	if query != "" && query != wl.visibilityQuery {
		wl.addToHistory(query)
	}
	wl.visibilityQuery = query
	wl.filterText = ""
	wl.updatePanelTitle()
	wl.loadData()
}

func (wl *WorkflowList) addToHistory(query string) {
	// Don't add duplicates of the most recent
	if len(wl.searchHistory) > 0 && wl.searchHistory[len(wl.searchHistory)-1] == query {
		return
	}
	wl.searchHistory = append(wl.searchHistory, query)
	if len(wl.searchHistory) > wl.maxHistorySize {
		wl.searchHistory = wl.searchHistory[1:]
	}
	wl.historyIndex = -1
}

func (wl *WorkflowList) showQueryTemplates() {
	templates := []struct {
		name  string
		query string
	}{
		// Status filters
		{"Running Workflows", "ExecutionStatus = 'Running'"},
		{"Failed Workflows", "ExecutionStatus = 'Failed'"},
		{"Completed Workflows", "ExecutionStatus = 'Completed'"},
		{"Cancelled Workflows", "ExecutionStatus = 'Canceled'"},
		{"Timed Out Workflows", "ExecutionStatus = 'TimedOut'"},
		// Time-based filters
		{"Started Today", "StartTime > $TODAY"},
		{"Started Yesterday", "StartTime > $YESTERDAY AND StartTime < $TODAY"},
		{"Started This Week", "StartTime > $THIS_WEEK"},
		{"Started Last Hour", "StartTime > $HOUR_AGO"},
		{"Started Last 30 Min", "StartTime > $MINUTES_AGO_30"},
		{"Started Last 7 Days", "StartTime > $DAYS_AGO_7"},
		// Combined filters
		{"Long Running (>1h)", "ExecutionStatus = 'Running' AND StartTime < $HOUR_AGO"},
		{"Long Running (>6h)", "ExecutionStatus = 'Running' AND StartTime < $HOURS_AGO_6"},
		{"Failed Today", "ExecutionStatus = 'Failed' AND StartTime > $TODAY"},
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query Templates", theme.IconInfo),
		Width:    70,
		Height:   24,
		Backdrop: true,
	})

	table := components.NewTable()
	table.SetHeaders("TEMPLATE", "QUERY")
	table.SetBorder(false)

	for _, t := range templates {
		table.AddRow(t.name, truncate(t.query, 45))
	}
	table.SelectRow(0)

	table.SetOnSelect(func(row int) {
		if row >= 0 && row < len(templates) {
			wl.closeModal()
			wl.applyVisibilityQuery(templates[row].query)
		}
	})

	modal.SetContent(table)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(table)
}

func (wl *WorkflowList) showDateRangePicker() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Date Range Filter", theme.IconInfo),
		Width:    55,
		Height:   14,
		Backdrop: true,
	})

	presets := []string{
		"Last Hour",
		"Last 24 Hours",
		"Last 7 Days",
		"Last 30 Days",
		"Today",
		"Yesterday",
	}

	form := components.NewForm()
	form.AddSelect("preset", "Time Range", presets)

	form.SetOnSubmit(func(values map[string]any) {
		preset := values["preset"].(string)
		wl.closeModal()
		wl.applyDatePreset(preset)
	})
	form.SetOnCancel(func() {
		wl.closeModal()
	})

	modal.SetContent(form)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		values := form.GetValues()
		preset := values["preset"].(string)
		wl.closeModal()
		wl.applyDatePreset(preset)
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) applyDatePreset(preset string) {
	now := time.Now()
	var startTime time.Time

	switch preset {
	case "Last Hour":
		startTime = now.Add(-1 * time.Hour)
	case "Last 24 Hours":
		startTime = now.Add(-24 * time.Hour)
	case "Last 7 Days":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "Last 30 Days":
		startTime = now.Add(-30 * 24 * time.Hour)
	case "Today":
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "Yesterday":
		yesterday := now.Add(-24 * time.Hour)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
	default:
		return
	}

	query := fmt.Sprintf("StartTime > '%s'", startTime.UTC().Format(time.RFC3339))
	wl.applyVisibilityQuery(query)
}

func (wl *WorkflowList) showSavedFilters() {
	// For now, show history as "saved" filters
	if len(wl.searchHistory) == 0 {
		wl.showNoSavedFilters()
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query History", theme.IconInfo),
		Width:    70,
		Height:   18,
		Backdrop: true,
	})

	table := components.NewTable()
	table.SetHeaders("#", "QUERY")
	table.SetBorder(false)

	// Show most recent first
	for i := len(wl.searchHistory) - 1; i >= 0; i-- {
		table.AddRow(
			fmt.Sprintf("%d", len(wl.searchHistory)-i),
			truncate(wl.searchHistory[i], 55),
		)
	}
	table.SelectRow(0)

	table.SetOnSelect(func(row int) {
		// Convert display row to history index (most recent first)
		historyIdx := len(wl.searchHistory) - 1 - row
		if historyIdx >= 0 && historyIdx < len(wl.searchHistory) {
			wl.closeModal()
			wl.applyVisibilityQuery(wl.searchHistory[historyIdx])
		}
	})

	modal.SetContent(table)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Apply"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(table)
}

func (wl *WorkflowList) showNoSavedFilters() {
	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Query History", theme.IconInfo),
		Width:    50,
		Height:   10,
		Backdrop: true,
	})

	text := tview.NewTextView().SetDynamicColors(true)
	text.SetBackgroundColor(theme.Bg())
	text.SetTextAlign(tview.AlignCenter)
	text.SetText(fmt.Sprintf(`[%s]No query history yet.[-]

[%s]Use 'F' to enter a visibility query.
Your queries will be saved here.[-]`,
		theme.TagFgDim(),
		theme.TagFg()))

	modal.SetContent(text)
	modal.SetHints([]components.KeyHint{
		{Key: "Esc", Description: "Close"},
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(modal)
}

func (wl *WorkflowList) showSaveFilter() {
	if wl.visibilityQuery == "" {
		return
	}

	modal := components.NewModal(components.ModalConfig{
		Title:    fmt.Sprintf("%s Save Filter", theme.IconInfo),
		Width:    60,
		Height:   12,
		Backdrop: true,
	})

	form := components.NewForm()
	form.AddTextField("name", "Filter Name", "")

	queryText := tview.NewTextView().SetDynamicColors(true)
	queryText.SetBackgroundColor(theme.Bg())
	queryText.SetText(fmt.Sprintf("[%s]Query:[-] %s", theme.TagFgDim(), wl.visibilityQuery))

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(queryText, 2, 0, false).
		AddItem(form, 0, 1, true)
	content.SetBackgroundColor(theme.Bg())

	form.SetOnSubmit(func(values map[string]any) {
		// For now, just add to history (persistent save would require config storage)
		wl.addToHistory(wl.visibilityQuery)
		wl.closeModal()
	})
	form.SetOnCancel(func() {
		wl.closeModal()
	})

	modal.SetContent(content)
	modal.SetHints([]components.KeyHint{
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	})
	modal.SetOnSubmit(func() {
		wl.addToHistory(wl.visibilityQuery)
		wl.closeModal()
	})
	modal.SetOnCancel(func() {
		wl.closeModal()
	})

	wl.app.JigApp().Pages().Push(modal)
	wl.app.JigApp().SetFocus(form)
}

func (wl *WorkflowList) clearVisibilityQuery() {
	wl.visibilityQuery = ""
	wl.updatePanelTitle()
	wl.loadData()
	wl.app.JigApp().Menu().SetHints(wl.Hints())
}

func (wl *WorkflowList) updatePanelTitle() {
	title := fmt.Sprintf("%s Workflows", theme.IconWorkflow)
	if wl.visibilityQuery != "" {
		q := wl.visibilityQuery
		if len(q) > 40 {
			q = q[:37] + "..."
		}
		// Panel doesn't parse tview color codes, use plain text
		title = fmt.Sprintf("%s Workflows (%s)", theme.IconWorkflow, q)
	} else if wl.filterText != "" {
		title = fmt.Sprintf("%s Workflows (/%s)", theme.IconWorkflow, wl.filterText)
	}
	wl.SetMasterTitle(title)
}

// resolveTimePlaceholders resolves time-based placeholders in Temporal visibility queries.
// Supported placeholders (using local timezone):
//   - $TODAY: Start of today (00:00:00)
//   - $YESTERDAY: Start of yesterday (00:00:00)
//   - $THIS_WEEK: Start of current week (Monday 00:00:00)
//   - $HOUR_AGO: 1 hour ago
//   - $HOURS_AGO_N: N hours ago (e.g., $HOURS_AGO_6)
//   - $MINUTES_AGO_N: N minutes ago (e.g., $MINUTES_AGO_30)
//   - $DAYS_AGO_N: N days ago at 00:00:00 (e.g., $DAYS_AGO_7)
func resolveTimePlaceholders(query string) (string, error) {
	now := time.Now()

	// Simple placeholders
	replacements := map[string]string{
		"$TODAY":     startOfDay(now).Format(time.RFC3339),
		"$YESTERDAY": startOfDay(now.AddDate(0, 0, -1)).Format(time.RFC3339),
		"$THIS_WEEK": startOfWeek(now).Format(time.RFC3339),
		"$HOUR_AGO":  now.Add(-1 * time.Hour).Format(time.RFC3339),
	}

	result := query
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, "'"+value+"'")
	}

	// Pattern-based placeholders: $HOURS_AGO_N, $MINUTES_AGO_N, $DAYS_AGO_N
	patterns := []struct {
		prefix string
		unit   time.Duration
		isDate bool // If true, use start of day
	}{
		{"$HOURS_AGO_", time.Hour, false},
		{"$MINUTES_AGO_", time.Minute, false},
		{"$DAYS_AGO_", 24 * time.Hour, true},
	}

	for _, p := range patterns {
		for {
			idx := strings.Index(result, p.prefix)
			if idx == -1 {
				break
			}

			// Find the end of the number
			endIdx := idx + len(p.prefix)
			for endIdx < len(result) && result[endIdx] >= '0' && result[endIdx] <= '9' {
				endIdx++
			}

			if endIdx == idx+len(p.prefix) {
				return "", fmt.Errorf("invalid placeholder: %s (missing number)", p.prefix)
			}

			numStr := result[idx+len(p.prefix) : endIdx]
			num, err := strconv.Atoi(numStr)
			if err != nil {
				return "", fmt.Errorf("invalid number in placeholder %s%s: %w", p.prefix, numStr, err)
			}

			var t time.Time
			if p.isDate {
				t = startOfDay(now.Add(-time.Duration(num) * p.unit))
			} else {
				t = now.Add(-time.Duration(num) * p.unit)
			}

			placeholder := p.prefix + numStr
			result = strings.Replace(result, placeholder, "'"+t.Format(time.RFC3339)+"'", 1)
		}
	}

	return result, nil
}

// startOfDay returns the start of the day (00:00:00) in local timezone.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// startOfWeek returns the start of the week (Monday 00:00:00) in local timezone.
func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday becomes 7
	}
	daysToMonday := weekday - 1
	monday := t.AddDate(0, 0, -daysToMonday)
	return startOfDay(monday)
}
