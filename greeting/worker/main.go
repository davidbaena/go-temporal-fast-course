package main

import (
	"log"
	"os"

	"go-temporal-fast-course/greeting/activities"
	"go-temporal-fast-course/greeting/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
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
	w.RegisterWorkflow(workflows.GreetUser)

	// Greet activities (for simple example)
	greetActivities := &activities.GreetActivities{}
	w.RegisterActivity(greetActivities.GetUserDetails)
	w.RegisterActivity(greetActivities.SendGreeting)
	w.RegisterActivity(greetActivities.LogGreeting)
	w.RegisterActivity(greetActivities.GetUserPreferencesId)

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
