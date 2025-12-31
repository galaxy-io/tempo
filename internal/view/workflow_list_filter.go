package view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
)

// applyFilter filters allWorkflows based on filterText and updates the display.
func (wl *WorkflowList) applyFilter() {
	wl.applyFilterWithFallback(false)
}

// applyFilterWithFallback filters locally, optionally falling back to server-side search.
func (wl *WorkflowList) applyFilterWithFallback(serverFallback bool) {
	if wl.filterText == "" {
		wl.workflows = wl.allWorkflows
	} else {
		filter := strings.ToLower(wl.filterText)
		wl.workflows = nil
		for _, w := range wl.allWorkflows {
			if strings.Contains(strings.ToLower(w.ID), filter) ||
				strings.Contains(strings.ToLower(w.Type), filter) ||
				strings.Contains(strings.ToLower(w.Status), filter) {
				wl.workflows = append(wl.workflows, w)
			}
		}

		if len(wl.workflows) == 0 && serverFallback && wl.visibilityQuery == "" {
			wl.convertFilterToVisibilityQuery()
			return
		}
	}
	wl.populateTable()
	wl.updateStats()
}

func (wl *WorkflowList) convertFilterToVisibilityQuery() {
	if wl.filterText == "" {
		return
	}

	searchTerm := wl.filterText
	wl.visibilityQuery = fmt.Sprintf(
		"WorkflowId STARTS_WITH '%s' OR WorkflowType STARTS_WITH '%s'",
		searchTerm, searchTerm,
	)
	wl.filterText = ""
	wl.updatePanelTitle()
	wl.loadData()
}

func (wl *WorkflowList) showFilter() {
	wl.originalWorkflows = wl.allWorkflows

	wl.app.ShowFilterMode(wl.filterText, FilterModeCallbacks{
		OnSubmit: func(text string) {
			wl.filterText = text
			if text != "" {
				// Apply filter with server fallback if no local results
				wl.applyFilterWithFallback(true)
			} else {
				wl.applyFilter()
			}
			wl.updatePanelTitle()
		},
		OnCancel: func() {
			wl.closeFilter()
		},
		OnChange: func(text string) {
			wl.filterText = text
			wl.applyFilterWithServerSearch(text)
		},
	})
}

// applyFilterWithServerSearch filters locally, and if no results, triggers server search.
func (wl *WorkflowList) applyFilterWithServerSearch(text string) {
	if text == "" {
		wl.workflows = wl.allWorkflows
		wl.populateTable()
		wl.updateStats()
		wl.updateFilterTitle("", "")
		return
	}

	// Try local filter first
	filter := strings.ToLower(text)
	wl.workflows = nil
	for _, w := range wl.allWorkflows {
		if strings.Contains(strings.ToLower(w.ID), filter) ||
			strings.Contains(strings.ToLower(w.Type), filter) ||
			strings.Contains(strings.ToLower(w.Status), filter) {
			wl.workflows = append(wl.workflows, w)
		}
	}

	// Show top match hint
	topHint := ""
	if len(wl.workflows) > 0 {
		topHint = wl.workflows[0].ID
	}
	wl.updateFilterTitle(text, topHint)

	// If no local results and query is long enough, search server
	if len(wl.workflows) == 0 && len(text) >= 2 {
		// Avoid duplicate requests
		if text == wl.lastCompletionQuery {
			return
		}
		wl.lastCompletionQuery = text
		wl.searchServer(text)
		return
	}

	wl.populateTable()
	wl.updateStats()
}

// searchServer performs a server-side search and updates the table.
func (wl *WorkflowList) searchServer(searchTerm string) {
	provider := wl.app.Provider()
	if provider == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := fmt.Sprintf(
			"WorkflowId STARTS_WITH '%s' OR WorkflowType STARTS_WITH '%s'",
			searchTerm, searchTerm,
		)
		opts := temporal.ListOptions{
			PageSize: 50,
			Query:    query,
		}
		workflows, _, err := provider.ListWorkflows(ctx, wl.namespace, opts)

		wl.app.JigApp().QueueUpdateDraw(func() {
			// Only update if we're still filtering with the same term
			if wl.filterText != searchTerm {
				return
			}

			if err != nil {
				return
			}

			wl.workflows = workflows
			wl.serverCompletions = make([]string, 0, len(workflows))
			for _, w := range workflows {
				wl.serverCompletions = append(wl.serverCompletions, w.ID)
			}

			// Update hint with top server result
			topHint := ""
			if len(workflows) > 0 {
				topHint = workflows[0].ID
			}
			wl.updateFilterTitle(searchTerm, topHint)

			wl.populateTable()
			wl.updateStats()
		})
	}()
}

// updateFilterTitle updates the panel title with filter info and hint.
func (wl *WorkflowList) updateFilterTitle(filter, hint string) {
	if filter == "" {
		wl.SetMasterTitle(fmt.Sprintf("%s Workflows", theme.IconWorkflow))
		wl.app.SetFilterSuggestion("")
		return
	}

	// Show only what the user typed in the title (no autocomplete suffix)
	title := fmt.Sprintf("%s Workflows (/%s)", theme.IconWorkflow, filter)
	wl.SetMasterTitle(title)

	// Set ghost text suggestion in command bar if we have a matching hint
	if hint != "" && strings.HasPrefix(strings.ToLower(hint), strings.ToLower(filter)) {
		wl.app.SetFilterSuggestion(hint)
	} else {
		wl.app.SetFilterSuggestion("")
	}
}

func (wl *WorkflowList) closeFilter() {
	wl.serverCompletions = nil
	wl.lastCompletionQuery = ""

	if wl.filterText == "" && wl.visibilityQuery == "" && wl.originalWorkflows != nil {
		wl.allWorkflows = wl.originalWorkflows
		wl.workflows = wl.originalWorkflows
		wl.originalWorkflows = nil
		wl.populateTable()
		wl.updateStats()
		wl.updatePanelTitle()
	}
}

func (wl *WorkflowList) clearAllFilters() {
	wl.filterText = ""
	wl.visibilityQuery = ""
	wl.serverCompletions = nil
	wl.lastCompletionQuery = ""

	if wl.originalWorkflows != nil {
		wl.allWorkflows = wl.originalWorkflows
		wl.workflows = wl.originalWorkflows
		wl.originalWorkflows = nil
		wl.populateTable()
		wl.updateStats()
		wl.updatePanelTitle()
	} else {
		wl.loadData()
	}
}
