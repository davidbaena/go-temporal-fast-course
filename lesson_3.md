# Lesson 3: Workers & Task Queues

## Learning Objectives
By the end of this lesson you will:
- ✅ Understand what a worker does and how it polls for tasks
- ✅ Know how task queues route work to workers
- ✅ Register workflows and activities correctly
- ✅ Configure worker options (metrics, concurrency, interceptors basics)
- ✅ Run and observe a worker servicing an order workflow

[← Back to Course Index](course.md) | [← Previous: Lesson 2](lesson_2.md) | [Next: Lesson 4 →](lesson_4.md)

---
## Why Before How: Why Do We Need Workers?
A Temporal **worker** is a long-lived process that:
1. Polls the Temporal server for workflow tasks (decisions) and activity tasks
2. Executes your registered workflow or activity code
3. Reports results back to the server

Without a worker, the Temporal server has your workflow state but **no code to execute**. The worker brings your business logic to life.

**Analogy:** The server is the kitchen coordinator; the worker is the chef that actually cooks each step.

**Durability:** If a worker crashes mid-execution:
- Activities may retry on another worker
- Workflow execution state is safe in the server and will resume when a worker picks up the next task

---
## Task Queues: Routing Work
A **task queue** is a named channel that associates workflow/activity tasks to workers.

In your current code:
```go
w := worker.New(c, "order-task-queue", worker.Options{})
```
This tells Temporal: "Send tasks for workflows/activities started with task queue `order-task-queue` to this worker." Your starter uses:
```go
workflowOptions := client.StartWorkflowOptions{
    ID:        "order_workflow_12345",
    TaskQueue: "order-task-queue",
}
```
If these names differ, your workflow would sit idle forever.

**Best Practices:**
- Use different task queues for different workload classes (e.g., `orders`, `emails`, `reports`)
- Avoid overloading a single queue with unrelated latency profiles
- Use prefixes or domains: `order-processing`, `user-onboarding`, `inventory-sync`

---
## Dissecting Your Existing Worker
Current file: `worker/main.go`
```go
c, err := client.Dial(client.Options{})
...
w := worker.New(c, "order-task-queue", worker.Options{})

w.RegisterWorkflow(order.OrderWorkflow)
w.RegisterActivity(order.ReserveStock)
w.RegisterActivity(order.ProcessPayment)
w.RegisterActivity(order.UpdateOrderStatus)

err = w.Run(worker.InterruptCh())
```
### What This Does:
1. Creates a client (gRPC connection to server)
2. Creates a worker bound to `order-task-queue`
3. Registers 1 workflow + 3 activities
4. Starts polling until interrupted (Ctrl+C)

### Improvements To Consider:
| Area | Improvement |
|------|------------|
| Logging | Use structured logging with workflow/activity context |
| Retries | Add activity retry policies inside workflow code |
| Separation | Group activities behind a struct for DI/testing |
| Metrics | Enable Prometheus via worker options |
| Interceptors | Add logging/tracing interceptor for advanced use |

---
## Worker Options Deep Dive
Common options:
```go
worker.Options{
    Identity: "orders-worker-1", // Helps trace logs
    MaxConcurrentActivityExecutionSize: 200,
    MaxConcurrentWorkflowTaskExecutionSize: 100,
    DisableWorkflowWorker: false,  // Set true if only running activities
    DisableActivityWorker: false,  // Set true if only running workflows
    BackgroundActivityContext: context.WithValue(context.Background(), key, value),
}
```
**Scaling:** Run multiple identical worker processes to scale horizontally. Temporal will distribute tasks fairly.

**Identity Usage:** If you run 5 containers of the same worker:
```
Identity: orders-worker-<pod-name>
```
This helps debug which instance handled which task.

---
## Registering Workflows & Activities (Patterns)
You can register either function references or names:
```go
w.RegisterWorkflow(OrderWorkflow)          // reference
w.RegisterActivity(ReserveStock)
```
Alternatively provide names if you want stable references when refactoring:
```go
w.RegisterWorkflowWithOptions(OrderWorkflow, workflow.RegisterOptions{Name: "OrderWorkflow"})
w.RegisterActivityWithOptions(ReserveStock, activity.RegisterOptions{Name: "ReserveStock"})
```
Then invoke by string name in workflow:
```go
workflow.ExecuteActivity(ctx, "ReserveStock", orderID)
```
**Why use names?** Decouple invocation from function identity → easier refactors, cross-language bridging.

---
## Activity Retry Policy (Improvement to Existing Workflow)
Your current `OrderWorkflow` invokes `ProcessPayment` which sometimes fails randomly:
```go
if rand.Float32() < 0.5 { ... }
```
We should wrap activities with a retry policy:
```go
ao := workflow.ActivityOptions{
    StartToCloseTimeout: 10 * time.Second,
    RetryPolicy: &workflow.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,
        MaximumAttempts:    5,
        NonRetryableErrorTypes: []string{"ValidationError"},
    },
}
ctx = workflow.WithActivityOptions(ctx, ao)
```
This makes `ProcessPayment` resilient to transient failure.

---
## Adding Structured Logging
Inside activities:
```go
logger := activity.GetLogger(ctx)
logger.Info("ReserveStock", "orderID", orderID)
```
Inside workflow:
```go
logger := workflow.GetLogger(ctx)
logger.Info("Workflow started", "orderID", orderID)
```
Logs automatically include WorkflowID and RunID context.

---
## Full Enhanced Worker Example
Create (or modify) `worker/main.go` to:
```go
package main

import (
    "log"
    "os"

    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"

    "go-temporal-fast-course/order"
)

func main() {
    c, err := client.Dial(client.Options{})
    if err != nil {
        log.Fatalln("Unable to create client", err)
    }
    defer c.Close()

    taskQueue := getenv("ORDER_TASK_QUEUE", "order-task-queue")

    w := worker.New(c, taskQueue, worker.Options{
        Identity: "orders-worker-" + hostname(),
    })

    // Register workflow and activities
    w.RegisterWorkflow(order.OrderWorkflow)
    w.RegisterActivity(order.ReserveStock)
    w.RegisterActivity(order.ProcessPayment)
    w.RegisterActivity(order.UpdateOrderStatus)

    if err := w.Run(worker.InterruptCh()); err != nil {
        log.Fatalln("Unable to start worker", err)
    }
}

func hostname() string {
    h, _ := os.Hostname()
    return h
}

func getenv(key, def string) string {
    v := os.Getenv(key)
    if v == "" { return def }
    return v
}
```
**Why?** Easier containerization and debugging in multi-instance environments.

---
## Starter (Client) Recap
File: `starter/main.go`
Responsibilities:
1. Create Temporal client
2. Start workflow with `ExecuteWorkflow`
3. Optionally block for result (`we.Get()`)

You can also start asynchronously and store WorkflowID/RunID for later queries.
```go
we, err := c.ExecuteWorkflow(ctx, options, order.OrderWorkflow, orderID)
log.Println("Started", we.GetID(), we.GetRunID())
```
Query status later:
```go
err := c.GetWorkflow(ctx, workflowID, runID).Get(ctx, &result)
```

---
## Running It All (Local Flow)
1. Ensure Temporal server is running (docker-compose) 
2. Start worker (polls for tasks)
3. Start workflow (starter client)
4. Observe workflow in Web UI

Commands:
```bash
# 1. Start Temporal (if not already)
docker-compose up -d temporal

# 2. Run worker (in one terminal)
go run worker/main.go

# 3. Run starter (in another terminal)
go run starter/main.go

# 4. View UI (browser)
open http://localhost:8080
```

---
## Edge Cases & Considerations
| Scenario | Handling Strategy |
|----------|-------------------|
| Activity failure | Configure retry or catch and compensate |
| Worker crash | Workflow state preserved; start new worker |
| Stalled queue | Inspect task queue metrics, scale workers |
| High latency activity | Increase StartToCloseTimeout, use heartbeats |
| Refactor workflow name | Use `RegisterWorkflowWithOptions` with stable name |

---
## Exercise
1. Duplicate the worker to a new file `worker/payment_worker.go` that only registers `ProcessPayment` activity (set `DisableWorkflowWorker: true`).
2. Change `OrderWorkflow` to call `ProcessPayment` via a separate task queue (`payment-task-queue`).
3. Start both workers; observe separation of concerns.
4. Add an environment variable override for payment queue: `PAYMENT_TASK_QUEUE`.

**Stretch:** Add a retry policy that escalates to manual review after 5 failed attempts (simulate by logging a message instead of failing silently).

---
## What You've Learned
✅ Worker responsibilities and lifecycle  
✅ Task queue routing and naming strategy  
✅ Registering workflows vs activities  
✅ Worker options for scaling & identity  
✅ Starting workflows via starter client  
✅ How to enhance reliability (retry policy)  
✅ Separation via multiple task queues  

---
## Ready for Lesson 4?
Lesson 4 will cover: 
- Bringing up Temporal with docker-compose (with correct images)
- Running end-to-end locally
- Using the Temporal UI to inspect history
- Querying workflow status externally

Say: **"I'm ready for Lesson 4"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 2](lesson_2.md) | [Next: Lesson 4 →](lesson_4.md)

