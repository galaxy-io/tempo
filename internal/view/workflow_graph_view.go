package view

import (
	"context"
	"fmt"
	"time"

	"github.com/atterpac/jig/components"
	"github.com/atterpac/jig/theme"
	"github.com/galaxy-io/tempo/internal/temporal"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowGraphView displays workflow relationships in a dual-pane layout.
// Left pane: GraphTree showing hierarchical tree of workflows
// Right pane: NodeGraph showing 2D visualization of relationships
type WorkflowGraphView struct {
	*components.Split
	app       *App
	namespace string
	workflow  *temporal.Workflow

	tree     *components.GraphTree
	graph    *components.NodeGraph
	treeData *components.GraphTreeData
	graphData *components.NodeGraphData

	relationships *temporal.WorkflowRelationships
	depthLimit    int
	loading       bool
}

// NewWorkflowGraphView creates a new workflow graph view.
func NewWorkflowGraphView(app *App, namespace string, workflow *temporal.Workflow) *WorkflowGraphView {
	wg := &WorkflowGraphView{
		app:        app,
		namespace:  namespace,
		workflow:   workflow,
		depthLimit: 1, // Default depth (keep low for performance)
	}
	wg.setup()

	// Register for automatic theme refresh
	theme.RegisterRefreshable(wg)

	return wg
}

func (wg *WorkflowGraphView) setup() {
	// Create the tree component
	wg.tree = components.NewGraphTree()
	wg.tree.SetShowEdgeLabels(true)
	wg.tree.SetOnChange(wg.onTreeSelectionChange)
	wg.tree.SetOnSelect(wg.onTreeSelect)
	wg.tree.SetOnLoadChildren(wg.loadChildren)

	// Create the graph component
	wg.graph = components.NewNodeGraph()
	wg.graph.SetShowEdgeLabels(true)
	wg.graph.SetNodeWidth(18)
	wg.graph.SetOnSelect(wg.onGraphSelect)

	// Wrap in panels with titles
	treePanel := components.NewPanel().
		SetTitle(fmt.Sprintf("%s Hierarchy", theme.IconNamespace)).
		SetContent(wg.tree)

	graphPanel := components.NewPanel().
		SetTitle(fmt.Sprintf("%s Graph View", theme.IconGrid)).
		SetContent(wg.graph)

	// Create split layout
	wg.Split = components.NewSplit().
		SetDirection(components.SplitHorizontal).
		SetRatio(0.4).
		SetLeft(treePanel).
		SetRight(graphPanel).
		SetResizable(true).
		SetShowDivider(true)
}

// RefreshTheme updates colors after a theme change.
func (wg *WorkflowGraphView) RefreshTheme() {
	// Components auto-refresh via theme.Register
}

// Name returns the view name.
func (wg *WorkflowGraphView) Name() string {
	return "workflow-graph"
}

// Start is called when the view becomes active.
func (wg *WorkflowGraphView) Start() {
	wg.Split.SetInputCapture(wg.handleInput)
	wg.loadData()
}

// Stop is called when the view is deactivated.
func (wg *WorkflowGraphView) Stop() {
	wg.Split.SetInputCapture(nil)
}

// Hints returns keybinding hints for this view.
func (wg *WorkflowGraphView) Hints() []KeyHint {
	return []KeyHint{
		{Key: "j/k", Description: "Navigate"},
		{Key: "h/l", Description: "Collapse/Expand"},
		{Key: "Tab", Description: "Switch Pane"},
		{Key: "c", Description: "Center Graph"},
		{Key: "+/-", Description: "Depth"},
		{Key: "Enter", Description: "Open Detail"},
		{Key: "r", Description: "Refresh"},
		{Key: "?", Description: "Help"},
		{Key: "esc", Description: "Back"},
	}
}

// Focus sets focus to the tree pane.
func (wg *WorkflowGraphView) Focus(delegate func(p tview.Primitive)) {
	// Ensure Split focuses the first pane (tree)
	wg.Split.FocusFirst()
	wg.Split.Focus(delegate)
}

func (wg *WorkflowGraphView) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		// Go back to previous view
		wg.app.JigApp().Pages().Pop()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case '+':
			wg.adjustDepth(1)
			return nil
		case '-':
			wg.adjustDepth(-1)
			return nil
		case 'r':
			wg.loadData()
			return nil
		case 'c':
			// Center graph on current selection
			if node := wg.tree.GetSelected(); node != nil {
				wg.graph.SetFocus(node.ID)
			}
			return nil
		case 'q':
			// Also allow q to go back (vim-style)
			wg.app.JigApp().Pages().Pop()
			return nil
		}
	}
	return event
}

func (wg *WorkflowGraphView) adjustDepth(delta int) {
	newDepth := wg.depthLimit + delta
	if newDepth < 1 {
		newDepth = 1
	}
	if newDepth > 10 {
		newDepth = 10
	}
	if newDepth != wg.depthLimit {
		wg.depthLimit = newDepth
		wg.app.ToastSuccess(fmt.Sprintf("Depth limit: %d", wg.depthLimit))
		wg.loadData()
	}
}

func (wg *WorkflowGraphView) loadData() {
	if wg.loading {
		return
	}
	wg.loading = true

	// Show initial state with just the current workflow while loading children
	wg.showInitialState()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		provider := wg.app.Provider()
		if provider == nil {
			wg.app.JigApp().QueueUpdateDraw(func() {
				wg.app.ToastError("No provider available")
				wg.loading = false
			})
			return
		}

		relationships, err := provider.GetWorkflowRelationships(
			ctx,
			wg.namespace,
			wg.workflow.ID,
			wg.workflow.RunID,
			wg.depthLimit,
		)
		if err != nil {
			wg.app.JigApp().QueueUpdateDraw(func() {
				wg.app.ToastError(fmt.Sprintf("Failed to load relationships: %v", err))
				wg.loading = false
			})
			return
		}

		wg.app.JigApp().QueueUpdateDraw(func() {
			wg.relationships = relationships
			wg.buildTreeData()
			wg.buildGraphData()
			wg.loading = false
		})
	}()
}

// showInitialState displays the current workflow immediately while loading relationships.
func (wg *WorkflowGraphView) showInitialState() {
	// Create minimal tree with just the current workflow
	wg.treeData = components.NewGraphTreeData()
	currentNode := &components.GraphTreeNode{
		ID:        wg.workflow.ID,
		Label:     truncateLabel(wg.workflow.ID, 30),
		Sublabel:  wg.workflow.Type,
		Status:    wg.workflow.Status,
		NodeType:  components.GraphNodePrimary,
		CanExpand: true,
		Loading:   true, // Show loading indicator
		Data:      wg.workflow,
	}
	wg.treeData.AddNode(currentNode)
	wg.treeData.RootID = wg.workflow.ID
	wg.tree.SetData(wg.treeData)

	// Create minimal graph with just the current workflow
	wg.graphData = components.NewNodeGraphData()
	wg.graphData.AddNode(&components.GraphNode{
		ID:      wg.workflow.ID,
		Label:   truncateLabel(wg.workflow.ID, 14),
		Status:  wg.workflow.Status,
		Focused: true,
		Data:    wg.workflow,
	})
	wg.graphData.FocusID = wg.workflow.ID
	wg.graph.SetData(wg.graphData)
}

func (wg *WorkflowGraphView) buildTreeData() {
	wg.treeData = components.NewGraphTreeData()

	if wg.relationships == nil || wg.relationships.Current == nil {
		return
	}

	// Add parent if exists
	if wg.relationships.Parent != nil {
		parent := wg.relationships.Parent
		parentNode := &components.GraphTreeNode{
			ID:        parent.ID,
			Label:     parent.ID,
			Sublabel:  parent.Type,
			Status:    parent.Status,
			NodeType:  components.GraphNodeSecondary,
			CanExpand: false,
			Expanded:  true,
			Data:      parent,
		}
		wg.treeData.AddNode(parentNode)
		wg.treeData.RootID = parent.ID
	}

	// Add current workflow
	current := wg.relationships.Current
	currentNode := &components.GraphTreeNode{
		ID:        current.ID,
		Label:     current.ID,
		Sublabel:  current.Type,
		Status:    current.Status,
		NodeType:  components.GraphNodePrimary,
		CanExpand: len(wg.relationships.Children) > 0,
		Expanded:  true,
		Data:      current,
	}

	// Link to parent if exists
	if wg.relationships.Parent != nil {
		parentNode := wg.treeData.GetNode(wg.relationships.Parent.ID)
		if parentNode != nil {
			parentNode.Children = append(parentNode.Children, current.ID)
		}
		wg.treeData.AddEdge(&components.GraphTreeEdge{
			From: wg.relationships.Parent.ID,
			To:   current.ID,
			Type: components.GraphEdgeSolid,
		})
	} else {
		wg.treeData.RootID = current.ID
	}

	wg.treeData.AddNode(currentNode)

	// Add children recursively
	wg.addChildrenToTree(currentNode, wg.relationships.Children, 1)

	// Add signal relationships as link nodes
	for _, signal := range wg.relationships.OutgoingSignals {
		signalNode := &components.GraphTreeNode{
			ID:       fmt.Sprintf("signal-%s-%s", signal.FromWorkflowID, signal.ToWorkflowID),
			Label:    fmt.Sprintf("Signal: %s", signal.SignalName),
			Sublabel: fmt.Sprintf("-> %s", truncateLabel(signal.ToWorkflowID, 20)),
			NodeType: components.GraphNodeLink,
			Data:     signal,
		}
		wg.treeData.AddNode(signalNode)
		currentNode.Children = append(currentNode.Children, signalNode.ID)
		wg.treeData.AddEdge(&components.GraphTreeEdge{
			From:  current.ID,
			To:    signalNode.ID,
			Type:  components.GraphEdgeDashed,
			Label: signal.SignalName,
		})
	}

	wg.tree.SetData(wg.treeData)
}

func (wg *WorkflowGraphView) addChildrenToTree(parentNode *components.GraphTreeNode, children []*temporal.WorkflowNode, depth int) {
	for _, child := range children {
		childNode := &components.GraphTreeNode{
			ID:        child.ID,
			Label:     child.ID,
			Sublabel:  child.Type,
			Status:    child.Status,
			NodeType:  components.GraphNodeSecondary,
			Depth:     depth,
			CanExpand: len(child.Children) > 0,
			Expanded:  depth < 2, // Auto-expand first 2 levels
			Data:      &child.Workflow,
		}

		wg.treeData.AddNode(childNode)
		parentNode.Children = append(parentNode.Children, child.ID)

		edgeType := components.GraphEdgeSolid
		if child.EdgeType == "signal" {
			edgeType = components.GraphEdgeDashed
		} else if child.EdgeType == "continue" {
			edgeType = components.GraphEdgeDotted
		}

		wg.treeData.AddEdge(&components.GraphTreeEdge{
			From: parentNode.ID,
			To:   child.ID,
			Type: edgeType,
		})

		// Recurse for grandchildren
		if len(child.Children) > 0 {
			wg.addChildrenToTree(childNode, child.Children, depth+1)
		}
	}
}

func (wg *WorkflowGraphView) buildGraphData() {
	wg.graphData = components.NewNodeGraphData()

	if wg.relationships == nil || wg.relationships.Current == nil {
		return
	}

	// Add parent if exists
	if wg.relationships.Parent != nil {
		parent := wg.relationships.Parent
		wg.graphData.AddNode(&components.GraphNode{
			ID:     parent.ID,
			Label:  truncateLabel(parent.ID, 14),
			Status: parent.Status,
			Data:   parent,
		})
	}

	// Add current workflow (focused)
	current := wg.relationships.Current
	wg.graphData.AddNode(&components.GraphNode{
		ID:      current.ID,
		Label:   truncateLabel(current.ID, 14),
		Status:  current.Status,
		Focused: true,
		Data:    current,
	})
	wg.graphData.FocusID = current.ID

	// Add edge from parent to current
	if wg.relationships.Parent != nil {
		wg.graphData.AddEdge(&components.GraphEdge{
			From: wg.relationships.Parent.ID,
			To:   current.ID,
			Type: components.GraphEdgeSolid,
		})
	}

	// Add children recursively
	wg.addChildrenToGraph(current.ID, wg.relationships.Children)

	// Add signal edges
	for _, signal := range wg.relationships.OutgoingSignals {
		// Only add edge if target exists in graph
		if wg.graphData.GetNode(signal.ToWorkflowID) != nil {
			wg.graphData.AddEdge(&components.GraphEdge{
				From:  signal.FromWorkflowID,
				To:    signal.ToWorkflowID,
				Type:  components.GraphEdgeDashed,
				Label: signal.SignalName,
			})
		}
	}

	wg.graph.SetData(wg.graphData)
}

func (wg *WorkflowGraphView) addChildrenToGraph(parentID string, children []*temporal.WorkflowNode) {
	for _, child := range children {
		wg.graphData.AddNode(&components.GraphNode{
			ID:     child.ID,
			Label:  truncateLabel(child.ID, 14),
			Status: child.Status,
			Data:   &child.Workflow,
		})

		edgeType := components.GraphEdgeSolid
		if child.EdgeType == "signal" {
			edgeType = components.GraphEdgeDashed
		} else if child.EdgeType == "continue" {
			edgeType = components.GraphEdgeDotted
		}

		wg.graphData.AddEdge(&components.GraphEdge{
			From: parentID,
			To:   child.ID,
			Type: edgeType,
		})

		// Recurse
		if len(child.Children) > 0 {
			wg.addChildrenToGraph(child.ID, child.Children)
		}
	}
}

func (wg *WorkflowGraphView) onTreeSelectionChange(node *components.GraphTreeNode) {
	if node == nil {
		return
	}
	// Update graph focus to match tree selection
	wg.graph.SetFocus(node.ID)
}

func (wg *WorkflowGraphView) onTreeSelect(node *components.GraphTreeNode) {
	if node == nil || node.Data == nil {
		return
	}

	// Navigate to workflow detail
	if wf, ok := node.Data.(*temporal.Workflow); ok {
		wg.app.NavigateToWorkflowDetail(wf.ID, wf.RunID)
	}
}

func (wg *WorkflowGraphView) onGraphSelect(node *components.GraphNode) {
	if node == nil || node.Data == nil {
		return
	}

	// Navigate to workflow detail
	if wf, ok := node.Data.(*temporal.Workflow); ok {
		wg.app.NavigateToWorkflowDetail(wf.ID, wf.RunID)
	}
}

func (wg *WorkflowGraphView) loadChildren(nodeID string) ([]*components.GraphTreeNode, []*components.GraphTreeEdge) {
	// Lazy load children for a node
	ctx := context.Background()
	provider := wg.app.Provider()
	if provider == nil {
		return nil, nil
	}

	// Get the workflow for this node
	treeNode := wg.treeData.GetNode(nodeID)
	if treeNode == nil || treeNode.Data == nil {
		return nil, nil
	}

	wf, ok := treeNode.Data.(*temporal.Workflow)
	if !ok {
		return nil, nil
	}

	children, err := provider.GetChildWorkflows(ctx, wg.namespace, wf.ID, wf.RunID)
	if err != nil {
		return nil, nil
	}

	var nodes []*components.GraphTreeNode
	var edges []*components.GraphTreeEdge

	for _, child := range children {
		childNode := &components.GraphTreeNode{
			ID:        child.ID,
			Label:     truncateLabel(child.ID, 30),
			Sublabel:  child.Type,
			Status:    child.Status,
			NodeType:  components.GraphNodeSecondary,
			CanExpand: true, // Assume children might have children
			Data:      &child,
		}
		nodes = append(nodes, childNode)

		edges = append(edges, &components.GraphTreeEdge{
			From: nodeID,
			To:   child.ID,
			Type: components.GraphEdgeSolid,
		})

		// Also add to graph
		wg.graphData.AddNode(&components.GraphNode{
			ID:     child.ID,
			Label:  truncateLabel(child.ID, 14),
			Status: child.Status,
			Data:   &child,
		})
		wg.graphData.AddEdge(&components.GraphEdge{
			From: nodeID,
			To:   child.ID,
			Type: components.GraphEdgeSolid,
		})
	}

	return nodes, edges
}

func truncateLabel(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
