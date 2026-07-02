package command

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/galaxy-io/tempo/internal/temporal"
)

// Context holds the variables available for template expansion.
type Context struct {
	WorkflowID   string
	RunID        string
	WorkflowType string
	Namespace    string
	Address      string
	Profile      string
	Args         []string

	// Connection details for injecting into temporal CLI commands
	TLSCertPath   string
	TLSKeyPath    string
	TLSCAPath     string
	TLSServerName string
	TLSSkipVerify bool
	APIKey        string
}

var placeholderRe = regexp.MustCompile(`\{([^}]+)\}`)

// ExpandCmd replaces {var} placeholders in a command template.
// Returns an error if a required variable is empty.
func ExpandCmd(template string, ctx Context) (string, error) {
	var expandErr error
	result := placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		key := match[1 : len(match)-1] // strip { }
		var val string
		switch key {
		case "workflow_id":
			val = ctx.WorkflowID
		case "run_id":
			val = ctx.RunID
		case "workflow_type":
			val = ctx.WorkflowType
		case "namespace":
			val = ctx.Namespace
		case "address":
			val = ctx.Address
		case "profile":
			val = ctx.Profile
		default:
			// Check positional args: {1}, {2}, etc.
			if n, err := strconv.Atoi(key); err == nil && n >= 1 {
				idx := n - 1
				if idx < len(ctx.Args) {
					val = ctx.Args[idx]
				} else {
					expandErr = fmt.Errorf("positional argument {%s} not provided", key)
					return match
				}
			} else {
				expandErr = fmt.Errorf("unknown variable {%s}", key)
				return match
			}
		}
		if val == "" {
			expandErr = fmt.Errorf("variable {%s} is empty", key)
			return match
		}
		return val
	})
	if expandErr != nil {
		return "", expandErr
	}
	return result, nil
}

// InjectConnectionFlags appends --address, --tls-*, and --api-key flags to
// temporal CLI commands so they connect to the active profile's server.
// Only modifies commands that start with "temporal ".
func InjectConnectionFlags(cmdStr string, ctx Context) string {
	if !strings.HasPrefix(cmdStr, "temporal ") {
		return cmdStr
	}

	var flags []string

	// Only add flags that aren't already present in the command
	if ctx.Address != "" && !strings.Contains(cmdStr, "--address") {
		flags = append(flags, fmt.Sprintf("--address %q", ctx.Address))
	}
	if ctx.TLSCertPath != "" && !strings.Contains(cmdStr, "--tls-cert") {
		flags = append(flags, fmt.Sprintf("--tls-cert-path %q", ctx.TLSCertPath))
	}
	if ctx.TLSKeyPath != "" && !strings.Contains(cmdStr, "--tls-key") {
		flags = append(flags, fmt.Sprintf("--tls-key-path %q", ctx.TLSKeyPath))
	}
	if ctx.TLSCAPath != "" && !strings.Contains(cmdStr, "--tls-ca") {
		flags = append(flags, fmt.Sprintf("--tls-ca-path %q", ctx.TLSCAPath))
	}
	if ctx.TLSServerName != "" && !strings.Contains(cmdStr, "--tls-server-name") {
		flags = append(flags, fmt.Sprintf("--tls-server-name %q", ctx.TLSServerName))
	}
	if ctx.TLSSkipVerify && !strings.Contains(cmdStr, "--tls-disable-host-verification") {
		flags = append(flags, "--tls-disable-host-verification")
	}

	// Enable base TLS if any TLS option is set.
	// Check for "--tls " or "--tls" at end of string to avoid matching --tls-* flags.
	needsTLS := ctx.TLSCertPath != "" || ctx.TLSKeyPath != "" || ctx.TLSCAPath != "" ||
		ctx.TLSServerName != "" || ctx.TLSSkipVerify || ctx.APIKey != ""
	hasTLSFlag := strings.Contains(cmdStr, "--tls ") || strings.HasSuffix(cmdStr, "--tls")
	if needsTLS && !hasTLSFlag {
		flags = append(flags, "--tls")
	}
	if ctx.APIKey != "" && !strings.Contains(cmdStr, "--api-key") {
		flags = append(flags, fmt.Sprintf("--api-key %q", ctx.APIKey))
	}

	if len(flags) == 0 {
		return cmdStr
	}

	return cmdStr + " " + strings.Join(flags, " ")
}

// Run executes a command string via sh -c and returns combined output.
func Run(ctx context.Context, cmdStr string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunStreaming executes a command with line-by-line streaming via scanner on stdout pipe.
// Stderr is merged into stdout.
func RunStreaming(ctx context.Context, cmdStr string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Stderr = nil // will be merged via 2>&1 in the shell command or we merge pipes

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		onLine(scanner.Text())
	}

	return cmd.Wait()
}

// ParseWorkflowsOutput parses JSONL output from temporal workflow list --output json.
// Supports both temporal CLI v1 (flat keys) and v2 (nested execution/type objects).
func ParseWorkflowsOutput(output string) ([]temporal.Workflow, error) {
	// First, try parsing as a JSON array (some CLI versions output an array)
	output = strings.TrimSpace(output)
	if strings.HasPrefix(output, "[") {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(output), &arr); err == nil {
			var workflows []temporal.Workflow
			for _, raw := range arr {
				if wf := parseWorkflowJSON(raw); wf.ID != "" {
					workflows = append(workflows, wf)
				}
			}
			if len(workflows) > 0 {
				return workflows, nil
			}
		}
	}

	// Otherwise, parse as JSONL (one JSON object per line)
	var workflows []temporal.Workflow
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if wf := parseWorkflowJSON(raw); wf.ID != "" {
			workflows = append(workflows, wf)
		}
	}
	if len(workflows) == 0 {
		return nil, fmt.Errorf("no workflows found in output")
	}
	return workflows, nil
}

// parseWorkflowJSON extracts a Workflow from a raw JSON map.
// Handles multiple key formats from different temporal CLI versions.
func parseWorkflowJSON(raw map[string]interface{}) temporal.Workflow {
	return temporal.Workflow{
		ID:        jsonStr(raw, "workflowId", "execution.workflowId", "workflowExecutionInfo.execution.workflowId"),
		RunID:     jsonStr(raw, "runId", "execution.runId", "workflowExecutionInfo.execution.runId"),
		Type:      jsonStr(raw, "workflowType", "type.name", "workflowExecutionInfo.type.name"),
		Status:    normalizeStatus(jsonStr(raw, "status", "workflowExecutionInfo.status")),
		TaskQueue: jsonStr(raw, "taskQueue", "workflowExecutionInfo.taskQueue"),
	}
}

// ParseWorkflowOutput parses JSON output from temporal workflow describe --output json.
func ParseWorkflowOutput(output string) (workflowID, runID string, err error) {
	output = strings.TrimSpace(output)
	// Find the first { in output (skip any non-JSON preamble from CLI)
	if idx := strings.Index(output, "{"); idx > 0 {
		output = output[idx:]
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return "", "", fmt.Errorf("parsing workflow JSON: %w", err)
	}

	workflowID = jsonStr(raw, "workflowId", "execution.workflowId", "workflowExecutionInfo.execution.workflowId")
	runID = jsonStr(raw, "runId", "execution.runId", "workflowExecutionInfo.execution.runId")

	if workflowID == "" {
		return "", "", fmt.Errorf("no workflowId found in output")
	}
	return workflowID, runID, nil
}

// jsonStr extracts a string value from a map, trying multiple key paths.
func jsonStr(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		parts := strings.Split(key, ".")
		current := m
		for i, part := range parts {
			if i == len(parts)-1 {
				if v, ok := current[part]; ok {
					if s, ok := v.(string); ok {
						return s
					}
				}
			} else {
				if nested, ok := current[part].(map[string]interface{}); ok {
					current = nested
				} else {
					break
				}
			}
		}
	}
	return ""
}

// normalizeStatus converts Temporal CLI status format to display format.
func normalizeStatus(s string) string {
	s = strings.TrimPrefix(s, "WORKFLOW_EXECUTION_STATUS_")
	switch strings.ToUpper(s) {
	case "RUNNING":
		return "Running"
	case "COMPLETED":
		return "Completed"
	case "FAILED":
		return "Failed"
	case "CANCELED":
		return "Canceled"
	case "TERMINATED":
		return "Terminated"
	case "TIMED_OUT":
		return "TimedOut"
	default:
		if s != "" {
			// Title case the first letter
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		}
		return s
	}
}
