package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"go-temporal-fast-course/order-processing/types"
	"go-temporal-fast-course/order-processing/workflows"
)

func main() {
	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: getEnv("TEMPORAL_HOST", "localhost:7233"),
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer c.Close()

	// Get task queue name
	taskQueue := getEnv("ORDER_TASK_QUEUE", "order-task-queue")

	// Determine which workflow to run
	workflowType := getEnv("WORKFLOW_TYPE", "order")

	switch workflowType {
	case "greet":
		runGreetWorkflow(c, taskQueue)
	case "order":
		runOrderWorkflow(c, taskQueue)
	default:
		log.Fatalf("Unknown workflow type: %s (use 'greet' or 'order')", workflowType)
	}
}

func runGreetWorkflow(c client.Client, taskQueue string) {
	// Generate workflow ID
	workflowID := fmt.Sprintf("greet-workflow-%d", time.Now().Unix())

	// Prepare workflow input
	input := workflows.GreetUserInput{
		UserID: getEnv("USER_ID", "user-123"),
	}

	// Configure workflow options
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	log.Printf("Starting GreetUser workflow: %s\n", workflowID)

	// Start workflow
	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflows.GreetUser, input)
	if err != nil {
		log.Fatalln("Unable to start workflow", err)
	}

	log.Printf("Started workflow - WorkflowID: %s, RunID: %s\n", we.GetID(), we.GetRunID())

	// Wait for workflow result
	var result workflows.GreetUserResult
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Workflow execution failed", err)
	}

	log.Printf("✅ Workflow completed successfully!\n")
	log.Printf("Message: %s\n", result.Message)
	log.Printf("Sent at: %s\n", result.SentAt)
}

func runOrderWorkflow(c client.Client, taskQueue string) {
	// Generate workflow and order IDs
	orderID := getEnv("ORDER_ID", fmt.Sprintf("ORDER-%d", time.Now().Unix()))
	workflowID := fmt.Sprintf("order-workflow-%s", orderID)

	// Prepare initial items
	initialItems := []types.LineItem{
		{SKU: "BOOK-001", Quantity: 2},
		{SKU: "PEN-042", Quantity: 5},
	}

	// Configure workflow options
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	log.Printf("Starting OrderWorkflow: %s\n", workflowID)
	log.Printf("Order ID: %s\n", orderID)

	// Start workflow
	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflows.OrderWorkflow, orderID, initialItems)
	if err != nil {
		log.Fatalln("Unable to start workflow", err)
	}

	log.Printf("Started workflow - WorkflowID: %s, RunID: %s\n", we.GetID(), we.GetRunID())
	log.Printf("\n📋 Workflow Management Commands:\n")
	log.Printf("  View in UI: http://localhost:8080/namespaces/default/workflows/%s\n", workflowID)
	log.Printf("\n  Query status:\n")
	log.Printf("    tctl workflow query -w %s -qt get-status\n", workflowID)
	log.Printf("\n  Approve payment:\n")
	log.Printf("    tctl workflow signal -w %s -n approve-payment -i '{\"ApprovedBy\":\"admin\"}'\n", workflowID)
	log.Printf("\n  Cancel order:\n")
	log.Printf("    tctl workflow signal -w %s -n cancel-order -i '{\"Reason\":\"customer requested\"}'\n", workflowID)
	log.Printf("\n  Add item:\n")
	log.Printf("    tctl workflow signal -w %s -n add-line-item -i '{\"SKU\":\"ITEM-999\",\"Quantity\":3}'\n", workflowID)

	// Check if we should wait for completion or run async
	if getEnv("ASYNC", "false") == "true" {
		log.Printf("\n🚀 Workflow started asynchronously. Use the commands above to interact.\n")
		return
	}

	log.Printf("\n⏳ Waiting for workflow to complete (send approval signal to proceed)...\n")

	// Optional: Send approval automatically after a delay for testing
	if getEnv("AUTO_APPROVE", "false") == "true" {
		go func() {
			time.Sleep(2 * time.Second)
			log.Printf("\n🤖 Auto-approving payment...\n")
			err := c.SignalWorkflow(
				context.Background(),
				workflowID,
				"",
				"approve-payment",
				types.PaymentApproval{ApprovedBy: "auto-approver", Timestamp: time.Now()},
			)
			if err != nil {
				log.Printf("Failed to send approval signal: %v\n", err)
			}
		}()
	}

	// Wait for workflow result
	var result string
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalf("❌ Workflow execution failed: %v\n", err)
	}

	log.Printf("\n✅ Workflow completed successfully!\n")
	log.Printf("Result: %s\n", result)

	// Query final status
	queryResp, err := c.QueryWorkflow(context.Background(), workflowID, "", "get-status")
	if err != nil {
		log.Printf("Failed to query status: %v\n", err)
	} else {
		var status types.OrderWorkflowStatus
		if err := queryResp.Get(&status); err == nil {
			log.Printf("\n📊 Final Status:\n")
			log.Printf("  Stage: %s\n", status.Stage)
			log.Printf("  Items: %d\n", len(status.Items))
			log.Printf("  Reserved: %v\n", status.Reserved)
			log.Printf("  Charged: %v\n", status.Charged)
			log.Printf("  Version: %s\n", status.Version)
		}
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
