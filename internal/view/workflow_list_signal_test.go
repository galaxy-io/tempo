package view

import (
	"testing"

	"github.com/galaxy-io/tempo/internal/temporal"
)

type mockTable struct {
	selectedRows []int
	selectedRow  int
}

func (m *mockTable) GetSelectedRows() []int { return m.selectedRows }
func (m *mockTable) SelectedRow() int       { return m.selectedRow }

type mockDiffApp struct {
	calledEmpty bool
	calledWithA *temporal.Workflow
	calledWithB *temporal.Workflow
}

func (m *mockDiffApp) NavigateToWorkflowDiff(a, b *temporal.Workflow) {
	m.calledWithA = a
	m.calledWithB = b
}

func (m *mockDiffApp) NavigateToWorkflowDiffEmpty() {
	m.calledEmpty = true
}

type tableForDiff interface {
	GetSelectedRows() []int
	SelectedRow() int
}

type appForDiff interface {
	NavigateToWorkflowDiff(a, b *temporal.Workflow)
	NavigateToWorkflowDiffEmpty()
}

func testableStartDiff(table tableForDiff, app appForDiff, workflows []temporal.Workflow) {
	selected := table.GetSelectedRows()
	if len(selected) == 2 {
		if selected[0] < len(workflows) && selected[1] < len(workflows) {
			wfA := workflows[selected[0]]
			wfB := workflows[selected[1]]
			app.NavigateToWorkflowDiff(&wfA, &wfB)
			return
		}
	}

	row := table.SelectedRow()
	if row < 0 || row >= len(workflows) {
		app.NavigateToWorkflowDiffEmpty()
		return
	}

	wf := workflows[row]
	app.NavigateToWorkflowDiff(&wf, nil)
}

func TestStartDiff(t *testing.T) {
	workflows := []temporal.Workflow{
		{ID: "wf-0"},
		{ID: "wf-1"},
		{ID: "wf-2"},
	}

	tests := []struct {
		name         string
		selectedRows []int
		selectedRow  int
		wantEmpty    bool
		wantA        string
		wantB        string
	}{
		{
			name:         "two rows selected - both workflows passed",
			selectedRows: []int{0, 2},
			selectedRow:  0,
			wantA:        "wf-0",
			wantB:        "wf-2",
		},
		{
			name:         "three rows selected - falls back to single",
			selectedRows: []int{0, 1, 2},
			selectedRow:  1,
			wantA:        "wf-1",
			wantB:        "",
		},
		{
			name:         "one row selected - falls back to single",
			selectedRows: []int{0},
			selectedRow:  0,
			wantA:        "wf-0",
			wantB:        "",
		},
		{
			name:         "no rows selected - uses current row",
			selectedRows: []int{},
			selectedRow:  1,
			wantA:        "wf-1",
			wantB:        "",
		},
		{
			name:         "no selection and invalid row - empty diff",
			selectedRows: []int{},
			selectedRow:  -1,
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &mockTable{
				selectedRows: tt.selectedRows,
				selectedRow:  tt.selectedRow,
			}
			app := &mockDiffApp{}

			testableStartDiff(table, app, workflows)

			if tt.wantEmpty {
				if !app.calledEmpty {
					t.Error("expected NavigateToWorkflowDiffEmpty to be called")
				}
				return
			}

			if app.calledEmpty {
				t.Error("did not expect NavigateToWorkflowDiffEmpty to be called")
			}

			gotA := ""
			if app.calledWithA != nil {
				gotA = app.calledWithA.ID
			}
			if gotA != tt.wantA {
				t.Errorf("workflowA.ID = %q, want %q", gotA, tt.wantA)
			}

			gotB := ""
			if app.calledWithB != nil {
				gotB = app.calledWithB.ID
			}
			if gotB != tt.wantB {
				t.Errorf("workflowB.ID = %q, want %q", gotB, tt.wantB)
			}
		})
	}
}
