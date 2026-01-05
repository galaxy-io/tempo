package temporal

import (
	"context"
	"time"
)

// Provider defines the interface for Temporal data access.
// This abstraction allows for different implementations (real SDK, mock, etc.)
type Provider interface {
	// Namespace Operations

	// ListNamespaces returns all namespaces visible to the client.
	ListNamespaces(ctx context.Context) ([]Namespace, error)

	// CreateNamespace registers a new namespace with the Temporal server.
	CreateNamespace(ctx context.Context, req NamespaceCreateRequest) error

	// DescribeNamespace returns detailed information about a namespace.
	DescribeNamespace(ctx context.Context, name string) (*NamespaceDetail, error)

	// UpdateNamespace modifies an existing namespace's configuration.
	UpdateNamespace(ctx context.Context, req NamespaceUpdateRequest) error

	// DeprecateNamespace marks a namespace as deprecated (soft delete).
	// Deprecated namespaces prevent new workflow executions but allow existing ones to complete.
	DeprecateNamespace(ctx context.Context, name string) error

	// DeleteNamespace permanently deletes a namespace.
	// The namespace must be deprecated first before it can be deleted.
	DeleteNamespace(ctx context.Context, name string) error

	// ListWorkflows returns workflows for a namespace with optional filtering.
	ListWorkflows(ctx context.Context, namespace string, opts ListOptions) ([]Workflow, string, error)

	// GetWorkflow returns details for a specific workflow execution.
	GetWorkflow(ctx context.Context, namespace, workflowID, runID string) (*Workflow, error)

	// GetWorkflowHistory returns the event history for a workflow execution.
	GetWorkflowHistory(ctx context.Context, namespace, workflowID, runID string) ([]HistoryEvent, error)

	// GetEnhancedWorkflowHistory returns event history with relational data for tree/timeline views.
	GetEnhancedWorkflowHistory(ctx context.Context, namespace, workflowID, runID string) ([]EnhancedHistoryEvent, error)

	// DescribeTaskQueue returns task queue info and active pollers.
	DescribeTaskQueue(ctx context.Context, namespace, taskQueue string) (*TaskQueueInfo, []Poller, error)

	// Close releases any resources held by the provider.
	Close() error

	// IsConnected returns true if the provider has an active connection.
	IsConnected() bool

	// CheckConnection verifies the connection is still alive by making a lightweight API call.
	CheckConnection(ctx context.Context) error

	// Reconnect attempts to re-establish a connection to the Temporal server.
	// Returns an error if reconnection fails.
	Reconnect(ctx context.Context) error

	// ReconnectWithConfig reconnects using a new configuration.
	// This enables hot-swapping to a different Temporal server/namespace.
	ReconnectWithConfig(ctx context.Context, config ConnectionConfig) error

	// Config returns the connection configuration used by this provider.
	Config() ConnectionConfig

	// Workflow Mutations

	// CancelWorkflow requests graceful cancellation of a workflow execution.
	// The workflow can handle the cancellation and perform cleanup.
	CancelWorkflow(ctx context.Context, namespace, workflowID, runID, reason string) error

	// TerminateWorkflow forcefully terminates a workflow execution immediately.
	// No cleanup code will run in the workflow.
	TerminateWorkflow(ctx context.Context, namespace, workflowID, runID, reason string) error

	// SignalWorkflow sends a signal to a running workflow execution.
	SignalWorkflow(ctx context.Context, namespace, workflowID, runID, signalName string, input []byte) error

	// SignalWithStartWorkflow starts a workflow if it doesn't exist and sends a signal to it.
	// Returns the run ID of the workflow.
	SignalWithStartWorkflow(ctx context.Context, namespace string, req SignalWithStartRequest) (string, error)

	// DeleteWorkflow permanently deletes a workflow execution and its history.
	DeleteWorkflow(ctx context.Context, namespace, workflowID, runID string) error

	// ResetWorkflow resets a workflow to a previous state, creating a new run.
	ResetWorkflow(ctx context.Context, namespace, workflowID, runID string, eventID int64, reason string) (string, error)

	// Schedule Operations

	// ListSchedules returns all schedules in a namespace.
	ListSchedules(ctx context.Context, namespace string, opts ListOptions) ([]Schedule, string, error)

	// GetSchedule returns details for a specific schedule.
	GetSchedule(ctx context.Context, namespace, scheduleID string) (*Schedule, error)

	// PauseSchedule pauses a schedule.
	PauseSchedule(ctx context.Context, namespace, scheduleID, reason string) error

	// UnpauseSchedule unpauses a schedule.
	UnpauseSchedule(ctx context.Context, namespace, scheduleID, reason string) error

	// TriggerSchedule immediately triggers a scheduled workflow execution.
	TriggerSchedule(ctx context.Context, namespace, scheduleID string) error

	// DeleteSchedule permanently deletes a schedule.
	DeleteSchedule(ctx context.Context, namespace, scheduleID string) error

	// Query Operations

	// QueryWorkflow executes a query against a running workflow and returns the result.
	// queryType is the name of the query handler (e.g., "__stack_trace" for built-in stack trace).
	// args is optional JSON-encoded arguments to pass to the query handler.
	QueryWorkflow(ctx context.Context, namespace, workflowID, runID, queryType string, args []byte) (*QueryResult, error)

	// Batch Operations

	// CancelWorkflows cancels multiple workflows and returns results for each.
	CancelWorkflows(ctx context.Context, namespace string, workflows []WorkflowIdentifier) ([]BatchResult, error)

	// TerminateWorkflows terminates multiple workflows and returns results for each.
	TerminateWorkflows(ctx context.Context, namespace string, workflows []WorkflowIdentifier, reason string) ([]BatchResult, error)

	// GetResetPoints returns valid reset points for a workflow execution.
	GetResetPoints(ctx context.Context, namespace, workflowID, runID string) ([]ResetPoint, error)
}

// ListOptions configures workflow list queries.
type ListOptions struct {
	PageSize  int
	PageToken string
	Query     string // Visibility query (e.g., "WorkflowType='OrderWorkflow'")
}

// Namespace represents a Temporal namespace.
type Namespace struct {
	Name            string
	State           string
	RetentionPeriod string
	Description     string
	OwnerEmail      string
}

// NamespaceCreateRequest contains parameters for creating a new namespace.
type NamespaceCreateRequest struct {
	Name          string
	Description   string
	OwnerEmail    string
	RetentionDays int // Minimum 1 day
}

// NamespaceUpdateRequest contains parameters for updating an existing namespace.
type NamespaceUpdateRequest struct {
	Name          string // Target namespace to update
	Description   string
	OwnerEmail    string
	RetentionDays int
}

// NamespaceDetail contains extended namespace information.
type NamespaceDetail struct {
	Namespace
	CreatedAt          time.Time
	UpdatedAt          time.Time
	HistoryArchival    string // Archival state + URI
	VisibilityArchival string // Archival state + URI
	ID                 string // Internal namespace UUID
	IsGlobalNamespace  bool
	FailoverVersion    int64
	Clusters           []string // Active clusters for multi-region
}

// Workflow represents a workflow execution.
type Workflow struct {
	ID        string
	RunID     string
	Type      string
	Status    string // "Running", "Completed", "Failed", "Canceled", "Terminated", "TimedOut"
	Namespace string
	TaskQueue string
	StartTime time.Time
	EndTime   *time.Time
	ParentID  *string
	Memo      map[string]string
	Input     string // JSON-formatted workflow input
	Output    string // JSON-formatted workflow result (or failure message)
}

// HistoryEvent represents a workflow history event.
type HistoryEvent struct {
	ID      int64
	Type    string
	Time    time.Time
	Details string
}

// EnhancedHistoryEvent extends HistoryEvent with relational fields for tree/timeline views.
type EnhancedHistoryEvent struct {
	ID      int64
	Type    string
	Time    time.Time
	Details string // Keep for backward compatibility

	// Relational fields for building event trees
	ScheduledEventID int64 // For Started/Completed events linking to Scheduled
	StartedEventID   int64 // For Completed events linking to Started
	InitiatedEventID int64 // For Child workflow events

	// Activity/Timer identity
	ActivityID   string
	ActivityType string
	TimerID      string

	// Child workflow info
	ChildWorkflowID   string
	ChildWorkflowType string

	// Timing for Gantt view
	EndTime *time.Time // Computed from linked completion event

	// Additional metadata
	Attempt   int32
	TaskQueue string
	Identity  string
	Failure   string
	Result    string
	Input     string // Workflow/Activity input
}

// TaskQueueInfo represents task queue status information.
type TaskQueueInfo struct {
	Name        string
	Type        string // "Workflow" or "Activity"
	PollerCount int
	Backlog     int
}

// Poller represents a worker polling a task queue.
type Poller struct {
	Identity       string
	LastAccessTime time.Time
	TaskQueueType  string // "Workflow" or "Activity"
	RatePerSecond  float64
}

// Schedule represents a Temporal schedule.
type Schedule struct {
	ID             string
	Spec           string // Human-readable schedule specification
	WorkflowType   string
	WorkflowID     string // Base workflow ID
	TaskQueue      string
	Paused         bool
	Notes          string
	NextRunTime    *time.Time
	LastRunTime    *time.Time
	LastRunStatus  string
	TotalActions   int64
	RecentActions  int64 // Actions in the last 24h
	OverlapPolicy  string
}

// ConnectionConfig holds Temporal server connection settings.
type ConnectionConfig struct {
	Address       string
	Namespace     string
	TLSCertPath   string
	TLSKeyPath    string
	TLSCAPath     string
	TLSServerName string
	TLSSkipVerify bool
}

// DefaultConnectionConfig returns default connection settings.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		Address:   "localhost:7233",
		Namespace: "default",
	}
}

// QueryResult represents the result of a workflow query.
type QueryResult struct {
	QueryType string
	Result    string // JSON-formatted result
	Error     string // Error message if query failed
}

// WorkflowIdentifier uniquely identifies a workflow execution.
type WorkflowIdentifier struct {
	WorkflowID string
	RunID      string
}

// BatchResult represents the result of a batch operation on a single workflow.
type BatchResult struct {
	WorkflowID string
	RunID      string
	Success    bool
	Error      string
}

// ResetPoint represents a valid point to reset a workflow to.
type ResetPoint struct {
	EventID     int64
	EventType   string
	Timestamp   time.Time
	Description string // Human-readable description (e.g., "Activity 'ProcessPayment' failed")
	Reason      string // Why this is a valid reset point
}

// SignalWithStartRequest contains parameters for starting a workflow with a signal.
type SignalWithStartRequest struct {
	WorkflowID    string
	WorkflowType  string
	TaskQueue     string
	SignalName    string
	SignalInput   []byte // JSON-encoded signal input
	WorkflowInput []byte // JSON-encoded workflow input
}
