package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	taskQueue = "demo-queue"
)

func main() {
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

	w := worker.New(c, taskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(OrderWorkflow)
	w.RegisterWorkflow(UserRegistration)
	w.RegisterWorkflow(UserOnboarding)
	w.RegisterWorkflow(AccountVerification)
	w.RegisterWorkflow(PaymentProcess)
	w.RegisterWorkflow(RefundProcess)
	w.RegisterWorkflow(SubscriptionBilling)
	w.RegisterWorkflow(DataImport)
	w.RegisterWorkflow(DataExport)
	w.RegisterWorkflow(ReportGeneration)
	w.RegisterWorkflow(ETLPipeline)
	w.RegisterWorkflow(DataValidation)
	w.RegisterWorkflow(EmailCampaign)
	w.RegisterWorkflow(NotificationBatch)
	w.RegisterWorkflow(SMSNotification)
	w.RegisterWorkflow(LongRunningProcess)
	w.RegisterWorkflow(MonitoringWorkflow)
	w.RegisterWorkflow(BatchProcessor)
	w.RegisterWorkflow(OrderSaga)
	w.RegisterWorkflow(ProvisioningWorkflow)

	// Demo workflows - guaranteed failures and long-running
	w.RegisterWorkflow(AlwaysFailingWorkflow)
	w.RegisterWorkflow(ExtendedRunningWorkflow)
	w.RegisterWorkflow(SlowProgressWorkflow)
	w.RegisterWorkflow(TimeoutWorkflow)
	w.RegisterWorkflow(ContinueAsNewWorkflow)
	w.RegisterWorkflow(GanttDemoWorkflow)

	// Relationship demo workflows - for graph view visualization
	w.RegisterWorkflow(RelationshipDemoWorkflow)
	w.RegisterWorkflow(OrderProcessingChild)
	w.RegisterWorkflow(FulfillmentChild)
	w.RegisterWorkflow(WarehouseGrandchild)
	w.RegisterWorkflow(ShippingGrandchild)
	w.RegisterWorkflow(PaymentProcessingChild)
	w.RegisterWorkflow(FraudCheckGrandchild)
	w.RegisterWorkflow(ChargeGrandchild)
	w.RegisterWorkflow(NotificationFanout)
	w.RegisterWorkflow(EmailNotificationChild)
	w.RegisterWorkflow(SMSNotificationChild)
	w.RegisterWorkflow(PushNotificationChild)

	// Child workflows
	w.RegisterWorkflow(PaymentChildWorkflow)
	w.RegisterWorkflow(ShippingChildWorkflow)
	w.RegisterWorkflow(InventoryChildWorkflow)
	w.RegisterWorkflow(NotificationChildWorkflow)
	w.RegisterWorkflow(BatchItemWorkflow)

	// Register activities
	w.RegisterActivity(ValidateOrder)
	w.RegisterActivity(ProcessPayment)
	w.RegisterActivity(ReserveInventory)
	w.RegisterActivity(ShipOrder)
	w.RegisterActivity(SendEmail)
	w.RegisterActivity(SendSMS)
	w.RegisterActivity(SendPushNotification)
	w.RegisterActivity(ValidateUser)
	w.RegisterActivity(CreateAccount)
	w.RegisterActivity(SendWelcomeEmail)
	w.RegisterActivity(SetupProfile)
	w.RegisterActivity(RunTutorial)
	w.RegisterActivity(VerifyEmail)
	w.RegisterActivity(VerifyPhone)
	w.RegisterActivity(ChargeCard)
	w.RegisterActivity(ProcessACH)
	w.RegisterActivity(IssueRefund)
	w.RegisterActivity(FetchData)
	w.RegisterActivity(TransformData)
	w.RegisterActivity(LoadData)
	w.RegisterActivity(ValidateData)
	w.RegisterActivity(GenerateReport)
	w.RegisterActivity(ProvisionResource)
	w.RegisterActivity(ConfigureResource)
	w.RegisterActivity(HealthCheck)
	w.RegisterActivity(ProcessBatchItem)
	w.RegisterActivity(LongRunningTask)

	// Demo activities - guaranteed failures
	w.RegisterActivity(AlwaysFailingActivity)
	w.RegisterActivity(SlowActivity)
	w.RegisterActivity(GanttActivity)

	fmt.Printf("Starting demo worker on task queue: %s\n", taskQueue)
	fmt.Printf("Address: %s, Namespace: %s\n", address, namespace)

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down worker...")
		w.Stop()
	}()

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

// ============================================================================
// WORKFLOW DEFINITIONS
// ============================================================================

// OrderWorkflow - multi-step order processing with child workflows
func OrderWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OrderWorkflow started", "input", input)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Validate order
	var validationResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ValidateOrder, input).Get(ctx, &validationResult); err != nil {
		return nil, err
	}

	// Step 2: Reserve inventory (child workflow)
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID + "-inventory",
	})
	var inventoryResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(childCtx, InventoryChildWorkflow, input).Get(ctx, &inventoryResult); err != nil {
		return nil, err
	}

	// Step 3: Process payment (child workflow)
	childCtx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID + "-payment",
	})
	var paymentResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(childCtx, PaymentChildWorkflow, input).Get(ctx, &paymentResult); err != nil {
		return nil, err
	}

	// Step 4: Ship order (child workflow)
	childCtx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID + "-shipping",
	})
	var shippingResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(childCtx, ShippingChildWorkflow, input).Get(ctx, &shippingResult); err != nil {
		return nil, err
	}

	// Step 5: Send confirmation
	if err := workflow.ExecuteActivity(ctx, SendEmail, map[string]interface{}{
		"type":    "order_confirmation",
		"orderId": input["orderId"],
	}).Get(ctx, nil); err != nil {
		logger.Warn("Failed to send confirmation email", "error", err)
	}

	return map[string]interface{}{
		"status":    "completed",
		"orderId":   input["orderId"],
		"inventory": inventoryResult,
		"payment":   paymentResult,
		"shipping":  shippingResult,
	}, nil
}

// InventoryChildWorkflow - reserves inventory
func InventoryChildWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ReserveInventory, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// PaymentChildWorkflow - processes payment
func PaymentChildWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ProcessPayment, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ShippingChildWorkflow - handles shipping
func ShippingChildWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ShipOrder, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// NotificationChildWorkflow - sends notifications
func NotificationChildWorkflow(ctx workflow.Context, input map[string]interface{}) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	notifType, _ := input["type"].(string)
	switch notifType {
	case "email":
		return workflow.ExecuteActivity(ctx, SendEmail, input).Get(ctx, nil)
	case "sms":
		return workflow.ExecuteActivity(ctx, SendSMS, input).Get(ctx, nil)
	case "push":
		return workflow.ExecuteActivity(ctx, SendPushNotification, input).Get(ctx, nil)
	default:
		return workflow.ExecuteActivity(ctx, SendEmail, input).Get(ctx, nil)
	}
}

// UserRegistration - registration with possible failure
func UserRegistration(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Validate user
	var validResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ValidateUser, input).Get(ctx, &validResult); err != nil {
		return nil, err
	}

	// Create account - may fail for demo purposes
	var accountResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, CreateAccount, input).Get(ctx, &accountResult); err != nil {
		return nil, fmt.Errorf("account creation failed: %w", err)
	}

	// Send welcome email
	if err := workflow.ExecuteActivity(ctx, SendWelcomeEmail, input).Get(ctx, nil); err != nil {
		// Non-fatal, log and continue
		workflow.GetLogger(ctx).Warn("Failed to send welcome email", "error", err)
	}

	return map[string]interface{}{
		"status": "registered",
		"userId": accountResult["userId"],
	}, nil
}

// UserOnboarding - multi-step onboarding
func UserOnboarding(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	steps := []string{"welcome", "profile", "preferences", "tutorial"}
	if s, ok := input["steps"].([]interface{}); ok {
		steps = make([]string, len(s))
		for i, v := range s {
			steps[i], _ = v.(string)
		}
	}

	completedSteps := []string{}
	for _, step := range steps {
		switch step {
		case "welcome":
			if err := workflow.ExecuteActivity(ctx, SendWelcomeEmail, input).Get(ctx, nil); err != nil {
				return nil, err
			}
		case "profile":
			if err := workflow.ExecuteActivity(ctx, SetupProfile, input).Get(ctx, nil); err != nil {
				return nil, err
			}
		case "preferences":
			workflow.Sleep(ctx, 500*time.Millisecond)
		case "tutorial":
			if err := workflow.ExecuteActivity(ctx, RunTutorial, input).Get(ctx, nil); err != nil {
				return nil, err
			}
		}
		completedSteps = append(completedSteps, step)
	}

	return map[string]interface{}{
		"status":         "onboarded",
		"completedSteps": completedSteps,
	}, nil
}

// AccountVerification - verification that may fail
func AccountVerification(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	method, _ := input["method"].(string)
	if method == "" {
		method = "email"
	}

	var err error
	switch method {
	case "email":
		err = workflow.ExecuteActivity(ctx, VerifyEmail, input).Get(ctx, nil)
	case "phone":
		err = workflow.ExecuteActivity(ctx, VerifyPhone, input).Get(ctx, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	return map[string]interface{}{
		"status": "verified",
		"method": method,
	}, nil
}

// PaymentProcess - payment with retries
func PaymentProcess(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	method, _ := input["method"].(string)

	var result map[string]interface{}
	var err error

	switch method {
	case "card":
		err = workflow.ExecuteActivity(ctx, ChargeCard, input).Get(ctx, &result)
	case "ach":
		err = workflow.ExecuteActivity(ctx, ProcessACH, input).Get(ctx, &result)
	default:
		err = workflow.ExecuteActivity(ctx, ChargeCard, input).Get(ctx, &result)
	}

	if err != nil {
		return nil, err
	}

	// Send receipt
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID + "-receipt",
	})
	workflow.ExecuteChildWorkflow(childCtx, NotificationChildWorkflow, map[string]interface{}{
		"type":      "email",
		"template":  "receipt",
		"paymentId": input["paymentId"],
	})

	return result, nil
}

// RefundProcess - refund that sometimes fails
func RefundProcess(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, IssueRefund, input).Get(ctx, &result); err != nil {
		return nil, fmt.Errorf("refund failed: %w", err)
	}

	return result, nil
}

// SubscriptionBilling - recurring billing
func SubscriptionBilling(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ChargeCard, input).Get(ctx, &result); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":   "billed",
		"chargeId": result["chargeId"],
	}, nil
}

// DataImport - ETL import with steps
func DataImport(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		HeartbeatTimeout:    10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch
	var fetchResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, FetchData, input).Get(ctx, &fetchResult); err != nil {
		return nil, err
	}

	// Transform
	var transformResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, TransformData, fetchResult).Get(ctx, &transformResult); err != nil {
		return nil, err
	}

	// Load
	var loadResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, LoadData, transformResult).Get(ctx, &loadResult); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":      "imported",
		"rowsImported": loadResult["rowCount"],
	}, nil
}

// DataExport - export with multiple steps
func DataExport(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	tables, _ := input["tables"].([]interface{})
	exportedTables := []string{}

	for _, t := range tables {
		tableName, _ := t.(string)
		if err := workflow.ExecuteActivity(ctx, FetchData, map[string]interface{}{
			"table": tableName,
		}).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("export failed for table %s: %w", tableName, err)
		}
		exportedTables = append(exportedTables, tableName)
	}

	return map[string]interface{}{
		"status": "exported",
		"tables": exportedTables,
	}, nil
}

// ReportGeneration - report with generation step
func ReportGeneration(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, GenerateReport, input).Get(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ETLPipeline - complex ETL with multiple sources
func ETLPipeline(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	sources, _ := input["sources"].([]interface{})
	results := map[string]interface{}{}

	for _, s := range sources {
		source, _ := s.(string)
		var fetchResult map[string]interface{}
		if err := workflow.ExecuteActivity(ctx, FetchData, map[string]interface{}{
			"source": source,
		}).Get(ctx, &fetchResult); err != nil {
			return nil, err
		}
		results[source] = fetchResult
	}

	// Transform all
	var transformResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, TransformData, results).Get(ctx, &transformResult); err != nil {
		return nil, err
	}

	// Load
	if err := workflow.ExecuteActivity(ctx, LoadData, transformResult).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":  "completed",
		"sources": sources,
	}, nil
}

// DataValidation - validation that may find issues
func DataValidation(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ValidateData, input).Get(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// EmailCampaign - batch email campaign
func EmailCampaign(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Simulate sending to multiple recipients
	for i := 0; i < 5; i++ {
		if err := workflow.ExecuteActivity(ctx, SendEmail, map[string]interface{}{
			"batch":    i,
			"campaign": input["campaignId"],
		}).Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("Batch failed", "batch", i, "error", err)
		}
	}

	return map[string]interface{}{
		"status":    "sent",
		"batchesSent": 5,
	}, nil
}

// NotificationBatch - batch notifications
func NotificationBatch(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	notifType, _ := input["type"].(string)

	switch notifType {
	case "push":
		if err := workflow.ExecuteActivity(ctx, SendPushNotification, input).Get(ctx, nil); err != nil {
			return nil, err
		}
	case "email":
		if err := workflow.ExecuteActivity(ctx, SendEmail, input).Get(ctx, nil); err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"status": "sent",
		"type":   notifType,
	}, nil
}

// SMSNotification - SMS sending
func SMSNotification(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, SendSMS, input).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status": "sent",
	}, nil
}

// LongRunningProcess - simulates a long-running workflow
func LongRunningProcess(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Run a long task with heartbeating
	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, LongRunningTask, input).Get(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// MonitoringWorkflow - continuous monitoring (completes after a few checks)
func MonitoringWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	checksPerformed := 0
	for i := 0; i < 3; i++ {
		if err := workflow.ExecuteActivity(ctx, HealthCheck, input).Get(ctx, nil); err != nil {
			return nil, fmt.Errorf("health check failed: %w", err)
		}
		checksPerformed++
		workflow.Sleep(ctx, 1*time.Second)
	}

	return map[string]interface{}{
		"status": "healthy",
		"checks": checksPerformed,
	}, nil
}

// BatchProcessor - processes items in parallel using child workflows
func BatchProcessor(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	itemCount := 5
	if ic, ok := input["itemCount"].(float64); ok && ic > 0 && ic <= 10 {
		itemCount = int(ic)
	}

	// Launch child workflows for each item
	var futures []workflow.ChildWorkflowFuture
	for i := 0; i < itemCount; i++ {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("%s-item-%d", workflow.GetInfo(ctx).WorkflowExecution.ID, i),
		})
		future := workflow.ExecuteChildWorkflow(childCtx, BatchItemWorkflow, map[string]interface{}{
			"itemIndex": i,
			"batchId":   input["batchId"],
		})
		futures = append(futures, future)
	}

	// Wait for all
	successCount := 0
	failCount := 0
	for _, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	return map[string]interface{}{
		"status":    "completed",
		"success":   successCount,
		"failed":    failCount,
		"totalItems": itemCount,
	}, nil
}

// BatchItemWorkflow - processes a single batch item
func BatchItemWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ProcessBatchItem, input).Get(ctx, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// OrderSaga - saga pattern with compensation
func OrderSaga(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Reserve inventory
	if err := workflow.ExecuteActivity(ctx, ReserveInventory, input).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Charge payment (may fail)
	if err := workflow.ExecuteActivity(ctx, ProcessPayment, input).Get(ctx, nil); err != nil {
		// Compensate - release inventory
		workflow.ExecuteActivity(ctx, ReserveInventory, map[string]interface{}{
			"action": "release",
		})
		return nil, fmt.Errorf("payment failed, inventory released: %w", err)
	}

	// Ship order
	if err := workflow.ExecuteActivity(ctx, ShipOrder, input).Get(ctx, nil); err != nil {
		// Compensate - refund payment
		workflow.ExecuteActivity(ctx, IssueRefund, input)
		return nil, fmt.Errorf("shipping failed, payment refunded: %w", err)
	}

	// Send confirmation
	workflow.ExecuteActivity(ctx, SendEmail, map[string]interface{}{
		"type":    "confirmation",
		"orderId": input["orderId"],
	})

	return map[string]interface{}{
		"status":  "completed",
		"orderId": input["orderId"],
	}, nil
}

// ProvisioningWorkflow - resource provisioning with steps
func ProvisioningWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Provision
	var provisionResult map[string]interface{}
	if err := workflow.ExecuteActivity(ctx, ProvisionResource, input).Get(ctx, &provisionResult); err != nil {
		return nil, err
	}

	// Configure
	if err := workflow.ExecuteActivity(ctx, ConfigureResource, provisionResult).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Health check
	if err := workflow.ExecuteActivity(ctx, HealthCheck, provisionResult).Get(ctx, nil); err != nil {
		return nil, fmt.Errorf("resource unhealthy after provisioning: %w", err)
	}

	return map[string]interface{}{
		"status":     "provisioned",
		"resourceId": provisionResult["resourceId"],
	}, nil
}

// ============================================================================
// ACTIVITY DEFINITIONS
// ============================================================================

func ValidateOrder(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 500)
	return map[string]interface{}{"valid": true}, nil
}

func ProcessPayment(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(300, 800)
	// 10% chance of failure for demo
	if shouldFail(10) {
		return nil, temporal.NewApplicationError("payment declined", "PAYMENT_DECLINED")
	}
	return map[string]interface{}{
		"chargeId": fmt.Sprintf("ch_%d", rand.Intn(100000)),
		"status":   "charged",
	}, nil
}

func ReserveInventory(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(100, 300)
	return map[string]interface{}{
		"reserved": true,
		"warehouse": "WH-01",
	}, nil
}

func ShipOrder(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 600)
	return map[string]interface{}{
		"trackingNumber": fmt.Sprintf("TRK%d", rand.Intn(1000000)),
		"carrier":        "FastShip",
	}, nil
}

func SendEmail(ctx context.Context, input map[string]interface{}) error {
	simulateWork(50, 150)
	return nil
}

func SendSMS(ctx context.Context, input map[string]interface{}) error {
	simulateWork(50, 100)
	// 5% failure rate
	if shouldFail(5) {
		return errors.New("SMS gateway timeout")
	}
	return nil
}

func SendPushNotification(ctx context.Context, input map[string]interface{}) error {
	simulateWork(30, 80)
	return nil
}

func ValidateUser(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(100, 200)
	return map[string]interface{}{"valid": true}, nil
}

func CreateAccount(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 400)
	// 15% failure for demo variety
	if shouldFail(15) {
		return nil, temporal.NewApplicationError("email already exists", "DUPLICATE_EMAIL")
	}
	return map[string]interface{}{
		"userId": fmt.Sprintf("usr_%d", rand.Intn(100000)),
	}, nil
}

func SendWelcomeEmail(ctx context.Context, input map[string]interface{}) error {
	simulateWork(50, 100)
	return nil
}

func SetupProfile(ctx context.Context, input map[string]interface{}) error {
	simulateWork(100, 200)
	return nil
}

func RunTutorial(ctx context.Context, input map[string]interface{}) error {
	simulateWork(200, 400)
	return nil
}

func VerifyEmail(ctx context.Context, input map[string]interface{}) error {
	simulateWork(100, 300)
	// 20% failure rate for verification
	if shouldFail(20) {
		return temporal.NewApplicationError("verification code expired", "CODE_EXPIRED")
	}
	return nil
}

func VerifyPhone(ctx context.Context, input map[string]interface{}) error {
	simulateWork(100, 300)
	if shouldFail(25) {
		return temporal.NewApplicationError("invalid verification code", "INVALID_CODE")
	}
	return nil
}

func ChargeCard(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(300, 600)
	if shouldFail(8) {
		return nil, temporal.NewApplicationError("card declined", "CARD_DECLINED")
	}
	return map[string]interface{}{
		"chargeId": fmt.Sprintf("ch_%d", rand.Intn(100000)),
		"status":   "succeeded",
	}, nil
}

func ProcessACH(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(500, 1000)
	return map[string]interface{}{
		"transferId": fmt.Sprintf("ach_%d", rand.Intn(100000)),
		"status":     "pending",
	}, nil
}

func IssueRefund(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 400)
	if shouldFail(10) {
		return nil, errors.New("refund limit exceeded")
	}
	return map[string]interface{}{
		"refundId": fmt.Sprintf("ref_%d", rand.Intn(100000)),
		"status":   "refunded",
	}, nil
}

func FetchData(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(500, 1500)
	return map[string]interface{}{
		"rowCount": rand.Intn(10000) + 1000,
		"source":   input["source"],
	}, nil
}

func TransformData(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(300, 800)
	return map[string]interface{}{
		"transformed": true,
		"rowCount":    input["rowCount"],
	}, nil
}

func LoadData(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(400, 1000)
	return map[string]interface{}{
		"loaded":   true,
		"rowCount": input["rowCount"],
	}, nil
}

func ValidateData(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 500)
	// Sometimes find validation issues
	if shouldFail(30) {
		return map[string]interface{}{
			"valid":  false,
			"errors": []string{"Missing required field", "Invalid date format"},
		}, nil
	}
	return map[string]interface{}{
		"valid":  true,
		"errors": []string{},
	}, nil
}

func GenerateReport(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(1000, 3000)
	return map[string]interface{}{
		"reportId": fmt.Sprintf("rpt_%d", rand.Intn(10000)),
		"pages":    rand.Intn(50) + 5,
		"format":   input["format"],
	}, nil
}

func ProvisionResource(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(1000, 2000)
	return map[string]interface{}{
		"resourceId":   fmt.Sprintf("res_%d", rand.Intn(10000)),
		"resourceType": input["resourceType"],
	}, nil
}

func ConfigureResource(ctx context.Context, input map[string]interface{}) error {
	simulateWork(500, 1000)
	return nil
}

func HealthCheck(ctx context.Context, input map[string]interface{}) error {
	simulateWork(100, 300)
	// 5% health check failures
	if shouldFail(5) {
		return errors.New("health check failed: connection timeout")
	}
	return nil
}

func ProcessBatchItem(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	simulateWork(200, 800)
	// 10% failure rate for batch items
	if shouldFail(10) {
		return nil, fmt.Errorf("failed to process item %v", input["itemIndex"])
	}
	return map[string]interface{}{
		"processed": true,
		"itemIndex": input["itemIndex"],
	}, nil
}

func LongRunningTask(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Simulate a longer running task with heartbeats
	for i := 0; i < 5; i++ {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("step %d/5", i+1))
		simulateWork(500, 1000)
	}
	return map[string]interface{}{
		"status":   "completed",
		"duration": "long",
	}, nil
}

// ============================================================================
// DEMO WORKFLOWS - GUARANTEED FAILURES AND LONG-RUNNING
// ============================================================================

// AlwaysFailingWorkflow - always fails after some work (for testing error display)
func AlwaysFailingWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("AlwaysFailingWorkflow started - this will fail", "input", input)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1, // Don't retry - fail immediately
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Do some work first so it's not instant
	workflow.Sleep(ctx, 2*time.Second)

	// Execute an activity that always fails
	if err := workflow.ExecuteActivity(ctx, AlwaysFailingActivity, input).Get(ctx, nil); err != nil {
		return nil, err
	}

	return nil, nil // Never reached
}

// ExtendedRunningWorkflow - runs for several minutes with periodic activity
func ExtendedRunningWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ExtendedRunningWorkflow started - will run for several minutes", "input", input)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Default to 5 minutes, configurable via input
	durationMinutes := 5
	if d, ok := input["durationMinutes"].(float64); ok && d > 0 {
		durationMinutes = int(d)
	}

	iterations := durationMinutes * 2 // Check every 30 seconds
	for i := 0; i < iterations; i++ {
		logger.Info("ExtendedRunningWorkflow progress", "iteration", i+1, "total", iterations)

		// Sleep for 30 seconds
		workflow.Sleep(ctx, 30*time.Second)

		// Periodic health check activity
		if err := workflow.ExecuteActivity(ctx, HealthCheck, map[string]interface{}{
			"iteration": i,
		}).Get(ctx, nil); err != nil {
			logger.Warn("Health check failed during extended run", "error", err)
		}
	}

	return map[string]interface{}{
		"status":          "completed",
		"durationMinutes": durationMinutes,
		"iterations":      iterations,
	}, nil
}

// SlowProgressWorkflow - runs for configurable duration with slow activities
func SlowProgressWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SlowProgressWorkflow started", "input", input)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Second,
		HeartbeatTimeout:    30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Number of slow steps, default 10
	steps := 10
	if s, ok := input["steps"].(float64); ok && s > 0 {
		steps = int(s)
	}

	completedSteps := 0
	for i := 0; i < steps; i++ {
		logger.Info("SlowProgressWorkflow executing step", "step", i+1, "total", steps)

		var result map[string]interface{}
		if err := workflow.ExecuteActivity(ctx, SlowActivity, map[string]interface{}{
			"step":       i,
			"totalSteps": steps,
		}).Get(ctx, &result); err != nil {
			return nil, fmt.Errorf("step %d failed: %w", i+1, err)
		}
		completedSteps++
	}

	return map[string]interface{}{
		"status":         "completed",
		"completedSteps": completedSteps,
	}, nil
}

// AlwaysFailingActivity - always returns an error
func AlwaysFailingActivity(ctx context.Context, input map[string]interface{}) error {
	simulateWork(500, 1000) // Do some work first
	return temporal.NewApplicationError(
		"This activity is designed to always fail for demo purposes",
		"DEMO_FAILURE",
		map[string]interface{}{
			"reason":    "intentional_failure",
			"timestamp": time.Now().Unix(),
		},
	)
}

// SlowActivity - takes 15-30 seconds to complete
func SlowActivity(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	step, _ := input["step"].(float64)
	totalSteps, _ := input["totalSteps"].(float64)

	// 15-30 seconds per step
	duration := time.Duration(15+rand.Intn(15)) * time.Second

	// Heartbeat periodically during slow work
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done := time.After(duration)
	elapsed := 0
	for {
		select {
		case <-ticker.C:
			elapsed += 5
			activity.RecordHeartbeat(ctx, fmt.Sprintf("step %d/%d - %ds elapsed", int(step)+1, int(totalSteps), elapsed))
		case <-done:
			return map[string]interface{}{
				"step":      step,
				"completed": true,
				"duration":  duration.Seconds(),
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ============================================================================
// HELPERS
// ============================================================================

func simulateWork(minMs, maxMs int) {
	duration := time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
	time.Sleep(duration)
}

func shouldFail(percentChance int) bool {
	return rand.Intn(100) < percentChance
}

// ============================================================================
// TIMEOUT AND CONTINUE-AS-NEW WORKFLOWS
// ============================================================================

// TimeoutWorkflow - designed to be started with a short WorkflowExecutionTimeout
// so it times out. The workflow itself sleeps longer than the timeout.
func TimeoutWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("TimeoutWorkflow started - will sleep until timeout", "input", input)

	// Default to 120 seconds sleep - caller sets a shorter WorkflowExecutionTimeout
	sleepSeconds := 120
	if s, ok := input["sleepSeconds"].(float64); ok && s > 0 {
		sleepSeconds = int(s)
	}

	// Sleep for longer than the workflow timeout
	workflow.Sleep(ctx, time.Duration(sleepSeconds)*time.Second)

	// This should never be reached if timeout is set correctly
	return map[string]interface{}{
		"status":       "completed",
		"sleepSeconds": sleepSeconds,
	}, nil
}

// ContinueAsNewWorkflow - demonstrates continue-as-new functionality
// Runs for a few iterations, then continues as new with updated state
func ContinueAsNewWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)

	// Get current iteration
	iteration := 0
	if i, ok := input["iteration"].(float64); ok {
		iteration = int(i)
	}

	maxIterations := 3
	if m, ok := input["maxIterations"].(float64); ok && m > 0 {
		maxIterations = int(m)
	}

	logger.Info("ContinueAsNewWorkflow iteration", "iteration", iteration, "max", maxIterations)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Do some work in this iteration
	if err := workflow.ExecuteActivity(ctx, HealthCheck, map[string]interface{}{
		"iteration": iteration,
	}).Get(ctx, nil); err != nil {
		logger.Warn("Health check failed", "iteration", iteration, "error", err)
	}

	// Sleep a bit between iterations
	workflow.Sleep(ctx, 2*time.Second)

	// Check if we should continue as new
	if iteration < maxIterations-1 {
		// Continue as new with incremented iteration
		return nil, workflow.NewContinueAsNewError(ctx, ContinueAsNewWorkflow, map[string]interface{}{
			"iteration":     iteration + 1,
			"maxIterations": maxIterations,
			"previousRunID": workflow.GetInfo(ctx).WorkflowExecution.RunID,
		})
	}

	// Final iteration - complete the workflow
	return map[string]interface{}{
		"status":          "completed",
		"totalIterations": maxIterations,
		"finalIteration":  iteration,
	}, nil
}

// GanttDemoWorkflow - demonstrates many activities for Gantt chart visualization
// Creates a complex activity pattern with parallel and sequential execution
func GanttDemoWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("GanttDemoWorkflow started - many activities for Gantt visualization")

	// Phase 1: Initialization (sequential)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// --- PHASE 1: Sequential initialization activities ---
	logger.Info("Phase 1: Initialization")

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "Initialize", "duration": 500, "phase": 1,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "LoadConfig", "duration": 300, "phase": 1,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "ValidateInput", "duration": 400, "phase": 1,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// --- PHASE 2: Parallel data fetching ---
	logger.Info("Phase 2: Parallel data fetching")

	var fetchFutures []workflow.Future
	fetchSources := []string{"DatabaseA", "DatabaseB", "CacheLayer", "ExternalAPI", "FileSystem"}
	for _, source := range fetchSources {
		future := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
			"name": "Fetch" + source, "duration": 800 + rand.Intn(1200), "phase": 2,
		})
		fetchFutures = append(fetchFutures, future)
	}

	// Wait for all fetches
	for i, f := range fetchFutures {
		if err := f.Get(ctx, nil); err != nil {
			logger.Warn("Fetch failed", "source", fetchSources[i], "error", err)
		}
	}

	// --- PHASE 3: Sequential processing ---
	logger.Info("Phase 3: Sequential processing")

	processingSteps := []struct {
		name     string
		duration int
	}{
		{"MergeData", 600},
		{"DeduplicateRecords", 400},
		{"ApplyTransforms", 700},
		{"ValidateSchema", 300},
		{"EnrichMetadata", 500},
	}

	for _, step := range processingSteps {
		if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
			"name": step.name, "duration": step.duration, "phase": 3,
		}).Get(ctx, nil); err != nil {
			return nil, err
		}
	}

	// --- PHASE 4: Parallel output generation ---
	logger.Info("Phase 4: Parallel output generation")

	var outputFutures []workflow.Future
	outputs := []struct {
		name     string
		duration int
	}{
		{"GenerateJSON", 400},
		{"GenerateCSV", 600},
		{"GenerateXML", 500},
		{"GeneratePDF", 1500},
		{"GenerateHTML", 700},
		{"UpdateSearchIndex", 900},
	}

	for _, out := range outputs {
		future := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
			"name": out.name, "duration": out.duration, "phase": 4,
		})
		outputFutures = append(outputFutures, future)
	}

	// Wait for all outputs
	for _, f := range outputFutures {
		f.Get(ctx, nil) // Ignore errors for demo
	}

	// --- PHASE 5: Notification fan-out (parallel) ---
	logger.Info("Phase 5: Notification fan-out")

	var notifyFutures []workflow.Future
	channels := []string{"Email", "Slack", "Webhook", "SMS", "PushNotification", "AuditLog", "MetricsCollector", "CacheInvalidation"}

	for _, channel := range channels {
		future := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
			"name": "Notify" + channel, "duration": 200 + rand.Intn(400), "phase": 5,
		})
		notifyFutures = append(notifyFutures, future)
	}

	// Wait for all notifications
	for _, f := range notifyFutures {
		f.Get(ctx, nil)
	}

	// --- PHASE 6: Cleanup (sequential) ---
	logger.Info("Phase 6: Cleanup")

	cleanupSteps := []string{"ReleaseLocks", "CleanupTempFiles", "UpdateStatus", "FinalizeAudit"}
	for _, step := range cleanupSteps {
		if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
			"name": step, "duration": 250, "phase": 6,
		}).Get(ctx, nil); err != nil {
			logger.Warn("Cleanup step failed", "step", step, "error", err)
		}
	}

	return map[string]interface{}{
		"status":           "completed",
		"totalActivities":  len(fetchSources) + len(processingSteps) + len(outputs) + len(channels) + len(cleanupSteps) + 3,
		"phases":           6,
	}, nil
}

// GanttActivity - generic activity for Gantt demo with configurable duration
func GanttActivity(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	name, _ := input["name"].(string)
	duration := 500
	if d, ok := input["duration"].(float64); ok {
		duration = int(d)
	}
	phase, _ := input["phase"].(float64)

	// Add some randomness to duration (±20%)
	variance := duration / 5
	actualDuration := duration - variance + rand.Intn(variance*2)

	// Simulate work with heartbeats for longer activities
	if actualDuration > 1000 {
		iterations := actualDuration / 500
		for i := 0; i < iterations; i++ {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("%s: %d%%", name, (i+1)*100/iterations))
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		time.Sleep(time.Duration(actualDuration) * time.Millisecond)
	}

	return map[string]interface{}{
		"name":     name,
		"phase":    phase,
		"duration": actualDuration,
	}, nil
}

// ============================================================================
// RELATIONSHIP DEMO WORKFLOWS - Complex hierarchy for graph view visualization
// ============================================================================

// RelationshipDemoWorkflow creates a complex workflow tree to demonstrate the
// relationship graph view. It spawns multiple children which spawn grandchildren,
// and sends signals between workflows.
//
// Structure:
//
//	RelationshipDemoWorkflow (root)
//	├── OrderProcessingChild
//	│   ├── FulfillmentChild
//	│   │   ├── WarehouseGrandchild
//	│   │   └── ShippingGrandchild
//	│   └── PaymentProcessingChild
//	│       ├── FraudCheckGrandchild
//	│       └── ChargeGrandchild
//	└── NotificationFanout
//	    ├── EmailNotificationChild
//	    ├── SMSNotificationChild
//	    └── PushNotificationChild
func RelationshipDemoWorkflow(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("RelationshipDemoWorkflow started - creating complex hierarchy for graph demo")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Initial activity
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "InitializeOrder", "duration": 500, "phase": 1,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Start OrderProcessingChild - handles fulfillment and payment
	orderChildCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-order-processing",
	})
	orderFuture := workflow.ExecuteChildWorkflow(orderChildCtx, OrderProcessingChild, map[string]interface{}{
		"orderId":  input["orderId"],
		"parentId": workflowID,
	})

	// Start NotificationFanout - handles all notifications in parallel
	notifyChildCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-notifications",
	})
	notifyFuture := workflow.ExecuteChildWorkflow(notifyChildCtx, NotificationFanout, map[string]interface{}{
		"orderId":  input["orderId"],
		"parentId": workflowID,
		"channels": []string{"email", "sms", "push"},
	})

	// Wait for order processing
	var orderResult map[string]interface{}
	if err := orderFuture.Get(ctx, &orderResult); err != nil {
		return nil, fmt.Errorf("order processing failed: %w", err)
	}

	// Signal the notification workflow that order is complete
	if err := workflow.SignalExternalWorkflow(ctx, workflowID+"-notifications", "", "order-complete", map[string]interface{}{
		"status":    "completed",
		"orderData": orderResult,
	}).Get(ctx, nil); err != nil {
		logger.Warn("Failed to signal notification workflow", "error", err)
	}

	// Wait for notifications
	var notifyResult map[string]interface{}
	if err := notifyFuture.Get(ctx, &notifyResult); err != nil {
		logger.Warn("Notifications had some failures", "error", err)
	}

	// Final activity
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "FinalizeOrder", "duration": 300, "phase": 1,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":        "completed",
		"orderId":       input["orderId"],
		"orderResult":   orderResult,
		"notifications": notifyResult,
		"childCount":    2,
		"grandchildren": 7,
	}, nil
}

// OrderProcessingChild handles order fulfillment and payment as child workflows
func OrderProcessingChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("OrderProcessingChild started")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Validate order first
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "ValidateOrder", "duration": 400, "phase": 2,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Start FulfillmentChild and PaymentProcessingChild in parallel
	fulfillCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-fulfillment",
	})
	fulfillFuture := workflow.ExecuteChildWorkflow(fulfillCtx, FulfillmentChild, map[string]interface{}{
		"orderId": input["orderId"],
	})

	paymentCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-payment",
	})
	paymentFuture := workflow.ExecuteChildWorkflow(paymentCtx, PaymentProcessingChild, map[string]interface{}{
		"orderId": input["orderId"],
		"amount":  99.99,
	})

	// Wait for both
	var fulfillResult, paymentResult map[string]interface{}
	if err := fulfillFuture.Get(ctx, &fulfillResult); err != nil {
		return nil, fmt.Errorf("fulfillment failed: %w", err)
	}
	if err := paymentFuture.Get(ctx, &paymentResult); err != nil {
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	return map[string]interface{}{
		"status":      "processed",
		"fulfillment": fulfillResult,
		"payment":     paymentResult,
	}, nil
}

// FulfillmentChild handles warehouse and shipping as grandchildren
func FulfillmentChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("FulfillmentChild started")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Start warehouse operations
	warehouseCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-warehouse",
	})
	warehouseFuture := workflow.ExecuteChildWorkflow(warehouseCtx, WarehouseGrandchild, input)

	// Wait for warehouse to complete before shipping
	var warehouseResult map[string]interface{}
	if err := warehouseFuture.Get(ctx, &warehouseResult); err != nil {
		return nil, fmt.Errorf("warehouse failed: %w", err)
	}

	// Now start shipping
	shippingCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-shipping",
	})
	var shippingResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(shippingCtx, ShippingGrandchild, map[string]interface{}{
		"orderId":       input["orderId"],
		"warehouseData": warehouseResult,
	}).Get(ctx, &shippingResult); err != nil {
		return nil, fmt.Errorf("shipping failed: %w", err)
	}

	return map[string]interface{}{
		"status":    "fulfilled",
		"warehouse": warehouseResult,
		"shipping":  shippingResult,
	}, nil
}

// WarehouseGrandchild handles inventory reservation and picking
func WarehouseGrandchild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Reserve inventory
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "ReserveInventory", "duration": 600, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Pick items
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "PickItems", "duration": 800, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Pack order
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "PackOrder", "duration": 500, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":    "packed",
		"warehouse": "WH-001",
		"location":  "Aisle-5-Shelf-3",
	}, nil
}

// ShippingGrandchild handles carrier selection and label generation
func ShippingGrandchild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Select carrier
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "SelectCarrier", "duration": 300, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Generate shipping label
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "GenerateLabel", "duration": 400, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Schedule pickup
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "SchedulePickup", "duration": 500, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":         "shipped",
		"carrier":        "FastShip",
		"trackingNumber": fmt.Sprintf("TRK%d", rand.Intn(1000000)),
	}, nil
}

// PaymentProcessingChild handles fraud check and charging as grandchildren
func PaymentProcessingChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("PaymentProcessingChild started")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Run fraud check first
	fraudCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-fraud-check",
	})
	var fraudResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(fraudCtx, FraudCheckGrandchild, input).Get(ctx, &fraudResult); err != nil {
		return nil, fmt.Errorf("fraud check failed: %w", err)
	}

	// If fraud check passed, charge the card
	chargeCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-charge",
	})
	var chargeResult map[string]interface{}
	if err := workflow.ExecuteChildWorkflow(chargeCtx, ChargeGrandchild, map[string]interface{}{
		"orderId":     input["orderId"],
		"amount":      input["amount"],
		"fraudResult": fraudResult,
	}).Get(ctx, &chargeResult); err != nil {
		return nil, fmt.Errorf("charge failed: %w", err)
	}

	return map[string]interface{}{
		"status":      "paid",
		"fraudCheck":  fraudResult,
		"chargeId":    chargeResult["chargeId"],
	}, nil
}

// FraudCheckGrandchild runs fraud detection
func FraudCheckGrandchild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Run fraud analysis
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "AnalyzeRiskFactors", "duration": 400, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "CheckBlacklist", "duration": 200, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "CalculateFraudScore", "duration": 300, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":     "passed",
		"riskScore":  rand.Intn(30), // Low risk
		"confidence": 0.95,
	}, nil
}

// ChargeGrandchild handles the actual payment charge
func ChargeGrandchild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Authorize payment
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "AuthorizePayment", "duration": 500, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	// Capture payment
	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "CapturePayment", "duration": 400, "phase": 3,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":   "charged",
		"chargeId": fmt.Sprintf("ch_%d", rand.Intn(100000)),
		"amount":   input["amount"],
	}, nil
}

// NotificationFanout sends notifications through multiple channels as children
func NotificationFanout(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("NotificationFanout started")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	// Wait for order completion signal (with timeout)
	signalChan := workflow.GetSignalChannel(ctx, "order-complete")
	var signalData map[string]interface{}

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &signalData)
		logger.Info("Received order-complete signal", "data", signalData)
	})
	selector.AddFuture(workflow.NewTimer(ctx, 30*time.Second), func(f workflow.Future) {
		logger.Info("Signal timeout - proceeding anyway")
	})
	selector.Select(ctx)

	// Start all notification children in parallel
	emailCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-email",
	})
	emailFuture := workflow.ExecuteChildWorkflow(emailCtx, EmailNotificationChild, map[string]interface{}{
		"orderId": input["orderId"],
		"type":    "order_confirmation",
	})

	smsCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-sms",
	})
	smsFuture := workflow.ExecuteChildWorkflow(smsCtx, SMSNotificationChild, map[string]interface{}{
		"orderId": input["orderId"],
		"message": "Your order is confirmed!",
	})

	pushCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: workflowID + "-push",
	})
	pushFuture := workflow.ExecuteChildWorkflow(pushCtx, PushNotificationChild, map[string]interface{}{
		"orderId": input["orderId"],
		"title":   "Order Confirmed",
	})

	// Collect results (don't fail if one notification fails)
	results := map[string]interface{}{}

	if err := emailFuture.Get(ctx, nil); err != nil {
		results["email"] = "failed"
	} else {
		results["email"] = "sent"
	}

	if err := smsFuture.Get(ctx, nil); err != nil {
		results["sms"] = "failed"
	} else {
		results["sms"] = "sent"
	}

	if err := pushFuture.Get(ctx, nil); err != nil {
		results["push"] = "failed"
	} else {
		results["push"] = "sent"
	}

	return map[string]interface{}{
		"status":   "notified",
		"channels": results,
	}, nil
}

// EmailNotificationChild sends email notification
func EmailNotificationChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "RenderEmailTemplate", "duration": 300, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "SendEmail", "duration": 400, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{"channel": "email", "status": "sent"}, nil
}

// SMSNotificationChild sends SMS notification
func SMSNotificationChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "FormatSMSMessage", "duration": 200, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "SendSMS", "duration": 300, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{"channel": "sms", "status": "sent"}, nil
}

// PushNotificationChild sends push notification
func PushNotificationChild(ctx workflow.Context, input map[string]interface{}) (map[string]interface{}, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "BuildPushPayload", "duration": 200, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, GanttActivity, map[string]interface{}{
		"name": "SendPushNotification", "duration": 250, "phase": 4,
	}).Get(ctx, nil); err != nil {
		return nil, err
	}

	return map[string]interface{}{"channel": "push", "status": "sent"}, nil
}
