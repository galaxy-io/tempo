package view

import (
	"fmt"

	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// EventTreeView displays workflow history events in a collapsible tree structure.
type EventTreeView struct {
	*tview.TreeView
	root         *tview.TreeNode
	nodes        []*temporal.EventTreeNode
	onSelect     func(node *temporal.EventTreeNode)
	onSelChange  func(node *temporal.EventTreeNode)
	selectedNode *temporal.EventTreeNode
}

// NewEventTreeView creates a new tree view for displaying workflow events.
func NewEventTreeView() *EventTreeView {
	root := tview.NewTreeNode("Events")
	tree := tview.NewTreeView().SetRoot(root).SetCurrentNode(root)
	tree.SetBackgroundColor(tcell.ColorDefault)
	tree.SetGraphics(true)

	etv := &EventTreeView{
		TreeView: tree,
		root:     root,
	}

	// Handle selection changes
	tree.SetChangedFunc(func(node *tview.TreeNode) {
		if node != nil {
			ref := node.GetReference()
			if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
				etv.selectedNode = eventNode
				if etv.onSelChange != nil {
					etv.onSelChange(eventNode)
				}
			}
		}
	})

	// Handle enter key (toggle expand/collapse or select)
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node == nil {
			return
		}

		ref := node.GetReference()
		if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
			// Toggle expand/collapse if has children
			if eventNode.HasChildren() {
				eventNode.Collapsed = !eventNode.Collapsed
				node.SetExpanded(!eventNode.Collapsed)
			}

			// Call select handler
			if etv.onSelect != nil {
				etv.onSelect(eventNode)
			}
		}
	})

	return etv
}

// Destroy is a no-op kept for backward compatibility.
func (etv *EventTreeView) Destroy() {}

// Draw applies theme colors dynamically before drawing.
func (etv *EventTreeView) Draw(screen tcell.Screen) {
	etv.SetBackgroundColor(theme.Bg())
	etv.SetGraphicsColor(theme.FgDim())
	etv.root.SetColor(theme.Accent())
	etv.refreshColors()
	etv.TreeView.Draw(screen)
}

// SetNodes populates the tree with event nodes.
func (etv *EventTreeView) SetNodes(nodes []*temporal.EventTreeNode) {
	etv.nodes = nodes
	etv.root.ClearChildren()

	for _, node := range nodes {
		treeNode := etv.createTreeNode(node, 0)
		etv.root.AddChild(treeNode)
	}

	// Expand root by default
	etv.root.SetExpanded(true)

	// Select first node if available
	if len(nodes) > 0 {
		if children := etv.root.GetChildren(); len(children) > 0 {
			etv.SetCurrentNode(children[0])
		}
	}
}

// createTreeNode recursively creates tview tree nodes from EventTreeNodes.
func (etv *EventTreeView) createTreeNode(node *temporal.EventTreeNode, depth int) *tview.TreeNode {
	// Build display text
	text := etv.formatNodeText(node)

	treeNode := tview.NewTreeNode(text).
		SetReference(node).
		SetSelectable(true).
		SetExpanded(!node.Collapsed)

	// Set color based on status
	treeNode.SetColor(etv.statusColor(node.Status))

	// Add children (attempts for activities with retries)
	for _, child := range node.Children {
		childTreeNode := etv.createTreeNode(child, depth+1)
		treeNode.AddChild(childTreeNode)
	}

	return treeNode
}

// formatNodeText creates the display text for a tree node.
func (etv *EventTreeView) formatNodeText(node *temporal.EventTreeNode) string {
	icon := etv.statusIcon(node.Status)
	name := node.Name

	// Add duration if completed
	var suffix string
	if node.EndTime != nil && node.Duration > 0 {
		suffix = fmt.Sprintf(" %s", temporal.FormatDuration(node.Duration))
	}

	// Add attempt count if multiple attempts
	if node.Attempts > 1 {
		suffix = fmt.Sprintf(" %d attempts", node.Attempts)
	}

	// Add status tag
	statusTag := fmt.Sprintf("[%s]", node.Status)

	return fmt.Sprintf("%s %s %s%s", icon, name, statusTag, suffix)
}

// statusIcon returns the icon for a node status.
func (etv *EventTreeView) statusIcon(status string) string {
	switch status {
	case "Running":
		return theme.IconRunning
	case "Completed":
		return theme.IconCompleted
	case "Failed":
		return theme.IconFailed
	case "Canceled":
		return theme.IconCanceled
	case "Terminated":
		return theme.IconTerminated
	case "TimedOut":
		return theme.IconTimedOut
	case "Fired":
		return theme.IconCompleted
	case "Scheduled", "Initiated", "Pending":
		return theme.IconPending
	default:
		return theme.IconEvent
	}
}

// statusColor returns the color for a node status.
func (etv *EventTreeView) statusColor(status string) tcell.Color {
	return temporal.GetWorkflowStatus(status).Color()
}

// refreshColors updates all node colors after theme change.
func (etv *EventTreeView) refreshColors() {
	etv.walkNodes(etv.root, func(node *tview.TreeNode) {
		ref := node.GetReference()
		if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
			node.SetColor(etv.statusColor(eventNode.Status))
		}
	})
}

// walkNodes traverses all nodes in the tree.
func (etv *EventTreeView) walkNodes(node *tview.TreeNode, fn func(*tview.TreeNode)) {
	fn(node)
	for _, child := range node.GetChildren() {
		etv.walkNodes(child, fn)
	}
}

// SetOnSelect sets the callback for when a node is activated (Enter pressed).
func (etv *EventTreeView) SetOnSelect(fn func(node *temporal.EventTreeNode)) {
	etv.onSelect = fn
}

// SetOnSelectionChanged sets the callback for when selection changes.
func (etv *EventTreeView) SetOnSelectionChanged(fn func(node *temporal.EventTreeNode)) {
	etv.onSelChange = fn
}

// SelectedNode returns the currently selected event node.
func (etv *EventTreeView) SelectedNode() *temporal.EventTreeNode {
	return etv.selectedNode
}

// ExpandAll expands all nodes in the tree.
func (etv *EventTreeView) ExpandAll() {
	etv.walkNodes(etv.root, func(node *tview.TreeNode) {
		node.SetExpanded(true)
		ref := node.GetReference()
		if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
			eventNode.Collapsed = false
		}
	})
}

// CollapseAll collapses all nodes in the tree (except root).
func (etv *EventTreeView) CollapseAll() {
	for _, child := range etv.root.GetChildren() {
		etv.walkNodes(child, func(node *tview.TreeNode) {
			node.SetExpanded(false)
			ref := node.GetReference()
			if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
				eventNode.Collapsed = true
			}
		})
	}
}

// JumpToFailed finds and selects the first failed node.
func (etv *EventTreeView) JumpToFailed() bool {
	var failedNode *tview.TreeNode
	etv.walkNodes(etv.root, func(node *tview.TreeNode) {
		if failedNode != nil {
			return
		}
		ref := node.GetReference()
		if eventNode, ok := ref.(*temporal.EventTreeNode); ok {
			if eventNode.Status == "Failed" || eventNode.Status == "TimedOut" {
				failedNode = node
			}
		}
	})

	if failedNode != nil {
		// Expand parent nodes to make it visible
		etv.expandParentsOf(failedNode)
		etv.SetCurrentNode(failedNode)
		return true
	}
	return false
}

// expandParentsOf expands all parent nodes of the given node.
func (etv *EventTreeView) expandParentsOf(target *tview.TreeNode) {
	// Walk from root and expand nodes on the path to target
	etv.expandPath(etv.root, target)
}

// expandPath recursively expands nodes on the path to target.
func (etv *EventTreeView) expandPath(current, target *tview.TreeNode) bool {
	if current == target {
		return true
	}

	for _, child := range current.GetChildren() {
		if etv.expandPath(child, target) {
			current.SetExpanded(true)
			if ref, ok := current.GetReference().(*temporal.EventTreeNode); ok {
				ref.Collapsed = false
			}
			return true
		}
	}

	return false
}

// NodeCount returns the total number of nodes.
func (etv *EventTreeView) NodeCount() int {
	count := 0
	etv.walkNodes(etv.root, func(_ *tview.TreeNode) {
		count++
	})
	return count - 1 // Exclude root
}
