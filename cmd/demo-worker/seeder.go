//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"go.temporal.io/sdk/client"
)

const taskQueue = "demo-queue"

// Seeder creates synthetic test data with workflows in various states.
// Run with: go run seeder.go
//
// This will create workflows that are:
// - Running (long-running workflows)
// - Completed (fast workflows that finish)
// - Failed (workflows that hit guaranteed failures)
// - Canceled (workflows canceled via API)
// - Terminated (workflows terminated via API)
// - TimedOut (workflows with short timeouts)
// - ContinuedAsNew (workflows that continue-as-new)

func main() {
	rand.Seed(time.Now().UnixNano())

	address := os.Getenv("TEMPORAL_ADDRESS")
	if address == "" {
		address = "localhost:7233"
	}
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	c, err := client.Dial(client.Options{
		HostPort:  address,
		Namespace: namespace,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	fmt.Printf("Seeding Temporal at %s (namespace: %s)\n", address, namespace)
	fmt.Println("============================================")

	var started []workflowRef

	// 1. Start workflows that will COMPLETE successfully (~6)
	fmt.Println("\n[1/9] Starting workflows that will complete...")
	completingWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"UserOnboarding", map[string]interface{}{"userId": "user-001", "steps": []string{"welcome", "profile"}}},
		{"ReportGeneration", map[string]interface{}{"format": "pdf", "reportType": "monthly"}},
		{"DataImport", map[string]interface{}{"source": "csv", "file": "users.csv"}},
		{"NotificationBatch", map[string]interface{}{"type": "push", "message": "Hello!"}},
		{"MonitoringWorkflow", map[string]interface{}{"target": "api-service"}},
	}
	for _, wf := range completingWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s)\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// 2. Start Gantt demo workflow (many activities)
	fmt.Println("\n[2/9] Starting Gantt demo workflow (31 activities)...")
	ganttRef, err := startWorkflow(ctx, c, "GanttDemoWorkflow", map[string]interface{}{
		"description": "Complex workflow for Gantt chart visualization",
	})
	if err != nil {
		log.Printf("  ✗ Failed to start GanttDemoWorkflow: %v", err)
	} else {
		fmt.Printf("  ✓ Started GanttDemoWorkflow (%s) - 31 activities across 6 phases\n", ganttRef.id)
		started = append(started, ganttRef)
	}

	// 3. Start workflows that will FAIL (~4)
	fmt.Println("\n[3/9] Starting workflows that will fail...")
	failingWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"AlwaysFailingWorkflow", map[string]interface{}{"reason": "demo"}},
		{"AlwaysFailingWorkflow", map[string]interface{}{"reason": "test-error-display"}},
		{"AccountVerification", map[string]interface{}{"method": "phone", "userId": "fail-user"}}, // 25% fail rate
		{"DataValidation", map[string]interface{}{"strict": true}},                                // 30% fail rate
	}
	for _, wf := range failingWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - expected to fail\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// 4. Start LONG-RUNNING workflows (~3)
	fmt.Println("\n[4/9] Starting long-running workflows...")
	longRunningWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"ExtendedRunningWorkflow", map[string]interface{}{"durationMinutes": 10}},
		{"SlowProgressWorkflow", map[string]interface{}{"steps": 20}},
		{"LongRunningProcess", map[string]interface{}{"task": "background-job"}},
	}
	for _, wf := range longRunningWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - will run for a while\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// 5. Start workflows to CANCEL (~3)
	fmt.Println("\n[5/9] Starting workflows to cancel...")
	var toCancel []workflowRef
	cancelWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"ExtendedRunningWorkflow", map[string]interface{}{"durationMinutes": 30}},
		{"SlowProgressWorkflow", map[string]interface{}{"steps": 50}},
		{"ETLPipeline", map[string]interface{}{"sources": []string{"db1", "db2", "db3"}}},
	}
	for _, wf := range cancelWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - will cancel\n", wf.name, ref.id)
			toCancel = append(toCancel, ref)
			started = append(started, ref)
		}
	}

	// 6. Start workflows to TERMINATE (~3)
	fmt.Println("\n[6/9] Starting workflows to terminate...")
	var toTerminate []workflowRef
	terminateWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"ExtendedRunningWorkflow", map[string]interface{}{"durationMinutes": 60}},
		{"BatchProcessor", map[string]interface{}{"batchId": "batch-term", "itemCount": 10}},
		{"ProvisioningWorkflow", map[string]interface{}{"resourceType": "vm"}},
	}
	for _, wf := range terminateWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - will terminate\n", wf.name, ref.id)
			toTerminate = append(toTerminate, ref)
			started = append(started, ref)
		}
	}

	// 7. Start workflows that will TIMEOUT (~3)
	fmt.Println("\n[7/9] Starting workflows with short timeouts...")
	timeoutWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"TimeoutWorkflow", map[string]interface{}{"sleepSeconds": 120}},
		{"TimeoutWorkflow", map[string]interface{}{"sleepSeconds": 180}},
		{"TimeoutWorkflow", map[string]interface{}{"sleepSeconds": 60}},
	}
	for _, wf := range timeoutWorkflows {
		ref, err := startWorkflowWithTimeout(ctx, c, wf.name, wf.input, 5*time.Second)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - will timeout in 5s\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// 8. Start continue-as-new workflows (~2)
	fmt.Println("\n[8/9] Starting continue-as-new workflows...")
	canWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"ContinueAsNewWorkflow", map[string]interface{}{"iteration": 0, "maxIterations": 3}},
		{"ContinueAsNewWorkflow", map[string]interface{}{"iteration": 0, "maxIterations": 5}},
	}
	for _, wf := range canWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - will continue-as-new\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// 9. Start relationship demo workflows (for graph view visualization)
	fmt.Println("\n[9/9] Starting relationship demo workflows (for graph view)...")
	relationshipWorkflows := []struct {
		name  string
		input map[string]interface{}
	}{
		{"RelationshipDemoWorkflow", map[string]interface{}{"orderId": "order-graph-demo-1"}},
		{"RelationshipDemoWorkflow", map[string]interface{}{"orderId": "order-graph-demo-2"}},
	}
	for _, wf := range relationshipWorkflows {
		ref, err := startWorkflow(ctx, c, wf.name, wf.input)
		if err != nil {
			log.Printf("  ✗ Failed to start %s: %v", wf.name, err)
		} else {
			fmt.Printf("  ✓ Started %s (%s) - creates 9 child/grandchild workflows\n", wf.name, ref.id)
			started = append(started, ref)
		}
	}

	// Give workflows a moment to start
	fmt.Println("\nWaiting 3 seconds for workflows to start...")
	time.Sleep(3 * time.Second)

	// Cancel workflows
	fmt.Println("\nCanceling workflows...")
	for _, ref := range toCancel {
		err := c.CancelWorkflow(ctx, ref.id, ref.runID)
		if err != nil {
			log.Printf("  ✗ Failed to cancel %s: %v", ref.id, err)
		} else {
			fmt.Printf("  ✓ Canceled %s\n", ref.id)
		}
	}

	// Terminate workflows
	fmt.Println("\nTerminating workflows...")
	for _, ref := range toTerminate {
		err := c.TerminateWorkflow(ctx, ref.id, ref.runID, "Terminated by seeder for demo purposes")
		if err != nil {
			log.Printf("  ✗ Failed to terminate %s: %v", ref.id, err)
		} else {
			fmt.Printf("  ✓ Terminated %s\n", ref.id)
		}
	}

	// Summary
	fmt.Println("\n============================================")
	fmt.Printf("Seeding complete! Started %d workflows:\n", len(started))
	fmt.Println("  - ~5 will Complete")
	fmt.Println("  - 1 GanttDemo (31 activities for timeline view)")
	fmt.Println("  - ~4 will Fail")
	fmt.Println("  - ~3 are Running (long-running)")
	fmt.Println("  - ~3 Canceled")
	fmt.Println("  - ~3 Terminated")
	fmt.Println("  - ~3 will Timeout")
	fmt.Println("  - ~2 will ContinueAsNew")
	fmt.Println("  - 2 RelationshipDemo (9 children each for graph view)")
	fmt.Println("\nNote: Make sure the demo-worker is running to process these workflows!")
}

type workflowRef struct {
	id    string
	runID string
}

func startWorkflow(ctx context.Context, c client.Client, workflowType string, input map[string]interface{}) (workflowRef, error) {
	workflowID := fmt.Sprintf("seed-%s-%d", workflowType, rand.Intn(100000))

	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	run, err := c.ExecuteWorkflow(ctx, opts, workflowType, input)
	if err != nil {
		return workflowRef{}, err
	}

	return workflowRef{id: run.GetID(), runID: run.GetRunID()}, nil
}

func startWorkflowWithTimeout(ctx context.Context, c client.Client, workflowType string, input map[string]interface{}, timeout time.Duration) (workflowRef, error) {
	workflowID := fmt.Sprintf("seed-%s-%d", workflowType, rand.Intn(100000))

	opts := client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: timeout,
	}

	run, err := c.ExecuteWorkflow(ctx, opts, workflowType, input)
	if err != nil {
		return workflowRef{}, err
	}

	return workflowRef{id: run.GetID(), runID: run.GetRunID()}, nil
}

