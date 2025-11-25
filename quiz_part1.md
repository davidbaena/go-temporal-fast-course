# Quiz: Part 1 Foundations

**Test your understanding of Temporal fundamentals!**

This quiz covers Lessons 1-3: core concepts, workflows & activities, and workers & task queues.

**Instructions:**
- Try to answer each question before revealing the answer
- Read the explanations even if you get it right - they contain valuable insights
- If you miss a question, review the corresponding lesson

---

## 🎯 Lesson 1: What is Temporal?

### Question 1: Core Purpose

**What is the primary guarantee that Temporal provides for your workflows?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Temporal guarantees that workflows **run to completion** despite failures, restarts, or infrastructure issues.

**Why this matters:**
- Your workflow code will resume exactly where it left off after any failure
- You don't need to build custom retry logic, state persistence, or failure recovery
- The workflow execution history is durably stored and can replay deterministically

**Key insight:** This is Temporal's core value proposition - durable execution that survives process crashes, network failures, and even datacenter outages.

📖 **Review:** [Lesson 1](lesson_1.md#temporal-guarantees)

</details>

---

### Question 2: Building Blocks

**Which of the following are the 5 fundamental building blocks of Temporal? (Select all that apply)**

A) Workflow
B) Database
C) Activity
D) Worker
E) API Gateway
F) Task Queue
G) Execution

<details>
<summary>Click to reveal answer</summary>

**Answer:** A, C, D, F, G

The **5 fundamental building blocks** are:
1. **Workflow** - Durable function that orchestrates business logic
2. **Activity** - Single, well-defined action (database call, API request, etc.)
3. **Worker** - Process that polls task queues and executes workflows/activities
4. **Task Queue** - Named queue that routes tasks to workers
5. **Execution** - A single run of a workflow with unique ID and history

**Why the others are wrong:**
- **Database** - While Temporal uses a database internally, it's not a building block you interact with
- **API Gateway** - Not part of Temporal's architecture (though you might use one in your system)

📖 **Review:** [Lesson 1](lesson_1.md#five-fundamental-building-blocks)

</details>

---

### Question 3: Use Cases

**Which of these scenarios is a POOR fit for Temporal?**

A) Processing a multi-step order that takes 2 days and involves payment, shipping, and notifications
B) Computing the sum of two numbers and returning the result
C) Running a monthly payroll process that charges cards, generates reports, and sends emails
D) Orchestrating a microservices saga with compensation logic

<details>
<summary>Click to reveal answer</summary>

**Answer:** B - Computing the sum of two numbers and returning the result

**Why B is a poor fit:**
- This is a **simple, synchronous operation** that completes in milliseconds
- No need for durable execution, retry logic, or state management
- The overhead of Temporal would add unnecessary complexity
- A simple HTTP endpoint or function call is more appropriate

**Why the others are GOOD fits:**
- **A (Multi-step order):** Long-running, requires coordination, needs fault tolerance
- **C (Monthly payroll):** Complex orchestration, multiple external systems, compensation needed
- **D (Microservices saga):** Distributed transactions, compensation patterns, state management

**Rule of thumb:** Use Temporal when you need:
- Multi-step processes
- Fault tolerance and retries
- Long-running operations (minutes to months)
- Compensation/rollback logic
- Coordination across services

📖 **Review:** [Lesson 1](lesson_1.md#when-to-use-temporal)

</details>

---

### Question 4: Architecture

**Where does Temporal store the workflow execution history?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** In a **durable database** (PostgreSQL, MySQL, or Cassandra) managed by the Temporal Server.

**Why this is critical:**
- The history is the **source of truth** for workflow state
- Even if all workers crash, the history persists
- When a workflow resumes, it **replays** the history to reconstruct state
- This enables Temporal's core guarantee: workflows always run to completion

**What's in the history:**
- Every event: workflow started, activity scheduled, activity completed, timers fired, etc.
- Input parameters and return values
- Signals received and queries executed

**Performance note:** This is why you should avoid storing large payloads directly in workflow state - it bloats the history.

📖 **Review:** [Lesson 1](lesson_1.md#architecture-overview)

</details>

---

## 🔧 Lesson 2: Workflows & Activities

### Question 5: Determinism

**Which of these operations would VIOLATE determinism in a workflow function?**

A) Calling `workflow.Now(ctx)` to get the current time
B) Calling `time.Now()` to get the current time
C) Calling `workflow.ExecuteActivity()` to run an activity
D) Using `workflow.Sleep(ctx, duration)` to wait

<details>
<summary>Click to reveal answer</summary>

**Answer:** B - Calling `time.Now()` directly

**Why this violates determinism:**
- When a workflow **replays** from history, `time.Now()` would return a different value each time
- This makes the workflow **non-deterministic** - same input, different output
- Replays could take different code paths, causing errors or unexpected behavior

**The deterministic alternatives:**
- **`workflow.Now(ctx)`** - Returns the timestamp from the workflow event, consistent across replays
- **`workflow.Sleep(ctx, duration)`** - Uses durable timers stored in history
- **`workflow.ExecuteActivity()`** - Activity results are stored in history

**Other non-deterministic operations to avoid:**
```go
// ❌ Don't do this in workflows:
time.Now()                    // Use workflow.Now(ctx)
rand.Intn(10)                 // Use workflow.SideEffect()
uuid.New()                    // Use workflow.SideEffect()
http.Get("api.com/data")      // Use an Activity
db.Query("SELECT...")         // Use an Activity
```

**Golden rule:** Workflows must be pure, deterministic functions. All side effects go in Activities.

📖 **Review:** [Lesson 2](lesson_2.md#understanding-determinism)

</details>

---

### Question 6: Workflow vs Activity

**You need to charge a customer's credit card during order processing. Should this be in the Workflow or an Activity?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** **Activity**

**Why it must be an Activity:**

1. **Non-deterministic:** Payment gateway responses vary and aren't predictable
2. **Side effect:** It changes external state (charges the card)
3. **Network I/O:** Calls external API
4. **Needs retries:** Payment might fail temporarily (network issue, gateway timeout)
5. **Not idempotent in workflow:** If the workflow replays, you don't want to charge twice!

**The proper pattern:**
```go
// ✅ Correct: Activity with retry policy
func ChargePaymentWorkflow(ctx workflow.Context, orderID string, amount float64) error {
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
            InitialInterval: 1 * time.Second,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)

    var paymentID string
    err := workflow.ExecuteActivity(ctx, ChargePaymentActivity, orderID, amount).Get(ctx, &paymentID)
    return err
}

// Activity implementation
func ChargePaymentActivity(ctx context.Context, orderID string, amount float64) (string, error) {
    // Safe to call external payment API here
    return paymentGateway.Charge(orderID, amount)
}
```

**What belongs in Workflows:**
- Orchestration logic (call this, then that)
- Conditional branching based on activity results
- Timers and delays
- State management

**What belongs in Activities:**
- API calls
- Database operations
- File I/O
- Anything non-deterministic

📖 **Review:** [Lesson 2](lesson_2.md#workflows-vs-activities)

</details>

---

### Question 7: Activity Options

**What happens if an Activity exceeds its `StartToCloseTimeout`?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Temporal will **fail the activity** and **retry it** according to the RetryPolicy (if configured).

**Timeline breakdown:**

1. **Activity starts** executing
2. **Timeout exceeded** (e.g., 30 seconds pass)
3. Temporal marks activity as **failed with timeout error**
4. **RetryPolicy kicks in:**
   - If retries remain: schedule retry with backoff
   - If max attempts reached: activity fails permanently
5. **Workflow receives error** (if no more retries)

**Example scenario:**
```go
activityOptions := workflow.ActivityOptions{
    StartToCloseTimeout: 10 * time.Second,  // Must complete in 10s
    RetryPolicy: &temporal.RetryPolicy{
        MaximumAttempts: 3,                  // Try up to 3 times
        InitialInterval: 1 * time.Second,
    },
}
```

If the activity takes 15 seconds:
- Attempt 1: Times out after 10s → Retry
- Attempt 2: Times out after 10s → Retry
- Attempt 3: Times out after 10s → Permanent failure

**Types of timeouts:**
- **StartToCloseTimeout:** Total time for single attempt (most common)
- **ScheduleToCloseTimeout:** Total time including all retries
- **ScheduleToStartTimeout:** Max time in queue before starting
- **HeartbeatTimeout:** Max time between heartbeat signals

**Best practice:** Always set timeouts to prevent stuck activities from blocking workflows forever.

📖 **Review:** [Lesson 2](lesson_2.md#activity-options-and-timeouts)

</details>

---

### Question 8: Workflow Patterns

**What's the benefit of using parallel activities instead of sequential?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** **Faster total execution time** when activities are independent and can run concurrently.

**Sequential Pattern (slower):**
```go
// Total time: 30s + 30s = 60 seconds
err := workflow.ExecuteActivity(ctx, SendEmail, order).Get(ctx, nil)      // 30s
err = workflow.ExecuteActivity(ctx, UpdateInventory, order).Get(ctx, nil) // 30s
```

**Parallel Pattern (faster):**
```go
// Total time: max(30s, 30s) = 30 seconds
emailFuture := workflow.ExecuteActivity(ctx, SendEmail, order)
inventoryFuture := workflow.ExecuteActivity(ctx, UpdateInventory, order)

// Wait for both
err := emailFuture.Get(ctx, nil)
err = inventoryFuture.Get(ctx, nil)
```

**Use parallel when:**
- Activities are **independent** (no data dependencies)
- You want to **reduce latency**
- Activities can safely run **concurrently**

**Example use case - Order processing:**
```go
// After payment succeeds, do these in parallel:
emailFuture := workflow.ExecuteActivity(ctx, SendConfirmationEmail, orderID)
slackFuture := workflow.ExecuteActivity(ctx, NotifySlack, orderID)
metricsFuture := workflow.ExecuteActivity(ctx, RecordMetrics, orderID)

// Wait for all notifications (even if one fails, others continue)
```

**Watch out for:**
- Activities that depend on each other (must be sequential)
- Resource constraints (don't overload downstream systems)
- Error handling (one failure shouldn't block independent work)

📖 **Review:** [Lesson 2](lesson_2.md#workflow-patterns)

</details>

---

## ⚙️ Lesson 3: Workers & Task Queues

### Question 9: Worker Basics

**What does a Worker do?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** A Worker **polls** task queues for tasks, then **executes** the corresponding workflow or activity code.

**How it works:**

1. **Registration:** Worker registers which workflows/activities it can execute
   ```go
   w := worker.New(c, "order-processing", worker.Options{})
   w.RegisterWorkflow(OrderWorkflow)
   w.RegisterActivity(ChargePaymentActivity)
   ```

2. **Polling:** Worker long-polls the task queue asking "any work for me?"

3. **Execution:** Temporal sends a task, worker executes the code

4. **Result:** Worker returns result to Temporal, which stores it in history

5. **Repeat:** Worker immediately polls for next task

**Key characteristics:**
- Workers are **stateless** - they don't store workflow state between tasks
- Multiple workers can run concurrently for **horizontal scaling**
- Workers can **crash and restart** without losing workflow progress
- Task assignment is **automatic** via task queue polling

**Think of it like:**
- Task Queue = Restaurant order queue
- Worker = Chef who takes orders and cooks them
- Multiple chefs (workers) can work from same queue (task queue)

📖 **Review:** [Lesson 3](lesson_3.md#how-workers-work)

</details>

---

### Question 10: Task Queues

**Why would you use multiple task queues instead of just one?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Multiple task queues allow you to **route different types of work to specialized workers**.

**Common scenarios:**

**1. Resource Isolation**
```go
// CPU-intensive tasks on dedicated workers
"video-processing-queue"     → Workers with GPUs
"data-analysis-queue"        → Workers with lots of RAM
"standard-queue"             → Standard workers
```

**2. Environment Separation**
```go
"production-queue"           → Production workers
"staging-queue"              → Staging workers
"development-queue"          → Dev workers (same code, different configs)
```

**3. Tenant Isolation**
```go
"customer-premium-queue"     → Fast, dedicated workers
"customer-standard-queue"    → Shared workers
```

**4. Priority Handling**
```go
"high-priority-queue"        → More workers, faster processing
"low-priority-queue"         → Fewer workers, background jobs
```

**5. Dependency Isolation**
```go
"payments-queue"             → Workers with payment SDK
"shipping-queue"             → Workers with shipping SDK
"notifications-queue"        → Workers with email/SMS clients
```

**Example:**
```go
// Start workflow on specific queue
workflowOptions := client.StartWorkflowOptions{
    ID:        "order-12345",
    TaskQueue: "high-priority-queue",  // Route to fast workers
}

// Worker listens to specific queue
w := worker.New(c, "high-priority-queue", worker.Options{
    MaxConcurrentActivityExecutionSize: 100,  // Handle more load
})
```

**Benefits:**
- ✅ Better resource utilization
- ✅ Blast radius containment (failures don't affect other queues)
- ✅ Independent scaling per workload type
- ✅ Clear separation of concerns

📖 **Review:** [Lesson 3](lesson_3.md#task-queue-routing)

</details>

---

### Question 11: Worker Scaling

**Your order processing workflow is getting slow during peak hours. What should you do?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** **Run more worker instances** (horizontal scaling) to process tasks faster.

**How it works:**

**Before (1 worker, slow):**
```
Task Queue: [Task1] [Task2] [Task3] [Task4] [Task5]
                ↓
            Worker1 (processing one at a time)
```

**After (3 workers, 3x faster):**
```
Task Queue: [Task1] [Task2] [Task3] [Task4] [Task5]
              ↓       ↓       ↓
           Worker1  Worker2  Worker3 (parallel processing)
```

**Scaling strategies:**

**1. Horizontal Scaling (add more workers):**
```bash
# Kubernetes example
kubectl scale deployment order-worker --replicas=10
```

**2. Tune concurrency per worker:**
```go
w := worker.New(c, "order-processing", worker.Options{
    MaxConcurrentActivityExecutionSize:     50,  // 50 activities at once
    MaxConcurrentWorkflowTaskExecutionSize: 10,  // 10 workflow tasks
})
```

**3. Split by task queue:**
```go
// Heavy work on dedicated queue
w1 := worker.New(c, "payment-processing", worker.Options{})
w1.RegisterActivity(ChargePaymentActivity)

// Light work on separate queue
w2 := worker.New(c, "notifications", worker.Options{})
w2.RegisterActivity(SendEmailActivity)
```

**Monitoring metrics:**
- **Task queue backlog** - How many tasks waiting?
- **Worker utilization** - Are workers busy or idle?
- **Task latency** - Time from scheduled to started
- **Throughput** - Tasks completed per second

**When to scale:**
- ⚠️ Task queue backlog growing
- ⚠️ Task latency increasing
- ⚠️ Workers at max capacity
- ⚠️ SLA violations (orders taking too long)

**Scaling is safe because:**
- ✅ Workers are stateless
- ✅ Temporal handles task distribution automatically
- ✅ No coordination needed between workers
- ✅ Can scale up/down without data loss

📖 **Review:** [Lesson 3](lesson_3.md#worker-scaling)

</details>

---

### Question 12: Registration

**What happens if a worker receives a task for a workflow it hasn't registered?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** The worker will **reject the task** and it will be **re-queued** for another worker (or fail if no worker can handle it).

**The flow:**

1. **Temporal sends task** to available worker
2. **Worker checks:** "Do I have this workflow/activity registered?"
3. **Not found:** Worker returns task to Temporal
4. **Temporal re-queues** task for another worker
5. **If no worker can handle it:** Task fails with error

**Example scenario:**
```go
// Worker 1 - Only has OrderWorkflow
w1 := worker.New(c, "order-queue", worker.Options{})
w1.RegisterWorkflow(OrderWorkflow)  // ✅ Can handle OrderWorkflow
w1.Start()

// Worker 2 - Only has RefundWorkflow
w2 := worker.New(c, "order-queue", worker.Options{})
w2.RegisterWorkflow(RefundWorkflow)  // ✅ Can handle RefundWorkflow
w2.Start()

// Start workflow
client.ExecuteWorkflow(ctx, options, OrderWorkflow, params)
// ✅ Worker 1 processes it
```

**Common mistake:**
```go
// 🚨 Forgot to register activity
w := worker.New(c, "queue", worker.Options{})
w.RegisterWorkflow(OrderWorkflow)  // Registered
// w.RegisterActivity(ChargePayment)  // ❌ FORGOT THIS!
w.Start()

// Workflow runs, but when it tries to execute ChargePayment activity:
// Error: "activity type not registered"
```

**Best practices:**

**1. Register everything the worker should handle:**
```go
w := worker.New(c, "order-queue", worker.Options{})

// Workflows
w.RegisterWorkflow(OrderWorkflow)
w.RegisterWorkflow(RefundWorkflow)

// Activities
w.RegisterActivity(ChargePaymentActivity)
w.RegisterActivity(UpdateInventoryActivity)
w.RegisterActivity(SendEmailActivity)

w.Start()
```

**2. Use consistent registration across worker instances:**
```go
// All workers on "order-queue" should register the same workflows/activities
```

**3. Version carefully when updating:**
```go
// When deploying new version, ensure smooth transition:
// Old workers: Have old workflow code
// New workers: Have new workflow code
// Both can run simultaneously during deployment
```

📖 **Review:** [Lesson 3](lesson_3.md#registering-workflows-and-activities)

</details>

---

## 🎯 Scoring Guide

Count how many you got right on the first try:

- **10-12 correct:** 🌟 Excellent! You have a solid grasp of Temporal fundamentals
- **7-9 correct:** 👍 Good understanding, review the questions you missed
- **4-6 correct:** 📚 You're getting there, revisit the lessons for the topics you struggled with
- **0-3 correct:** 🔄 Take another pass through Part 1 lessons

---

## 📚 What's Next?

Once you feel confident with Part 1 foundations:

**Ready to build?** Continue to **[Lesson 4: Running Your First Workflow](lesson_4.md)**

Or review specific lessons:
- [Lesson 1: What is Temporal?](lesson_1.md)
- [Lesson 2: Workflows & Activities](lesson_2.md)
- [Lesson 3: Workers & Task Queues](lesson_3.md)

---

**Questions or confused about any topic?** Review the lesson materials and experiment with the code examples. Understanding these foundations is crucial for the rest of the course!

---

_Part 1 Quiz • Temporal Fast Course • Last Updated: November 2025_