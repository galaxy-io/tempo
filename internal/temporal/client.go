package temporal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/galaxy-io/tempo/internal/config"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	namespacepb "go.temporal.io/api/namespace/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	logFile   *os.File
	sdkLogger *fileLogger
)

// fileLogger writes logs to a file.
type fileLogger struct {
	logger *log.Logger
}

func (l *fileLogger) Debug(msg string, keyvals ...interface{}) {
	l.logger.Printf("DEBUG: %s %v", msg, keyvals)
}

func (l *fileLogger) Info(msg string, keyvals ...interface{}) {
	l.logger.Printf("INFO: %s %v", msg, keyvals)
}

func (l *fileLogger) Warn(msg string, keyvals ...interface{}) {
	l.logger.Printf("WARN: %s %v", msg, keyvals)
}

func (l *fileLogger) Error(msg string, keyvals ...interface{}) {
	l.logger.Printf("ERROR: %s %v", msg, keyvals)
}

// initLogFile sets up logging to a file in the config directory.
func initLogFile() {
	if logFile != nil {
		return
	}

	logPath := filepath.Join(config.ConfigDir(), "tempo.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fall back to discarding logs if we can't open the file
		sdkLogger = &fileLogger{logger: log.New(os.Stderr, "", 0)}
		return
	}
	logFile = f
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	sdkLogger = &fileLogger{logger: log.New(f, "", log.Ldate|log.Ltime)}
}

// Client implements the Provider interface using the Temporal SDK.
type Client struct {
	client    client.Client
	config    ConnectionConfig
	connected bool
	mu        sync.RWMutex
}

// NewClient creates a new Temporal SDK client with the given configuration.
func NewClient(ctx context.Context, connConfig ConnectionConfig) (*Client, error) {
	// Redirect logs to file instead of stdout
	initLogFile()

	opts := client.Options{
		HostPort:  connConfig.Address,
		Namespace: connConfig.Namespace,
		Logger:    sdkLogger,
	}

	// Configure TLS if any TLS options are provided
	if connConfig.TLSCertPath != "" || connConfig.TLSCAPath != "" || connConfig.TLSSkipVerify {
		tlsConfig, err := buildTLSConfig(connConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
		opts.ConnectionOptions.TLS = tlsConfig
	}

	c, err := client.DialContext(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Temporal server: %w", err)
	}

	return &Client{
		client:    c,
		config:    connConfig,
		connected: true,
	}, nil
}

// buildTLSConfig creates a TLS configuration from the connection config.
func buildTLSConfig(config ConnectionConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.TLSSkipVerify,
	}

	if config.TLSServerName != "" {
		tlsConfig.ServerName = config.TLSServerName
	}

	// Load client certificate if provided
	if config.TLSCertPath != "" && config.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if config.TLSCAPath != "" {
		caCert, err := os.ReadFile(config.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// Close releases the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// CheckConnection verifies the connection is still alive by making a lightweight API call.
func (c *Client) CheckConnection(ctx context.Context) error {
	c.mu.RLock()
	cl := c.client
	c.mu.RUnlock()

	if cl == nil {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return fmt.Errorf("client is nil")
	}

	// Make a lightweight API call to check connection
	// ListNamespaces with PageSize 1 is a good health check
	_, err := cl.WorkflowService().ListNamespaces(ctx, &workflowservice.ListNamespacesRequest{
		PageSize: 1,
	})
	if err != nil {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return fmt.Errorf("connection check failed: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

// Reconnect attempts to re-establish a connection to the Temporal server.
func (c *Client) Reconnect(ctx context.Context) error {
	c.mu.RLock()
	reconnConfig := c.config
	c.mu.RUnlock()
	return c.reconnectWithConfig(ctx, reconnConfig)
}

// ReconnectWithConfig reconnects using a new configuration.
// This enables hot-swapping to a different Temporal server/namespace.
func (c *Client) ReconnectWithConfig(ctx context.Context, newConfig ConnectionConfig) error {
	return c.reconnectWithConfig(ctx, newConfig)
}

// reconnectWithConfig is the internal implementation for reconnection.
func (c *Client) reconnectWithConfig(ctx context.Context, connConfig ConnectionConfig) error {
	c.mu.Lock()
	// Close existing client if any
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	c.connected = false
	c.mu.Unlock()

	opts := client.Options{
		HostPort:  connConfig.Address,
		Namespace: connConfig.Namespace,
		Logger:    sdkLogger,
	}

	// Configure TLS if any TLS options are provided
	if connConfig.TLSCertPath != "" || connConfig.TLSCAPath != "" || connConfig.TLSSkipVerify {
		tlsConfig, err := buildTLSConfig(connConfig)
		if err != nil {
			return fmt.Errorf("failed to configure TLS: %w", err)
		}
		opts.ConnectionOptions.TLS = tlsConfig
	}

	newClient, err := client.DialContext(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	c.mu.Lock()
	c.client = newClient
	c.config = connConfig // Update stored config
	c.connected = true
	c.mu.Unlock()

	return nil
}

// Config returns the connection configuration used by this client.
func (c *Client) Config() ConnectionConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// ListNamespaces returns all namespaces visible to the client.
func (c *Client) ListNamespaces(ctx context.Context) ([]Namespace, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	var namespaces []Namespace
	var nextPageToken []byte

	for {
		resp, err := c.client.WorkflowService().ListNamespaces(ctx, &workflowservice.ListNamespacesRequest{
			PageSize:      100,
			NextPageToken: nextPageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list namespaces: %w", err)
		}

		for _, ns := range resp.GetNamespaces() {
			info := ns.GetNamespaceInfo()
			config := ns.GetConfig()

			retention := "N/A"
			if config.GetWorkflowExecutionRetentionTtl() != nil {
				retention = formatDuration(config.GetWorkflowExecutionRetentionTtl())
			}

			namespaces = append(namespaces, Namespace{
				Name:            info.GetName(),
				State:           MapNamespaceState(info.GetState()),
				RetentionPeriod: retention,
				Description:     info.GetDescription(),
				OwnerEmail:      info.GetOwnerEmail(),
			})
		}

		nextPageToken = resp.GetNextPageToken()
		if len(nextPageToken) == 0 {
			break
		}
	}

	return namespaces, nil
}

// CreateNamespace registers a new namespace with the Temporal server.
func (c *Client) CreateNamespace(ctx context.Context, req NamespaceCreateRequest) error {
	if req.RetentionDays < 1 {
		return fmt.Errorf("retention period must be at least 1 day")
	}

	retention := durationpb.New(time.Duration(req.RetentionDays) * 24 * time.Hour)

	_, err := c.client.WorkflowService().RegisterNamespace(ctx, &workflowservice.RegisterNamespaceRequest{
		Namespace:                        req.Name,
		Description:                      req.Description,
		OwnerEmail:                       req.OwnerEmail,
		WorkflowExecutionRetentionPeriod: retention,
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	return nil
}

// DescribeNamespace returns detailed information about a namespace.
func (c *Client) DescribeNamespace(ctx context.Context, name string) (*NamespaceDetail, error) {
	resp, err := c.client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{
		Namespace: name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe namespace: %w", err)
	}

	info := resp.GetNamespaceInfo()
	config := resp.GetConfig()
	replication := resp.GetReplicationConfig()

	retention := "N/A"
	if config.GetWorkflowExecutionRetentionTtl() != nil {
		retention = formatDuration(config.GetWorkflowExecutionRetentionTtl())
	}

	// Format archival info
	historyArchival := formatArchivalState(config.GetHistoryArchivalState(), config.GetHistoryArchivalUri())
	visibilityArchival := formatArchivalState(config.GetVisibilityArchivalState(), config.GetVisibilityArchivalUri())

	// Extract cluster names
	var clusters []string
	for _, cluster := range replication.GetClusters() {
		clusters = append(clusters, cluster.GetClusterName())
	}

	detail := &NamespaceDetail{
		Namespace: Namespace{
			Name:            info.GetName(),
			State:           MapNamespaceState(info.GetState()),
			RetentionPeriod: retention,
			Description:     info.GetDescription(),
			OwnerEmail:      info.GetOwnerEmail(),
		},
		ID:                 info.GetId(),
		IsGlobalNamespace:  resp.GetIsGlobalNamespace(),
		FailoverVersion:    resp.GetFailoverVersion(),
		HistoryArchival:    historyArchival,
		VisibilityArchival: visibilityArchival,
		Clusters:           clusters,
	}

	// Parse timestamps if available
	if info.GetData() != nil {
		// Note: CreatedAt and UpdatedAt are not directly exposed in the API response
		// They would need to be extracted from namespace info data if stored there
	}

	return detail, nil
}

// UpdateNamespace modifies an existing namespace's configuration.
func (c *Client) UpdateNamespace(ctx context.Context, req NamespaceUpdateRequest) error {
	// First describe to get current state
	current, err := c.client.WorkflowService().DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{
		Namespace: req.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to get current namespace config: %w", err)
	}

	// Build update request preserving existing values where not specified
	updateReq := &workflowservice.UpdateNamespaceRequest{
		Namespace: req.Name,
	}

	// Update info fields
	description := req.Description
	ownerEmail := req.OwnerEmail
	if description == "" {
		description = current.GetNamespaceInfo().GetDescription()
	}
	if ownerEmail == "" {
		ownerEmail = current.GetNamespaceInfo().GetOwnerEmail()
	}
	updateReq.UpdateInfo = &namespacepb.UpdateNamespaceInfo{
		Description: description,
		OwnerEmail:  ownerEmail,
	}

	// Update config if retention specified
	if req.RetentionDays > 0 {
		updateReq.Config = &namespacepb.NamespaceConfig{
			WorkflowExecutionRetentionTtl: durationpb.New(time.Duration(req.RetentionDays) * 24 * time.Hour),
		}
	}

	_, err = c.client.WorkflowService().UpdateNamespace(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update namespace: %w", err)
	}
	return nil
}

// DeprecateNamespace marks a namespace as deprecated (soft delete).
func (c *Client) DeprecateNamespace(ctx context.Context, name string) error {
	_, err := c.client.WorkflowService().DeprecateNamespace(ctx, &workflowservice.DeprecateNamespaceRequest{
		Namespace: name,
	})
	if err != nil {
		return fmt.Errorf("failed to deprecate namespace: %w", err)
	}
	return nil
}

// DeleteNamespace permanently deletes a namespace.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	_, err := c.client.OperatorService().DeleteNamespace(ctx, &operatorservice.DeleteNamespaceRequest{
		Namespace: name,
	})
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}
	return nil
}

// formatArchivalState formats archival state and URI for display.
func formatArchivalState(state enums.ArchivalState, uri string) string {
	stateStr := "Disabled"
	switch state {
	case enums.ARCHIVAL_STATE_ENABLED:
		stateStr = "Enabled"
	case enums.ARCHIVAL_STATE_DISABLED:
		stateStr = "Disabled"
	}

	if uri != "" {
		return fmt.Sprintf("%s (%s)", stateStr, uri)
	}
	return stateStr
}

// ListWorkflows returns workflows for a namespace with optional filtering.
func (c *Client) ListWorkflows(ctx context.Context, namespace string, opts ListOptions) ([]Workflow, string, error) {
	if c.client == nil {
		return nil, "", fmt.Errorf("client not connected")
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	req := &workflowservice.ListWorkflowExecutionsRequest{
		Namespace:     namespace,
		PageSize:      int32(pageSize),
		NextPageToken: []byte(opts.PageToken),
	}

	if opts.Query != "" {
		req.Query = opts.Query
	}

	resp, err := c.client.WorkflowService().ListWorkflowExecutions(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list workflows: %w", err)
	}

	var workflows []Workflow
	for _, exec := range resp.GetExecutions() {
		wf := Workflow{
			ID:        exec.GetExecution().GetWorkflowId(),
			RunID:     exec.GetExecution().GetRunId(),
			Type:      exec.GetType().GetName(),
			Status:    MapWorkflowStatus(exec.GetStatus()),
			Namespace: namespace,
			TaskQueue: exec.GetTaskQueue(),
			StartTime: exec.GetStartTime().AsTime(),
		}

		if exec.GetCloseTime() != nil && !exec.GetCloseTime().AsTime().IsZero() {
			t := exec.GetCloseTime().AsTime()
			wf.EndTime = &t
		}

		if exec.GetParentExecution() != nil && exec.GetParentExecution().GetWorkflowId() != "" {
			parentID := exec.GetParentExecution().GetWorkflowId()
			wf.ParentID = &parentID
		}

		// Extract memo if present
		if exec.GetMemo() != nil && exec.GetMemo().GetFields() != nil {
			wf.Memo = make(map[string]string)
			for k, v := range exec.GetMemo().GetFields() {
				// Try to extract string value from payload
				if v != nil && v.GetData() != nil {
					var strVal string
					if err := json.Unmarshal(v.GetData(), &strVal); err == nil {
						wf.Memo[k] = strVal
					} else {
						wf.Memo[k] = string(v.GetData())
					}
				}
			}
		}

		workflows = append(workflows, wf)
	}

	return workflows, string(resp.GetNextPageToken()), nil
}

// GetWorkflow returns details for a specific workflow execution.
func (c *Client) GetWorkflow(ctx context.Context, namespace, workflowID, runID string) (*Workflow, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	resp, err := c.client.WorkflowService().DescribeWorkflowExecution(ctx, &workflowservice.DescribeWorkflowExecutionRequest{
		Namespace: namespace,
		Execution: &commonpb.WorkflowExecution{
			WorkflowId: workflowID,
			RunId:      runID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe workflow: %w", err)
	}

	info := resp.GetWorkflowExecutionInfo()
	wf := &Workflow{
		ID:        info.GetExecution().GetWorkflowId(),
		RunID:     info.GetExecution().GetRunId(),
		Type:      info.GetType().GetName(),
		Status:    MapWorkflowStatus(info.GetStatus()),
		Namespace: namespace,
		TaskQueue: info.GetTaskQueue(),
		StartTime: info.GetStartTime().AsTime(),
	}

	if info.GetCloseTime() != nil && !info.GetCloseTime().AsTime().IsZero() {
		t := info.GetCloseTime().AsTime()
		wf.EndTime = &t
	}

	if info.GetParentExecution() != nil && info.GetParentExecution().GetWorkflowId() != "" {
		parentID := info.GetParentExecution().GetWorkflowId()
		wf.ParentID = &parentID
	}

	// Note: Input/Output are populated separately from event history
	// to avoid redundant API calls. See workflow_detail.go loadData().

	return wf, nil
}

// GetWorkflowHistory returns the event history for a workflow execution.
func (c *Client) GetWorkflowHistory(ctx context.Context, namespace, workflowID, runID string) ([]HistoryEvent, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	var events []HistoryEvent
	var nextPageToken []byte

	for {
		resp, err := c.client.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
			Namespace: namespace,
			Execution: &commonpb.WorkflowExecution{
				WorkflowId: workflowID,
				RunId:      runID,
			},
			NextPageToken: nextPageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow history: %w", err)
		}

		for _, event := range resp.GetHistory().GetEvents() {
			he := HistoryEvent{
				ID:      event.GetEventId(),
				Type:    formatEventType(event.GetEventType().String()),
				Time:    event.GetEventTime().AsTime(),
				Details: extractEventDetails(event),
			}
			events = append(events, he)
		}

		nextPageToken = resp.GetNextPageToken()
		if len(nextPageToken) == 0 {
			break
		}
	}

	return events, nil
}

// GetEnhancedWorkflowHistory returns event history with relational data for tree/timeline views.
func (c *Client) GetEnhancedWorkflowHistory(ctx context.Context, namespace, workflowID, runID string) ([]EnhancedHistoryEvent, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	var events []EnhancedHistoryEvent
	var nextPageToken []byte

	for {
		resp, err := c.client.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
			Namespace: namespace,
			Execution: &commonpb.WorkflowExecution{
				WorkflowId: workflowID,
				RunId:      runID,
			},
			NextPageToken: nextPageToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow history: %w", err)
		}

		for _, event := range resp.GetHistory().GetEvents() {
			he := extractEnhancedEvent(event)
			events = append(events, he)
		}

		nextPageToken = resp.GetNextPageToken()
		if len(nextPageToken) == 0 {
			break
		}
	}

	return events, nil
}

// extractEnhancedEvent extracts structured data from a history event for tree/timeline views.
func extractEnhancedEvent(event *historypb.HistoryEvent) EnhancedHistoryEvent {
	he := EnhancedHistoryEvent{
		ID:      event.GetEventId(),
		Type:    formatEventType(event.GetEventType().String()),
		Time:    event.GetEventTime().AsTime(),
		Details: extractEventDetails(event),
	}

	switch event.GetEventType() {
	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED:
		attrs := event.GetWorkflowExecutionStartedEventAttributes()
		if attrs != nil {
			if attrs.GetTaskQueue() != nil {
				he.TaskQueue = attrs.GetTaskQueue().GetName()
			}
			if attrs.GetIdentity() != "" {
				he.Identity = attrs.GetIdentity()
			}
			he.Attempt = attrs.GetAttempt()
			if attrs.GetInput() != nil {
				he.Input = formatPayloads(attrs.GetInput())
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED:
		attrs := event.GetWorkflowExecutionCompletedEventAttributes()
		if attrs != nil && attrs.GetResult() != nil {
			he.Result = formatPayloads(attrs.GetResult())
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED:
		attrs := event.GetWorkflowExecutionFailedEventAttributes()
		if attrs != nil && attrs.GetFailure() != nil {
			he.Failure = attrs.GetFailure().GetMessage()
			if attrs.GetFailure().GetStackTrace() != "" {
				he.Failure += "\n\nStack Trace:\n" + attrs.GetFailure().GetStackTrace()
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_CANCELED:
		attrs := event.GetWorkflowExecutionCanceledEventAttributes()
		if attrs != nil && attrs.GetDetails() != nil {
			he.Result = formatPayloads(attrs.GetDetails())
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_TERMINATED:
		attrs := event.GetWorkflowExecutionTerminatedEventAttributes()
		if attrs != nil && attrs.GetReason() != "" {
			he.Failure = attrs.GetReason()
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_TIMED_OUT:
		he.Failure = "Workflow timed out"

	case enums.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED:
		attrs := event.GetWorkflowTaskScheduledEventAttributes()
		if attrs != nil && attrs.GetTaskQueue() != nil {
			he.TaskQueue = attrs.GetTaskQueue().GetName()
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_STARTED:
		attrs := event.GetWorkflowTaskStartedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.Identity = attrs.GetIdentity()
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_COMPLETED:
		attrs := event.GetWorkflowTaskCompletedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
			he.Identity = attrs.GetIdentity()
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT:
		attrs := event.GetWorkflowTaskTimedOutEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_FAILED:
		attrs := event.GetWorkflowTaskFailedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			if attrs.GetFailure() != nil {
				he.Failure = attrs.GetFailure().GetMessage()
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
		attrs := event.GetActivityTaskScheduledEventAttributes()
		if attrs != nil {
			he.ActivityID = attrs.GetActivityId()
			if attrs.GetActivityType() != nil {
				he.ActivityType = attrs.GetActivityType().GetName()
			}
			if attrs.GetTaskQueue() != nil {
				he.TaskQueue = attrs.GetTaskQueue().GetName()
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_STARTED:
		attrs := event.GetActivityTaskStartedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.Attempt = attrs.GetAttempt()
			he.Identity = attrs.GetIdentity()
			if attrs.GetLastFailure() != nil {
				he.Failure = attrs.GetLastFailure().GetMessage()
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED:
		attrs := event.GetActivityTaskCompletedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
			he.Identity = attrs.GetIdentity()
			if attrs.GetResult() != nil {
				he.Result = formatPayloads(attrs.GetResult())
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_FAILED:
		attrs := event.GetActivityTaskFailedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
			if attrs.GetFailure() != nil {
				he.Failure = attrs.GetFailure().GetMessage()
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_TIMED_OUT:
		attrs := event.GetActivityTaskTimedOutEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
			if attrs.GetFailure() != nil {
				he.Failure = attrs.GetFailure().GetMessage()
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_CANCEL_REQUESTED:
		attrs := event.GetActivityTaskCancelRequestedEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_CANCELED:
		attrs := event.GetActivityTaskCanceledEventAttributes()
		if attrs != nil {
			he.ScheduledEventID = attrs.GetScheduledEventId()
			he.StartedEventID = attrs.GetStartedEventId()
		}

	case enums.EVENT_TYPE_TIMER_STARTED:
		attrs := event.GetTimerStartedEventAttributes()
		if attrs != nil {
			he.TimerID = attrs.GetTimerId()
		}

	case enums.EVENT_TYPE_TIMER_FIRED:
		attrs := event.GetTimerFiredEventAttributes()
		if attrs != nil {
			he.TimerID = attrs.GetTimerId()
			he.StartedEventID = attrs.GetStartedEventId()
		}

	case enums.EVENT_TYPE_TIMER_CANCELED:
		attrs := event.GetTimerCanceledEventAttributes()
		if attrs != nil {
			he.TimerID = attrs.GetTimerId()
			he.StartedEventID = attrs.GetStartedEventId()
		}

	case enums.EVENT_TYPE_START_CHILD_WORKFLOW_EXECUTION_INITIATED:
		attrs := event.GetStartChildWorkflowExecutionInitiatedEventAttributes()
		if attrs != nil {
			he.ChildWorkflowID = attrs.GetWorkflowId()
			if attrs.GetWorkflowType() != nil {
				he.ChildWorkflowType = attrs.GetWorkflowType().GetName()
			}
			if attrs.GetTaskQueue() != nil {
				he.TaskQueue = attrs.GetTaskQueue().GetName()
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_STARTED:
		attrs := event.GetChildWorkflowExecutionStartedEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
			if attrs.GetWorkflowType() != nil {
				he.ChildWorkflowType = attrs.GetWorkflowType().GetName()
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_COMPLETED:
		attrs := event.GetChildWorkflowExecutionCompletedEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
			if attrs.GetResult() != nil {
				he.Result = formatPayloads(attrs.GetResult())
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_FAILED:
		attrs := event.GetChildWorkflowExecutionFailedEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
			if attrs.GetFailure() != nil {
				he.Failure = attrs.GetFailure().GetMessage()
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_CANCELED:
		attrs := event.GetChildWorkflowExecutionCanceledEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_TIMED_OUT:
		attrs := event.GetChildWorkflowExecutionTimedOutEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_TERMINATED:
		attrs := event.GetChildWorkflowExecutionTerminatedEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
				he.ChildRunID = attrs.GetWorkflowExecution().GetRunId()
			}
		}

	case enums.EVENT_TYPE_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_INITIATED:
		attrs := event.GetSignalExternalWorkflowExecutionInitiatedEventAttributes()
		if attrs != nil && attrs.GetWorkflowExecution() != nil {
			he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
		}

	case enums.EVENT_TYPE_EXTERNAL_WORKFLOW_EXECUTION_SIGNALED:
		attrs := event.GetExternalWorkflowExecutionSignaledEventAttributes()
		if attrs != nil {
			he.InitiatedEventID = attrs.GetInitiatedEventId()
			if attrs.GetWorkflowExecution() != nil {
				he.ChildWorkflowID = attrs.GetWorkflowExecution().GetWorkflowId()
			}
		}
	}

	return he
}

// formatEventType cleans up the event type string for display
func formatEventType(eventType string) string {
	// Remove EVENT_TYPE_ prefix if present (older protobuf format)
	eventType = strings.TrimPrefix(eventType, "EVENT_TYPE_")

	// If it contains underscores, convert from SCREAMING_SNAKE_CASE to PascalCase
	if strings.Contains(eventType, "_") {
		parts := strings.Split(strings.ToLower(eventType), "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
		return strings.Join(parts, "")
	}

	// Otherwise it's already in a readable format (e.g., WorkflowExecutionStarted)
	return eventType
}

// extractEventDetails extracts a verbose summary string from a history event.
func extractEventDetails(event *historypb.HistoryEvent) string {
	var details []string

	switch event.GetEventType() {
	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED:
		attrs := event.GetWorkflowExecutionStartedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowType() != nil {
				details = append(details, fmt.Sprintf("WorkflowType: %s", attrs.GetWorkflowType().GetName()))
			}
			if attrs.GetTaskQueue() != nil {
				details = append(details, fmt.Sprintf("TaskQueue: %s", attrs.GetTaskQueue().GetName()))
			}
			if attrs.GetInput() != nil {
				details = append(details, fmt.Sprintf("Input: %s", formatPayloads(attrs.GetInput())))
			}
			if attrs.GetWorkflowExecutionTimeout() != nil {
				details = append(details, fmt.Sprintf("ExecutionTimeout: %s", attrs.GetWorkflowExecutionTimeout().AsDuration()))
			}
			if attrs.GetWorkflowRunTimeout() != nil {
				details = append(details, fmt.Sprintf("RunTimeout: %s", attrs.GetWorkflowRunTimeout().AsDuration()))
			}
			if attrs.GetWorkflowTaskTimeout() != nil {
				details = append(details, fmt.Sprintf("TaskTimeout: %s", attrs.GetWorkflowTaskTimeout().AsDuration()))
			}
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
			if attrs.GetAttempt() > 1 {
				details = append(details, fmt.Sprintf("Attempt: %d", attrs.GetAttempt()))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED:
		attrs := event.GetWorkflowExecutionCompletedEventAttributes()
		if attrs != nil {
			if attrs.GetResult() != nil {
				details = append(details, fmt.Sprintf("Result: %s", formatPayloads(attrs.GetResult())))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED:
		attrs := event.GetWorkflowExecutionFailedEventAttributes()
		if attrs != nil {
			if attrs.GetFailure() != nil {
				details = append(details, fmt.Sprintf("Failure: %s", attrs.GetFailure().GetMessage()))
				if attrs.GetFailure().GetStackTrace() != "" {
					// Truncate stack trace for display
					trace := attrs.GetFailure().GetStackTrace()
					if len(trace) > 200 {
						trace = trace[:200] + "..."
					}
					details = append(details, fmt.Sprintf("StackTrace: %s", trace))
				}
			}
			details = append(details, fmt.Sprintf("RetryState: %s", attrs.GetRetryState().String()))
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_TIMED_OUT:
		attrs := event.GetWorkflowExecutionTimedOutEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("RetryState: %s", attrs.GetRetryState().String()))
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_CANCELED:
		attrs := event.GetWorkflowExecutionCanceledEventAttributes()
		if attrs != nil {
			if attrs.GetDetails() != nil {
				details = append(details, fmt.Sprintf("Details: %s", formatPayloads(attrs.GetDetails())))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_TERMINATED:
		attrs := event.GetWorkflowExecutionTerminatedEventAttributes()
		if attrs != nil {
			if attrs.GetReason() != "" {
				details = append(details, fmt.Sprintf("Reason: %s", attrs.GetReason()))
			}
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED:
		attrs := event.GetWorkflowTaskScheduledEventAttributes()
		if attrs != nil {
			if attrs.GetTaskQueue() != nil {
				details = append(details, fmt.Sprintf("TaskQueue: %s", attrs.GetTaskQueue().GetName()))
			}
			if attrs.GetStartToCloseTimeout() != nil {
				details = append(details, fmt.Sprintf("StartToCloseTimeout: %s", attrs.GetStartToCloseTimeout().AsDuration()))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_STARTED:
		attrs := event.GetWorkflowTaskStartedEventAttributes()
		if attrs != nil {
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_COMPLETED:
		attrs := event.GetWorkflowTaskCompletedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT:
		attrs := event.GetWorkflowTaskTimedOutEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			details = append(details, fmt.Sprintf("TimeoutType: %s", attrs.GetTimeoutType().String()))
		}

	case enums.EVENT_TYPE_WORKFLOW_TASK_FAILED:
		attrs := event.GetWorkflowTaskFailedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("Cause: %s", attrs.GetCause().String()))
			if attrs.GetFailure() != nil {
				details = append(details, fmt.Sprintf("Failure: %s", attrs.GetFailure().GetMessage()))
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
		attrs := event.GetActivityTaskScheduledEventAttributes()
		if attrs != nil {
			if attrs.GetActivityType() != nil {
				details = append(details, fmt.Sprintf("ActivityType: %s", attrs.GetActivityType().GetName()))
			}
			if attrs.GetActivityId() != "" {
				details = append(details, fmt.Sprintf("ActivityId: %s", attrs.GetActivityId()))
			}
			if attrs.GetTaskQueue() != nil {
				details = append(details, fmt.Sprintf("TaskQueue: %s", attrs.GetTaskQueue().GetName()))
			}
			if attrs.GetInput() != nil {
				details = append(details, fmt.Sprintf("Input: %s", formatPayloads(attrs.GetInput())))
			}
			if attrs.GetScheduleToCloseTimeout() != nil {
				details = append(details, fmt.Sprintf("ScheduleToCloseTimeout: %s", attrs.GetScheduleToCloseTimeout().AsDuration()))
			}
			if attrs.GetScheduleToStartTimeout() != nil {
				details = append(details, fmt.Sprintf("ScheduleToStartTimeout: %s", attrs.GetScheduleToStartTimeout().AsDuration()))
			}
			if attrs.GetStartToCloseTimeout() != nil {
				details = append(details, fmt.Sprintf("StartToCloseTimeout: %s", attrs.GetStartToCloseTimeout().AsDuration()))
			}
			if attrs.GetRetryPolicy() != nil {
				rp := attrs.GetRetryPolicy()
				details = append(details, fmt.Sprintf("RetryPolicy: MaxAttempts=%d", rp.GetMaximumAttempts()))
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_STARTED:
		attrs := event.GetActivityTaskStartedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("Attempt: %d", attrs.GetAttempt()))
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED:
		attrs := event.GetActivityTaskCompletedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			if attrs.GetResult() != nil {
				details = append(details, fmt.Sprintf("Result: %s", formatPayloads(attrs.GetResult())))
			}
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_FAILED:
		attrs := event.GetActivityTaskFailedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			if attrs.GetFailure() != nil {
				details = append(details, fmt.Sprintf("Failure: %s", attrs.GetFailure().GetMessage()))
			}
			details = append(details, fmt.Sprintf("RetryState: %s", attrs.GetRetryState().String()))
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_TIMED_OUT:
		attrs := event.GetActivityTaskTimedOutEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			if attrs.GetFailure() != nil {
				details = append(details, fmt.Sprintf("TimeoutType: %s", attrs.GetFailure().GetMessage()))
			}
			details = append(details, fmt.Sprintf("RetryState: %s", attrs.GetRetryState().String()))
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_CANCEL_REQUESTED:
		attrs := event.GetActivityTaskCancelRequestedEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
		}

	case enums.EVENT_TYPE_ACTIVITY_TASK_CANCELED:
		attrs := event.GetActivityTaskCanceledEventAttributes()
		if attrs != nil {
			details = append(details, fmt.Sprintf("ScheduledEventId: %d", attrs.GetScheduledEventId()))
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
			if attrs.GetDetails() != nil {
				details = append(details, fmt.Sprintf("Details: %s", formatPayloads(attrs.GetDetails())))
			}
		}

	case enums.EVENT_TYPE_TIMER_STARTED:
		attrs := event.GetTimerStartedEventAttributes()
		if attrs != nil {
			if attrs.GetTimerId() != "" {
				details = append(details, fmt.Sprintf("TimerId: %s", attrs.GetTimerId()))
			}
			if attrs.GetStartToFireTimeout() != nil {
				details = append(details, fmt.Sprintf("StartToFireTimeout: %s", attrs.GetStartToFireTimeout().AsDuration()))
			}
		}

	case enums.EVENT_TYPE_TIMER_FIRED:
		attrs := event.GetTimerFiredEventAttributes()
		if attrs != nil {
			if attrs.GetTimerId() != "" {
				details = append(details, fmt.Sprintf("TimerId: %s", attrs.GetTimerId()))
			}
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
		}

	case enums.EVENT_TYPE_TIMER_CANCELED:
		attrs := event.GetTimerCanceledEventAttributes()
		if attrs != nil {
			if attrs.GetTimerId() != "" {
				details = append(details, fmt.Sprintf("TimerId: %s", attrs.GetTimerId()))
			}
			details = append(details, fmt.Sprintf("StartedEventId: %d", attrs.GetStartedEventId()))
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_SIGNALED:
		attrs := event.GetWorkflowExecutionSignaledEventAttributes()
		if attrs != nil {
			if attrs.GetSignalName() != "" {
				details = append(details, fmt.Sprintf("SignalName: %s", attrs.GetSignalName()))
			}
			if attrs.GetInput() != nil {
				details = append(details, fmt.Sprintf("Input: %s", formatPayloads(attrs.GetInput())))
			}
			if attrs.GetIdentity() != "" {
				details = append(details, fmt.Sprintf("Identity: %s", attrs.GetIdentity()))
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_ACCEPTED:
		attrs := event.GetWorkflowExecutionUpdateAcceptedEventAttributes()
		if attrs != nil {
			if attrs.GetAcceptedRequest() != nil {
				if attrs.GetAcceptedRequest().GetMeta() != nil {
					details = append(details, fmt.Sprintf("UpdateId: %s", attrs.GetAcceptedRequest().GetMeta().GetUpdateId()))
				}
			}
		}

	case enums.EVENT_TYPE_WORKFLOW_EXECUTION_UPDATE_COMPLETED:
		attrs := event.GetWorkflowExecutionUpdateCompletedEventAttributes()
		if attrs != nil {
			if attrs.GetMeta() != nil {
				details = append(details, fmt.Sprintf("UpdateId: %s", attrs.GetMeta().GetUpdateId()))
			}
		}

	case enums.EVENT_TYPE_START_CHILD_WORKFLOW_EXECUTION_INITIATED:
		attrs := event.GetStartChildWorkflowExecutionInitiatedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowType() != nil {
				details = append(details, fmt.Sprintf("WorkflowType: %s", attrs.GetWorkflowType().GetName()))
			}
			if attrs.GetWorkflowId() != "" {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowId()))
			}
			if attrs.GetTaskQueue() != nil {
				details = append(details, fmt.Sprintf("TaskQueue: %s", attrs.GetTaskQueue().GetName()))
			}
			if attrs.GetInput() != nil {
				details = append(details, fmt.Sprintf("Input: %s", formatPayloads(attrs.GetInput())))
			}
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_STARTED:
		attrs := event.GetChildWorkflowExecutionStartedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowType() != nil {
				details = append(details, fmt.Sprintf("WorkflowType: %s", attrs.GetWorkflowType().GetName()))
			}
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
				details = append(details, fmt.Sprintf("RunId: %s", attrs.GetWorkflowExecution().GetRunId()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_COMPLETED:
		attrs := event.GetChildWorkflowExecutionCompletedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			if attrs.GetResult() != nil {
				details = append(details, fmt.Sprintf("Result: %s", formatPayloads(attrs.GetResult())))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_FAILED:
		attrs := event.GetChildWorkflowExecutionFailedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			if attrs.GetFailure() != nil {
				details = append(details, fmt.Sprintf("Failure: %s", attrs.GetFailure().GetMessage()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_CANCELED:
		attrs := event.GetChildWorkflowExecutionCanceledEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_TIMED_OUT:
		attrs := event.GetChildWorkflowExecutionTimedOutEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_TERMINATED:
		attrs := event.GetChildWorkflowExecutionTerminatedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_MARKER_RECORDED:
		attrs := event.GetMarkerRecordedEventAttributes()
		if attrs != nil {
			if attrs.GetMarkerName() != "" {
				details = append(details, fmt.Sprintf("MarkerName: %s", attrs.GetMarkerName()))
			}
		}

	case enums.EVENT_TYPE_EXTERNAL_WORKFLOW_EXECUTION_SIGNALED:
		attrs := event.GetExternalWorkflowExecutionSignaledEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			details = append(details, fmt.Sprintf("InitiatedEventId: %d", attrs.GetInitiatedEventId()))
		}

	case enums.EVENT_TYPE_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_INITIATED:
		attrs := event.GetSignalExternalWorkflowExecutionInitiatedEventAttributes()
		if attrs != nil {
			if attrs.GetWorkflowExecution() != nil {
				details = append(details, fmt.Sprintf("WorkflowId: %s", attrs.GetWorkflowExecution().GetWorkflowId()))
			}
			if attrs.GetSignalName() != "" {
				details = append(details, fmt.Sprintf("SignalName: %s", attrs.GetSignalName()))
			}
			if attrs.GetInput() != nil {
				details = append(details, fmt.Sprintf("Input: %s", formatPayloads(attrs.GetInput())))
			}
		}

	default:
		// For unhandled event types, return event type name
		details = append(details, fmt.Sprintf("EventType: %s", event.GetEventType().String()))
	}

	return strings.Join(details, ", ")
}

// formatPayloads formats payloads for display
func formatPayloads(payloads *commonpb.Payloads) string {
	if payloads == nil {
		return ""
	}

	var results []string
	for _, p := range payloads.GetPayloads() {
		if p == nil {
			continue
		}
		data := p.GetData()
		if len(data) == 0 {
			continue
		}

		// Try to parse as JSON for nicer display
		var jsonVal interface{}
		if err := json.Unmarshal(data, &jsonVal); err == nil {
			// Format as compact JSON
			if b, err := json.Marshal(jsonVal); err == nil {
				results = append(results, string(b))
				continue
			}
		}

		// Fall back to raw string (truncated)
		s := string(data)
		if len(s) > 100 {
			s = s[:100] + "..."
		}
		results = append(results, s)
	}

	return strings.Join(results, ", ")
}

// DescribeTaskQueue returns task queue info and active pollers.
func (c *Client) DescribeTaskQueue(ctx context.Context, namespace, taskQueue string) (*TaskQueueInfo, []Poller, error) {
	// Query workflow task queue
	wfResp, err := c.client.WorkflowService().DescribeTaskQueue(ctx, &workflowservice.DescribeTaskQueueRequest{
		Namespace: namespace,
		TaskQueue: &taskqueue.TaskQueue{
			Name: taskQueue,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
		TaskQueueType: enums.TASK_QUEUE_TYPE_WORKFLOW,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe workflow task queue: %w", err)
	}

	// Query activity task queue
	actResp, err := c.client.WorkflowService().DescribeTaskQueue(ctx, &workflowservice.DescribeTaskQueueRequest{
		Namespace: namespace,
		TaskQueue: &taskqueue.TaskQueue{
			Name: taskQueue,
			Kind: enums.TASK_QUEUE_KIND_NORMAL,
		},
		TaskQueueType: enums.TASK_QUEUE_TYPE_ACTIVITY,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe activity task queue: %w", err)
	}

	// Combine poller info
	var pollers []Poller

	for _, p := range wfResp.GetPollers() {
		pollers = append(pollers, Poller{
			Identity:       p.GetIdentity(),
			LastAccessTime: p.GetLastAccessTime().AsTime(),
			TaskQueueType:  TaskQueueTypeWorkflow,
			RatePerSecond:  p.GetRatePerSecond(),
		})
	}

	for _, p := range actResp.GetPollers() {
		pollers = append(pollers, Poller{
			Identity:       p.GetIdentity(),
			LastAccessTime: p.GetLastAccessTime().AsTime(),
			TaskQueueType:  TaskQueueTypeActivity,
			RatePerSecond:  p.GetRatePerSecond(),
		})
	}

	info := &TaskQueueInfo{
		Name:        taskQueue,
		Type:        "Combined",
		PollerCount: len(pollers),
		Backlog:     0, // Backlog info requires enhanced visibility or approximation
	}

	return info, pollers, nil
}

// formatDuration formats a protobuf duration as a human-readable string.
func formatDuration(d *durationpb.Duration) string {
	if d == nil {
		return "N/A"
	}

	dur := d.AsDuration()

	if dur < time.Hour {
		return fmt.Sprintf("%d minutes", int(dur.Minutes()))
	}
	if dur < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(dur.Hours()))
	}

	days := int(dur.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// CancelWorkflow requests graceful cancellation of a workflow execution.
func (c *Client) CancelWorkflow(ctx context.Context, namespace, workflowID, runID, reason string) error {
	return c.client.CancelWorkflow(ctx, workflowID, runID)
}

// TerminateWorkflow forcefully terminates a workflow execution immediately.
func (c *Client) TerminateWorkflow(ctx context.Context, namespace, workflowID, runID, reason string) error {
	return c.client.TerminateWorkflow(ctx, workflowID, runID, reason)
}

// SignalWorkflow sends a signal to a running workflow execution.
func (c *Client) SignalWorkflow(ctx context.Context, namespace, workflowID, runID, signalName string, input []byte) error {
	return c.client.SignalWorkflow(ctx, workflowID, runID, signalName, input)
}

// SignalWithStartWorkflow starts a workflow if it doesn't exist and sends a signal to it.
func (c *Client) SignalWithStartWorkflow(ctx context.Context, namespace string, req SignalWithStartRequest) (string, error) {
	opts := client.StartWorkflowOptions{
		ID:        req.WorkflowID,
		TaskQueue: req.TaskQueue,
	}

	run, err := c.client.SignalWithStartWorkflow(
		ctx,
		req.WorkflowID,
		req.SignalName,
		req.SignalInput,
		opts,
		req.WorkflowType,
		req.WorkflowInput,
	)
	if err != nil {
		return "", fmt.Errorf("failed to signal with start workflow: %w", err)
	}
	return run.GetRunID(), nil
}

// DeleteWorkflow permanently deletes a workflow execution and its history.
func (c *Client) DeleteWorkflow(ctx context.Context, namespace, workflowID, runID string) error {
	_, err := c.client.WorkflowService().DeleteWorkflowExecution(ctx,
		&workflowservice.DeleteWorkflowExecutionRequest{
			Namespace: namespace,
			WorkflowExecution: &commonpb.WorkflowExecution{
				WorkflowId: workflowID,
				RunId:      runID,
			},
		})
	return err
}

// ResetWorkflow resets a workflow to a previous state, creating a new run.
func (c *Client) ResetWorkflow(ctx context.Context, namespace, workflowID, runID string, eventID int64, reason string) (string, error) {
	resp, err := c.client.WorkflowService().ResetWorkflowExecution(ctx, &workflowservice.ResetWorkflowExecutionRequest{
		Namespace: namespace,
		WorkflowExecution: &commonpb.WorkflowExecution{
			WorkflowId: workflowID,
			RunId:      runID,
		},
		Reason:                    reason,
		WorkflowTaskFinishEventId: eventID,
	})
	if err != nil {
		return "", err
	}
	return resp.GetRunId(), nil
}

// ListSchedules returns all schedules in a namespace.
func (c *Client) ListSchedules(ctx context.Context, namespace string, opts ListOptions) ([]Schedule, string, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	resp, err := c.client.ScheduleClient().List(ctx, client.ScheduleListOptions{
		PageSize: pageSize,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to list schedules: %w", err)
	}

	var schedules []Schedule
	for resp.HasNext() {
		entry, err := resp.Next()
		if err != nil {
			return nil, "", fmt.Errorf("failed to iterate schedules: %w", err)
		}

		schedule := Schedule{
			ID:           entry.ID,
			Paused:       entry.Paused,
			Notes:        entry.Note,
			WorkflowType: entry.WorkflowType.Name,
		}

		// Extract spec info
		if entry.Spec != nil && len(entry.Spec.Intervals) > 0 {
			schedule.Spec = formatScheduleSpec(entry.Spec)
		}

		// Recent and future actions
		if len(entry.RecentActions) > 0 {
			lastAction := entry.RecentActions[len(entry.RecentActions)-1]
			t := lastAction.ActualTime
			schedule.LastRunTime = &t
		}
		if len(entry.NextActionTimes) > 0 {
			t := entry.NextActionTimes[0]
			schedule.NextRunTime = &t
		}

		schedules = append(schedules, schedule)
	}

	return schedules, "", nil
}

// GetSchedule returns details for a specific schedule.
func (c *Client) GetSchedule(ctx context.Context, namespace, scheduleID string) (*Schedule, error) {
	handle := c.client.ScheduleClient().GetHandle(ctx, scheduleID)
	desc, err := handle.Describe(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to describe schedule: %w", err)
	}

	schedule := &Schedule{
		ID:     scheduleID,
		Paused: desc.Schedule.State.Paused,
		Notes:  desc.Schedule.State.Note,
	}

	// Extract workflow info from action
	if desc.Schedule.Action != nil {
		if startAction, ok := desc.Schedule.Action.(*client.ScheduleWorkflowAction); ok {
			// Workflow is an interface{} representing the workflow type
			if wfType, ok := startAction.Workflow.(string); ok {
				schedule.WorkflowType = wfType
			}
			schedule.WorkflowID = startAction.ID
			schedule.TaskQueue = startAction.TaskQueue
		}
	}

	// Extract spec info
	if desc.Schedule.Spec != nil {
		schedule.Spec = formatScheduleSpec(desc.Schedule.Spec)
	}

	// Info from description
	schedule.TotalActions = int64(desc.Info.NumActions)
	if len(desc.Info.RecentActions) > 0 {
		lastAction := desc.Info.RecentActions[len(desc.Info.RecentActions)-1]
		t := lastAction.ActualTime
		schedule.LastRunTime = &t
	}
	if len(desc.Info.NextActionTimes) > 0 {
		t := desc.Info.NextActionTimes[0]
		schedule.NextRunTime = &t
	}

	return schedule, nil
}

// PauseSchedule pauses a schedule.
func (c *Client) PauseSchedule(ctx context.Context, namespace, scheduleID, reason string) error {
	handle := c.client.ScheduleClient().GetHandle(ctx, scheduleID)
	return handle.Pause(ctx, client.SchedulePauseOptions{
		Note: reason,
	})
}

// UnpauseSchedule unpauses a schedule.
func (c *Client) UnpauseSchedule(ctx context.Context, namespace, scheduleID, reason string) error {
	handle := c.client.ScheduleClient().GetHandle(ctx, scheduleID)
	return handle.Unpause(ctx, client.ScheduleUnpauseOptions{
		Note: reason,
	})
}

// TriggerSchedule immediately triggers a scheduled workflow execution.
func (c *Client) TriggerSchedule(ctx context.Context, namespace, scheduleID string) error {
	handle := c.client.ScheduleClient().GetHandle(ctx, scheduleID)
	return handle.Trigger(ctx, client.ScheduleTriggerOptions{})
}

// DeleteSchedule permanently deletes a schedule.
func (c *Client) DeleteSchedule(ctx context.Context, namespace, scheduleID string) error {
	handle := c.client.ScheduleClient().GetHandle(ctx, scheduleID)
	return handle.Delete(ctx)
}

// formatScheduleSpec creates a human-readable schedule specification.
func formatScheduleSpec(spec *client.ScheduleSpec) string {
	if spec == nil {
		return ""
	}

	var parts []string

	// Check for cron expressions
	if len(spec.CronExpressions) > 0 {
		parts = append(parts, spec.CronExpressions[0])
	}

	// Check for intervals
	if len(spec.Intervals) > 0 {
		interval := spec.Intervals[0]
		parts = append(parts, fmt.Sprintf("every %s", interval.Every))
	}

	// Check for calendars
	if len(spec.Calendars) > 0 {
		parts = append(parts, "calendar-based")
	}

	if len(parts) == 0 {
		return "custom"
	}

	return strings.Join(parts, ", ")
}

// QueryWorkflow executes a query against a running workflow and returns the result.
func (c *Client) QueryWorkflow(ctx context.Context, namespace, workflowID, runID, queryType string, args []byte) (*QueryResult, error) {
	// Build query input if args provided
	var queryArgs interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &queryArgs); err != nil {
			// If not valid JSON, pass as raw string
			queryArgs = string(args)
		}
	}

	// Execute the query
	response, err := c.client.QueryWorkflow(ctx, workflowID, runID, queryType, queryArgs)
	if err != nil {
		return &QueryResult{
			QueryType: queryType,
			Error:     err.Error(),
		}, nil
	}

	// Decode the result
	var result interface{}
	if err := response.Get(&result); err != nil {
		return &QueryResult{
			QueryType: queryType,
			Error:     fmt.Sprintf("failed to decode query result: %v", err),
		}, nil
	}

	// Format result as JSON for display
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &QueryResult{
			QueryType: queryType,
			Result:    fmt.Sprintf("%v", result),
		}, nil
	}

	return &QueryResult{
		QueryType: queryType,
		Result:    string(resultJSON),
	}, nil
}

// CancelWorkflows cancels multiple workflows and returns results for each.
func (c *Client) CancelWorkflows(ctx context.Context, namespace string, workflows []WorkflowIdentifier) ([]BatchResult, error) {
	results := make([]BatchResult, len(workflows))

	for i, wf := range workflows {
		err := c.client.CancelWorkflow(ctx, wf.WorkflowID, wf.RunID)
		results[i] = BatchResult{
			WorkflowID: wf.WorkflowID,
			RunID:      wf.RunID,
			Success:    err == nil,
		}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results, nil
}

// TerminateWorkflows terminates multiple workflows and returns results for each.
func (c *Client) TerminateWorkflows(ctx context.Context, namespace string, workflows []WorkflowIdentifier, reason string) ([]BatchResult, error) {
	results := make([]BatchResult, len(workflows))

	for i, wf := range workflows {
		err := c.client.TerminateWorkflow(ctx, wf.WorkflowID, wf.RunID, reason)
		results[i] = BatchResult{
			WorkflowID: wf.WorkflowID,
			RunID:      wf.RunID,
			Success:    err == nil,
		}
		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results, nil
}

// GetResetPoints returns valid reset points for a workflow execution.
func (c *Client) GetResetPoints(ctx context.Context, namespace, workflowID, runID string) ([]ResetPoint, error) {
	// Get workflow history to find reset points
	events, err := c.GetEnhancedWorkflowHistory(ctx, namespace, workflowID, runID)
	if err != nil {
		return nil, err
	}

	var resetPoints []ResetPoint

	// Track activity/timer state for building descriptions
	activityInfo := make(map[int64]string)  // scheduledEventID -> activity type
	timerInfo := make(map[int64]string)     // startedEventID -> timer ID

	for _, event := range events {
		// Track activity scheduled events
		if strings.Contains(event.Type, "ActivityTaskScheduled") {
			activityInfo[event.ID] = event.ActivityType
		}

		// Track timer started events
		if strings.Contains(event.Type, "TimerStarted") {
			timerInfo[event.ID] = event.TimerID
		}

		// WorkflowTaskCompleted events are valid reset points
		if strings.Contains(event.Type, "WorkflowTaskCompleted") {
			resetPoints = append(resetPoints, ResetPoint{
				EventID:     event.ID,
				EventType:   event.Type,
				Timestamp:   event.Time,
				Description: fmt.Sprintf("Workflow task completed at event %d", event.ID),
				Reason:      "Reset to this workflow task",
			})
		}

		// ActivityTaskFailed - reset to before the failure
		if strings.Contains(event.Type, "ActivityTaskFailed") {
			actType := activityInfo[event.ScheduledEventID]
			if actType == "" {
				actType = "Unknown"
			}
			resetPoints = append(resetPoints, ResetPoint{
				EventID:     event.ScheduledEventID - 1, // Reset to workflow task before activity was scheduled
				EventType:   event.Type,
				Timestamp:   event.Time,
				Description: fmt.Sprintf("Activity '%s' failed: %s", actType, truncateString(event.Failure, 50)),
				Reason:      "Reset to retry failed activity",
			})
		}

		// ActivityTaskTimedOut - reset to before the timeout
		if strings.Contains(event.Type, "ActivityTaskTimedOut") {
			actType := activityInfo[event.ScheduledEventID]
			if actType == "" {
				actType = "Unknown"
			}
			resetPoints = append(resetPoints, ResetPoint{
				EventID:     event.ScheduledEventID - 1,
				EventType:   event.Type,
				Timestamp:   event.Time,
				Description: fmt.Sprintf("Activity '%s' timed out", actType),
				Reason:      "Reset to retry timed out activity",
			})
		}

		// WorkflowTaskFailed - this is a good reset point
		if strings.Contains(event.Type, "WorkflowTaskFailed") {
			resetPoints = append(resetPoints, ResetPoint{
				EventID:     event.ScheduledEventID - 1,
				EventType:   event.Type,
				Timestamp:   event.Time,
				Description: fmt.Sprintf("Workflow task failed: %s", truncateString(event.Failure, 50)),
				Reason:      "Reset to retry failed workflow task",
			})
		}
	}

	return resetPoints, nil
}

// truncateString truncates a string to maxLen and adds ellipsis if needed.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure Client implements Provider
var _ Provider = (*Client)(nil)
