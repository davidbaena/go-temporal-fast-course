package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"go-temporal-fast-course/greeting/workflows"
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

	runGreetWorkflow(c, taskQueue)
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
	var result workflows.GreetUserOutput
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Workflow execution failed", err)
	}

	log.Printf("✅ Workflow completed successfully!\n")
	log.Printf("Message: %s\n", result.Message)
	log.Printf("Sent at: %s\n", result.SentAt)
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
