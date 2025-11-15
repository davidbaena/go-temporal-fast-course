package main

import (
	"log"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"go-temporal-fast-course/order-processing/activities"
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

	// Get task queue name from environment
	taskQueue := getEnv("ORDER_TASK_QUEUE", "order-task-queue")

	// Create worker with options
	w := worker.New(c, taskQueue, worker.Options{
		Identity:                               "order-worker-" + hostname(),
		MaxConcurrentActivityExecutionSize:     100,
		MaxConcurrentWorkflowTaskExecutionSize: 50,
	})

	// Register workflows
	w.RegisterWorkflow(workflows.OrderWorkflow)
	w.RegisterWorkflow(workflows.GreetUser)

	// Register activities
	// Inventory activities
	inventoryActivities := &activities.InventoryActivities{}
	w.RegisterActivity(inventoryActivities.ReserveStock)
	w.RegisterActivity(inventoryActivities.ReleaseStock)
	w.RegisterActivity(inventoryActivities.FetchInventorySnapshot)

	// Payment activities
	paymentActivities := &activities.PaymentActivities{}
	w.RegisterActivity(paymentActivities.ProcessPayment)
	w.RegisterActivity(paymentActivities.RefundPayment)

	// Customer activities
	customerActivities := &activities.CustomerActivities{}
	w.RegisterActivity(customerActivities.FetchCustomerProfile)

	// Recommendation activities
	recommendationActivities := &activities.RecommendationActivities{}
	w.RegisterActivity(recommendationActivities.FetchRecommendations)

	// Order activities
	orderActivities := &activities.OrderActivities{}
	w.RegisterActivity(orderActivities.UpdateOrderStatus)

	// Notification activities
	notificationActivities := &activities.NotificationActivities{}
	w.RegisterActivity(notificationActivities.SendOrderConfirmation)
	w.RegisterActivity(notificationActivities.SendCancellationEmail)

	// Greet activities (for simple example)
	greetActivities := &activities.GreetActivities{}
	w.RegisterActivity(greetActivities.GetUserDetails)
	w.RegisterActivity(greetActivities.GetUserPreferences)
	w.RegisterActivity(greetActivities.SendGreeting)
	w.RegisterActivity(greetActivities.LogGreeting)

	log.Println("Worker starting on task queue:", taskQueue)
	log.Println("Worker identity:", "order-worker-"+hostname())

	// Start worker
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
