# Quiz: Part 2 Building & Running

**Test your practical knowledge of running Temporal workflows!**

This quiz covers Lessons 4-6: running workflows, error handling & retries, and signals & queries.

**Instructions:**
- Try to answer each question before revealing the answer
- Read the explanations even if you get it right - they contain valuable insights
- If you miss a question, review the corresponding lesson

---

## 🚀 Lesson 4: Running Your First Workflow

### Question 1: Starting Temporal Server

**What's the recommended way to run Temporal Server locally for development?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use **Docker Compose** with the official Temporal images.

**Why Docker Compose:**
- ✅ **One command** to start entire stack (server, UI, database)
- ✅ **Consistent environment** across team members
- ✅ **No complex installation** - just Docker Desktop
- ✅ **Production-like setup** with proper dependencies
- ✅ **Easy cleanup** - tear down with one command

**Example docker-compose.yml:**
```yaml
version: '3.8'
services:
  temporal:
    image: temporalio/auto-setup:latest
    ports:
      - "7233:7233"  # gRPC endpoint
    environment:
      - DB=postgresql

  temporal-ui:
    image: temporalio/ui:latest
    ports:
      - "8233:8233"  # Web UI
    depends_on:
      - temporal

  postgresql:
    image: postgres:13
    environment:
      POSTGRES_PASSWORD: temporal
      POSTGRES_USER: temporal
```

**Starting the stack:**
```bash
docker-compose up -d
```

**Accessing:**
- Server: `localhost:7233` (for clients/workers)
- Web UI: `http://localhost:8233` (for monitoring)

**Alternatives (not recommended for dev):**
- ❌ **Temporal CLI** - Good for quick tests, but limited features
- ❌ **Building from source** - Complex, slow, unnecessary for most users
- ❌ **Cloud immediately** - Overkill for learning/local dev

📖 **Review:** [Lesson 4](lesson_4.md#setting-up-temporal-server)

</details>

---

### Question 2: Workflow Client

**What information do you MUST provide when starting a workflow execution?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** You must provide:
1. **Workflow ID** (unique identifier)
2. **Task Queue** name (where to route the work)
3. **Workflow function** (which workflow to execute)

**Example:**
```go
client, err := client.Dial(client.Options{
    HostPort: "localhost:7233",
})

workflowOptions := client.StartWorkflowOptions{
    ID:        "order-12345",              // 1. Unique ID
    TaskQueue: "order-processing",         // 2. Task queue name
}

// 3. Workflow function + parameters
workflowRun, err := client.ExecuteWorkflow(
    context.Background(),
    workflowOptions,
    OrderWorkflow,                          // The workflow function
    orderParams,                            // Workflow parameters
)
```

**Why each is required:**

**1. Workflow ID:**
- Ensures **idempotency** - same ID won't create duplicate workflows
- Allows you to **query/signal** the workflow later
- Enables **deduplication** (retry-safe client calls)

**2. Task Queue:**
- Routes work to correct **worker pool**
- Enables **isolation** and **scaling**
- Allows **environment separation** (dev/staging/prod)

**3. Workflow Function:**
- Tells Temporal **which code** to execute
- Worker must have this workflow **registered**

**Optional (but commonly used):**
```go
workflowOptions := client.StartWorkflowOptions{
    ID:                       "order-12345",
    TaskQueue:                "order-processing",
    WorkflowExecutionTimeout: 24 * time.Hour,      // Max total duration
    WorkflowRunTimeout:       1 * time.Hour,       // Max single run
    WorkflowTaskTimeout:      10 * time.Second,    // Max decision time
    RetryPolicy:              &temporal.RetryPolicy{...},
}
```

**Common mistake:**
```go
// ❌ Don't generate random IDs - lose idempotency!
ID: uuid.New().String()  // Every retry creates new workflow

// ✅ Use business identifier
ID: fmt.Sprintf("order-%s", orderID)  // Idempotent
```

📖 **Review:** [Lesson 4](lesson_4.md#creating-workflow-client)

</details>

---

### Question 3: Web UI

**What can you see in the Temporal Web UI for a completed workflow execution?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** The Web UI shows the complete **execution history** with all events, inputs/outputs, timelines, and metadata.

**What you can inspect:**

**1. Event History (the most important!):**
```
WorkflowExecutionStarted
  Input: {"orderID": "12345", "amount": 99.99}

WorkflowTaskScheduled
WorkflowTaskStarted

ActivityTaskScheduled (ValidateOrder)
ActivityTaskStarted (ValidateOrder)
ActivityTaskCompleted (ValidateOrder)
  Result: {"valid": true}

ActivityTaskScheduled (ChargePayment)
ActivityTaskStarted (ChargePayment)
ActivityTaskCompleted (ChargePayment)
  Result: {"transactionID": "tx-789"}

WorkflowExecutionCompleted
  Result: {"orderID": "12345", "status": "success"}
```

**2. Timeline View:**
- Visual representation of workflow duration
- Activity execution times
- Gaps (waiting/sleeping)
- Retries and failures

**3. Summary Information:**
- Workflow ID and Run ID
- Status (Running, Completed, Failed, Canceled, Terminated)
- Start and end times
- Task queue name
- Parent workflow (if child)

**4. Input/Output:**
- Workflow input parameters
- Final result or error
- Activity inputs/outputs

**5. Workers:**
- Which worker executed tasks
- Worker identity and build info

**6. Pending Activities:**
- Currently running activities
- Scheduled but not started
- Retry attempts

**Why this is powerful:**
- 🔍 **Full observability** - see exactly what happened
- 🐛 **Debugging** - understand failures step-by-step
- 📊 **Performance analysis** - identify slow activities
- 🔄 **Replay** - understand state at any point in time
- 📝 **Audit trail** - compliance and troubleshooting

**Common use cases:**

**Debugging a failure:**
```
1. Open workflow in UI
2. Find the failed activity in history
3. Check the error message and stack trace
4. See the input that caused the failure
5. Check retry attempts
```

**Performance investigation:**
```
1. View timeline
2. Identify long-running activities
3. Check if parallel activities ran correctly
4. Look for unexpected delays
```

📖 **Review:** [Lesson 4](lesson_4.md#temporal-web-ui)

</details>

---

### Question 4: Workflow Execution

**What's the difference between Workflow ID and Run ID?**

<details>
<summary>Click to reveal answer</summary>

**Answer:**
- **Workflow ID**: Unique identifier for the **workflow instance** (you provide this)
- **Run ID**: Unique identifier for a **single execution attempt** (Temporal generates this)

**Think of it like:**
- **Workflow ID** = Your order number (`order-12345`)
- **Run ID** = Specific delivery attempt (`attempt-1`, `attempt-2`, etc.)

**When you get multiple Run IDs:**

**1. Workflow retries (after failure):**
```go
// First attempt
Workflow ID: "order-12345"
Run ID:      "abc-def-123"  // Fails due to error

// Automatic retry (new run)
Workflow ID: "order-12345"    // Same workflow
Run ID:      "xyz-789-456"    // New run ID
```

**2. Continue-As-New:**
```go
// Long-running workflow that resets periodically
Workflow ID: "monthly-payroll-2025-01"
Run ID:      "run-1"  // Processes 1000 employees
Run ID:      "run-2"  // Continues with next 1000
Run ID:      "run-3"  // Continues with next 1000
```

**3. Cron workflows:**
```go
// Scheduled workflow that runs daily
Workflow ID: "daily-report"
Run ID:      "2025-01-01-run"  // Jan 1st execution
Run ID:      "2025-01-02-run"  // Jan 2nd execution
```

**Practical example:**
```go
// Start workflow
workflowRun, err := client.ExecuteWorkflow(ctx, options, OrderWorkflow, params)

// Get both IDs
workflowID := workflowRun.GetID()      // "order-12345" (what you set)
runID := workflowRun.GetRunID()        // "abc-123-def" (Temporal generated)

// Query specific run
client.QueryWorkflow(ctx, workflowID, runID, "getStatus")

// Signal the workflow (goes to latest run)
client.SignalWorkflow(ctx, workflowID, "", "approve", approvalData)
```

**Key differences:**

| Aspect | Workflow ID | Run ID |
|--------|-------------|--------|
| **Who sets it** | You (developer) | Temporal (automatic) |
| **Uniqueness** | Unique per workflow | Unique per execution attempt |
| **When needed** | Always required | Optional (defaults to latest) |
| **Can reuse** | No (unless workflow completed) | No (always unique) |
| **Purpose** | Business identifier | Technical execution tracking |

**Why this matters:**
- When **signaling/querying**, you usually only need Workflow ID (targets current run)
- For **debugging**, Run ID helps you find the exact execution attempt
- **Idempotency** is based on Workflow ID, not Run ID

📖 **Review:** [Lesson 4](lesson_4.md#workflow-execution)

</details>

---

## 🛡️ Lesson 5: Error Handling & Retries

### Question 5: Retry Policy

**An activity fails with a network timeout. What determines if it will retry?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** The **RetryPolicy** configured for the activity determines retry behavior.

**How retry policies work:**

```go
activityOptions := workflow.ActivityOptions{
    StartToCloseTimeout: 30 * time.Second,
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:    1 * time.Second,    // First retry after 1s
        BackoffCoefficient: 2.0,                 // Double each time
        MaximumInterval:    60 * time.Second,   // Cap at 60s
        MaximumAttempts:    5,                   // Try max 5 times
    },
}
```

**Retry timeline for network timeout:**

```
Attempt 1: Fails (network timeout)
  ↓ wait 1s (InitialInterval)
Attempt 2: Fails
  ↓ wait 2s (1s × BackoffCoefficient)
Attempt 3: Fails
  ↓ wait 4s (2s × BackoffCoefficient)
Attempt 4: Fails
  ↓ wait 8s
Attempt 5: Fails
  ↓ MaximumAttempts reached
Activity fails permanently → Workflow receives error
```

**What gets retried automatically:**
- ✅ Network errors (timeouts, connection refused)
- ✅ Temporary failures (rate limits, service unavailable)
- ✅ Transient errors (database deadlock)

**What does NOT retry (non-retryable errors):**
```go
// Mark errors as non-retryable
return temporal.NewNonRetryableApplicationError(
    "invalid credit card format",
    "InvalidInput",
    nil,
)
```

**Default retry policy (if you don't specify):**
```go
// Temporal provides sensible defaults:
InitialInterval:    1 second
BackoffCoefficient: 2.0
MaximumInterval:    100 × InitialInterval
MaximumAttempts:    unlimited (retries forever!)
```

**⚠️ Warning: Default = infinite retries!**
```go
// ❌ Without RetryPolicy - will retry forever!
activityOptions := workflow.ActivityOptions{
    StartToCloseTimeout: 30 * time.Second,
    // No RetryPolicy = infinite retries
}

// ✅ Always set MaximumAttempts
RetryPolicy: &temporal.RetryPolicy{
    MaximumAttempts: 3,  // Fail after 3 tries
}
```

**Choosing retry strategy:**

**Payment processing:**
```go
// Few retries, fast failure
MaximumAttempts:    3,
InitialInterval:    2 * time.Second,
BackoffCoefficient: 1.5,  // Gentle backoff
```

**External API calls:**
```go
// More retries, exponential backoff
MaximumAttempts:    10,
InitialInterval:    1 * time.Second,
BackoffCoefficient: 2.0,
MaximumInterval:    5 * time.Minute,
```

**Idempotent operations:**
```go
// Aggressive retries OK (safe to repeat)
MaximumAttempts:    0,  // Unlimited
```

📖 **Review:** [Lesson 5](lesson_5.md#activity-retry-policies)

</details>

---

### Question 6: Timeouts

**What's the difference between `StartToCloseTimeout` and `ScheduleToCloseTimeout`?**

<details>
<summary>Click to reveal answer</summary>

**Answer:**
- **StartToCloseTimeout**: Max duration for a **single attempt** (from start to completion)
- **ScheduleToCloseTimeout**: Max duration **including all retry attempts** (from scheduled to final completion)

**Visual timeline:**

```
Activity Scheduled
    ↓
    |←──── ScheduleToCloseTimeout (60s total) ────→|
    |                                                |
    ↓                                                |
[Attempt 1]                                         |
    |←── StartToCloseTimeout (10s) ──→|            |
    ↓                                   ↓            |
  Start ──→ Timeout! (10s passed)                   |
                                                     |
    ↓ (wait for retry)                              |
[Attempt 2]                                         |
    |←── StartToCloseTimeout (10s) ──→|            |
    ↓                                   ↓            |
  Start ──→ Timeout! (10s passed)                   |
                                                     |
    ↓ (wait for retry)                              |
[Attempt 3]                                         |
    |←── StartToCloseTimeout (10s) ──→|            |
    ↓                                   ↓            |
  Start ──→ Success!                                |
                                                     ↓
                                          Total: 50s (< 60s) ✅
```

**Configuration example:**
```go
activityOptions := workflow.ActivityOptions{
    // Single attempt must complete in 10s
    StartToCloseTimeout: 10 * time.Second,

    // All attempts combined must finish in 2 minutes
    ScheduleToCloseTimeout: 2 * time.Minute,

    RetryPolicy: &temporal.RetryPolicy{
        MaximumAttempts: 5,
        InitialInterval: 5 * time.Second,
    },
}
```

**Scenario 1: ScheduleToCloseTimeout exceeded**
```go
// StartToCloseTimeout: 10s
// ScheduleToCloseTimeout: 30s
// RetryPolicy: 5 attempts with 10s intervals

Attempt 1: 10s (timeout)
Wait: 10s
Attempt 2: 10s (timeout)
Wait: 10s
// Total: 40s > 30s ScheduleToCloseTimeout
// Activity fails permanently even though attempts remain!
```

**Scenario 2: Both timeouts configured correctly**
```go
StartToCloseTimeout:    30 * time.Second,    // Each attempt: 30s max
ScheduleToCloseTimeout: 5 * time.Minute,     // All attempts: 5min max

// This allows:
// - Up to 10 retries with 30s backoff intervals
// - Each attempt has full 30s to complete
// - Total budget of 5 minutes
```

**All activity timeout types:**

| Timeout | Scope | Use Case |
|---------|-------|----------|
| **StartToCloseTimeout** | Single attempt | Prevent stuck activities |
| **ScheduleToCloseTimeout** | All attempts | Overall deadline |
| **ScheduleToStartTimeout** | Queue wait time | Detect worker capacity issues |
| **HeartbeatTimeout** | Between heartbeats | Long-running activity health |

**Best practices:**

```go
// Short-lived activity (API call)
StartToCloseTimeout:    10 * time.Second,
ScheduleToCloseTimeout: 1 * time.Minute,  // Allow for 5-6 retries

// Long-running activity (video processing)
StartToCloseTimeout:    10 * time.Minute,
ScheduleToCloseTimeout: 1 * time.Hour,    // Allow for 5-6 attempts
HeartbeatTimeout:       30 * time.Second, // Check progress

// Critical deadline (payment must complete before order expires)
ScheduleToCloseTimeout: 5 * time.Minute,  // Hard deadline
StartToCloseTimeout:    30 * time.Second,
```

**Common mistake:**
```go
// ❌ ScheduleToCloseTimeout too short for retries
StartToCloseTimeout:    30 * time.Second,
ScheduleToCloseTimeout: 30 * time.Second,  // No room for retries!
RetryPolicy: &temporal.RetryPolicy{
    MaximumAttempts: 5,  // Won't get used!
}

// ✅ Give room for retries
ScheduleToCloseTimeout: 5 * time.Minute,  // Allows for retries
```

📖 **Review:** [Lesson 5](lesson_5.md#timeouts)

</details>

---

### Question 7: Compensating Transactions

**When should you use compensating transactions in a workflow?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use compensating transactions when you need to **undo/rollback** changes made by previous activities after a later activity fails.

**Classic scenario - Order processing:**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // 1. Reserve inventory
    err := workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, nil)
    if err != nil {
        return err  // Nothing to compensate yet
    }

    // 2. Charge payment
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    if err != nil {
        // ⚠️ Inventory reserved but payment failed!
        // Must compensate: release the inventory
        workflow.ExecuteActivity(ctx, ReleaseInventory, order).Get(ctx, nil)
        return err
    }

    // 3. Arrange shipping
    err = workflow.ExecuteActivity(ctx, ArrangeShipping, order).Get(ctx, nil)
    if err != nil {
        // ⚠️ Need to undo BOTH previous steps
        workflow.ExecuteActivity(ctx, RefundPayment, order).Get(ctx, nil)
        workflow.ExecuteActivity(ctx, ReleaseInventory, order).Get(ctx, nil)
        return err
    }

    return nil
}
```

**Better pattern - Saga pattern with defer:**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var compensations []func()

    // Setup compensation handler
    defer func() {
        if len(compensations) > 0 {
            // Run compensations in reverse order
            for i := len(compensations) - 1; i >= 0; i-- {
                compensations[i]()
            }
        }
    }()

    // 1. Reserve inventory
    err := workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, nil)
    if err != nil {
        return err
    }
    compensations = append(compensations, func() {
        workflow.ExecuteActivity(ctx, ReleaseInventory, order).Get(ctx, nil)
    })

    // 2. Charge payment
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    if err != nil {
        return err  // defer will run compensations
    }
    compensations = append(compensations, func() {
        workflow.ExecuteActivity(ctx, RefundPayment, order).Get(ctx, nil)
    })

    // 3. Ship
    err = workflow.ExecuteActivity(ctx, ArrangeShipping, order).Get(ctx, nil)
    if err != nil {
        return err  // defer will compensate payment + inventory
    }

    // Success! Clear compensations
    compensations = nil
    return nil
}
```

**When compensation is needed:**

**✅ Multi-step processes with side effects:**
- Financial transactions (charge → refund)
- Resource reservations (reserve → release)
- External API calls (create → delete)
- Distributed transactions

**✅ Long-running workflows:**
- Hotel + flight + car rental bookings
- Multi-stage approval processes
- Supply chain orchestration

**❌ When NOT needed:**

**Idempotent operations:**
```go
// No compensation needed - safe to retry
func SendEmail(ctx context.Context, email Email) error {
    // Sending same email twice is fine
}
```

**Read-only operations:**
```go
// No state change = no compensation
func ValidateOrder(ctx context.Context, order Order) error {
    return validator.Validate(order)
}
```

**Atomic operations (handled by system):**
```go
// Database handles rollback automatically
func SaveOrderToDB(ctx context.Context, order Order) error {
    tx := db.Begin()
    defer tx.Rollback()  // DB handles this

    tx.Insert(order)
    tx.Insert(orderItems)

    return tx.Commit()
}
```

**Key principles:**

1. **Compensations should be idempotent**
```go
// ✅ Safe to call multiple times
func ReleaseInventory(ctx context.Context, orderID string) error {
    // Check if already released
    if !inventory.IsReserved(orderID) {
        return nil  // Already done
    }
    return inventory.Release(orderID)
}
```

2. **Reverse order of operations**
```
Forward:  Reserve → Charge → Ship
Backward: Cancel Ship → Refund → Release
```

3. **Handle compensation failures**
```go
// Log but don't fail workflow if compensation fails
err := workflow.ExecuteActivity(ctx, RefundPayment, order).Get(ctx, nil)
if err != nil {
    workflow.GetLogger(ctx).Error("Compensation failed", "error", err)
    // Alert operations team
    workflow.ExecuteActivity(ctx, AlertOps, err)
}
```

📖 **Review:** [Lesson 5](lesson_5.md#compensating-transactions)

</details>

---

### Question 8: Error Types

**What's the difference between a retryable and non-retryable error?**

<details>
<summary>Click to reveal answer</summary>

**Answer:**
- **Retryable error**: Temporary failure that might succeed if retried (network timeout, rate limit)
- **Non-retryable error**: Permanent failure that will never succeed even with retries (invalid input, not found)

**Retryable errors (transient):**

```go
func ChargePaymentActivity(ctx context.Context, payment Payment) error {
    resp, err := paymentGateway.Charge(payment)

    // These will automatically retry:
    if err == ErrNetworkTimeout {
        return err  // ✅ Might work next time
    }
    if err == ErrServiceUnavailable {
        return err  // ✅ Service might recover
    }
    if err == ErrRateLimited {
        return err  // ✅ Rate limit might reset
    }

    // Activity will retry based on RetryPolicy
    return err
}
```

**Non-retryable errors (permanent):**

```go
func ChargePaymentActivity(ctx context.Context, payment Payment) error {
    resp, err := paymentGateway.Charge(payment)

    // These should NOT retry:
    if err == ErrInvalidCardNumber {
        // ❌ Will never work, even with retries
        return temporal.NewNonRetryableApplicationError(
            "invalid card number",
            "InvalidInput",
            err,
        )
    }
    if err == ErrInsufficientFunds {
        // ❌ Retrying won't add money to account
        return temporal.NewNonRetryableApplicationError(
            "insufficient funds",
            "PaymentDeclined",
            err,
        )
    }
    if err == ErrCardExpired {
        return temporal.NewNonRetryableApplicationError(
            "card expired",
            "InvalidCard",
            err,
        )
    }

    return nil  // Success
}
```

**Decision tree:**

```
Error occurred
    ↓
Is it a temporary issue?
    ↓                           ↓
   YES                         NO
    ↓                           ↓
Return normal error      Return NonRetryableApplicationError
    ↓                           ↓
Temporal retries          Workflow receives error immediately
```

**Examples by category:**

**Retryable (transient failures):**
- Network timeout
- Connection refused
- Service unavailable (503)
- Rate limited (429)
- Database deadlock
- Temporary file lock
- Resource temporarily unavailable

**Non-retryable (permanent failures):**
- Invalid input format
- Resource not found (404)
- Authentication failed (401)
- Permission denied (403)
- Duplicate key violation
- Business rule violation
- Resource already exists (409)

**Pattern for handling both:**

```go
func ProcessOrderActivity(ctx context.Context, orderID string) error {
    order, err := orderService.GetOrder(orderID)

    if err != nil {
        // Classify the error
        switch {
        case errors.Is(err, ErrOrderNotFound):
            // Permanent - order doesn't exist
            return temporal.NewNonRetryableApplicationError(
                fmt.Sprintf("order %s not found", orderID),
                "OrderNotFound",
                err,
            )

        case errors.Is(err, ErrDatabaseConnectionFailed):
            // Temporary - might reconnect
            return fmt.Errorf("database connection failed: %w", err)

        case errors.Is(err, ErrInvalidOrderState):
            // Permanent - business rule violation
            return temporal.NewNonRetryableApplicationError(
                "order in invalid state for processing",
                "InvalidState",
                err,
            )

        default:
            // Unknown - default to retryable (safe)
            return fmt.Errorf("unexpected error: %w", err)
        }
    }

    return nil
}
```

**Impact on workflows:**

**Retryable error:**
```go
// Workflow waits for retries
err := workflow.ExecuteActivity(ctx, ChargePayment, payment).Get(ctx, nil)
// This line doesn't execute until:
// - Activity succeeds, OR
// - Activity exhausts all retry attempts
```

**Non-retryable error:**
```go
// Workflow gets error immediately, no waiting
err := workflow.ExecuteActivity(ctx, ValidateCard, card).Get(ctx, nil)
if err != nil {
    // Handle immediately - card is invalid, no point retrying
    return fmt.Errorf("validation failed: %w", err)
}
```

**Best practice:**

```go
// Be conservative: when in doubt, make it retryable
// It's better to retry unnecessarily than to fail permanently on a transient issue

// But mark clear permanent failures as non-retryable
// to fail fast and avoid wasting resources
```

📖 **Review:** [Lesson 5](lesson_5.md#error-handling)

</details>

---

## 📡 Lesson 6: Signals & Queries

### Question 9: Signals

**What's the main difference between calling an Activity and sending a Signal to a workflow?**

<details>
<summary>Click to reveal answer</summary>

**Answer:**
- **Activity**: Workflow **calls** activity and **waits** for result (synchronous, blocking)
- **Signal**: External client **sends** data to workflow, **doesn't wait** (asynchronous, non-blocking)

**Visual comparison:**

**Activity (workflow → activity):**
```
Workflow                     Activity
   |                            |
   |----ExecuteActivity------→ |
   |                            | (processing...)
   |←-------Result------------- |
   | (continues with result)    |
```

**Signal (client → workflow):**
```
Client                       Workflow
   |                            |
   |-------Signal------------→ |
   | (returns immediately)      | (receives signal)
   |                            | (continues processing)
   |                            |
```

**Code example - Activity:**
```go
// Workflow calls activity and WAITS
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var result PaymentResult

    // ⏸️ Blocks here until activity completes
    err := workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &result)

    // Can use result immediately
    if result.Success {
        // continue...
    }
}
```

**Code example - Signal:**
```go
// Workflow receives signal asynchronously
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // Setup signal channel
    approvalChannel := workflow.GetSignalChannel(ctx, "approval")

    // Wait for approval signal
    var approved bool
    approvalChannel.Receive(ctx, &approved)  // ⏸️ Blocks until signal received

    if approved {
        // continue processing
    }
}

// Client sends signal (doesn't wait for workflow to process it)
client.SignalWorkflow(ctx, workflowID, runID, "approval", true)
// ✅ Returns immediately, doesn't wait for workflow reaction
```

**When to use each:**

**Use Activity when:**
- ✅ Workflow needs to **do work** (call API, query DB)
- ✅ Workflow needs the **result** to continue
- ✅ Operation is part of **workflow logic**
- ✅ You want **automatic retries**

**Use Signal when:**
- ✅ **External event** affects workflow (user approval, payment received)
- ✅ Workflow should **wait** for external input
- ✅ **Human-in-the-loop** patterns
- ✅ **Dynamic updates** to running workflow

**Real-world examples:**

**Activity - Workflow drives the action:**
```go
// Workflow initiates payment
err := workflow.ExecuteActivity(ctx, ChargeCustomer, amount).Get(ctx, nil)
```

**Signal - External event drives the workflow:**
```go
// User clicks "Approve" button → sends signal
// Workflow waits for this external event

approvalChan := workflow.GetSignalChannel(ctx, "user-approval")
var decision ApprovalDecision
approvalChan.Receive(ctx, &decision)
```

**Combined pattern - Order with approval:**
```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // 1. Activity: Validate order
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)

    // 2. Signal: Wait for manager approval
    approvalChan := workflow.GetSignalChannel(ctx, "approval")
    var approved bool

    // Wait up to 24 hours for approval
    selector := workflow.NewSelector(ctx)
    selector.AddReceive(approvalChan, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &approved)
    })
    selector.AddFuture(workflow.NewTimer(ctx, 24*time.Hour), func(f workflow.Future) {
        approved = false  // Timeout = auto-reject
    })
    selector.Select(ctx)

    if !approved {
        return errors.New("order not approved")
    }

    // 3. Activity: Process payment
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)

    return nil
}
```

**Key differences summary:**

| Aspect | Activity | Signal |
|--------|----------|--------|
| **Direction** | Workflow → Activity | Client → Workflow |
| **Initiated by** | Workflow code | External client |
| **Blocking** | Yes (workflow waits) | No (client doesn't wait) |
| **Return value** | Yes | No |
| **Use case** | Do work | Receive events |
| **Retries** | Automatic | N/A (delivery guaranteed) |

📖 **Review:** [Lesson 6](lesson_6.md#signals-vs-activities)

</details>

---

### Question 10: Queries

**Can a Query modify workflow state?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** **No!** Queries are **read-only** and **must not** modify workflow state.

**Why queries must be read-only:**

1. **Queries don't appear in history** - If they modified state, replays would be inconsistent
2. **Queries can run during replay** - Modifying state would corrupt the workflow
3. **Queries should be fast** - No side effects means no waiting for database/API calls
4. **Queries can be called anytime** - Even on completed workflows (from history)

**✅ Correct Query implementation:**
```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var currentStatus string = "pending"
    var paymentID string

    // Setup query handler - READ ONLY
    err := workflow.SetQueryHandler(ctx, "getStatus", func() (string, error) {
        return currentStatus, nil  // ✅ Just returns current state
    })

    err = workflow.SetQueryHandler(ctx, "getPaymentInfo", func() (PaymentInfo, error) {
        // ✅ Read-only: return data without modification
        return PaymentInfo{
            Status:    currentStatus,
            PaymentID: paymentID,
        }, nil
    })

    // Workflow logic that modifies state
    currentStatus = "processing"
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &paymentID)
    currentStatus = "completed"

    return nil
}
```

**❌ Wrong Query implementation:**
```go
// DON'T DO THIS!
err := workflow.SetQueryHandler(ctx, "cancelOrder", func() error {
    canceled = true  // ❌ WRONG: Modifying workflow state
    return nil
})

err = workflow.SetQueryHandler(ctx, "incrementCounter", func() int {
    counter++  // ❌ WRONG: Side effect
    return counter
})

err = workflow.SetQueryHandler(ctx, "logStatus", func() string {
    workflow.GetLogger(ctx).Info("Status checked")  // ❌ WRONG: Side effect
    return status
})
```

**What to use instead for mutations:**

**Use Signal to modify state:**
```go
// ✅ Correct: Use signal for state changes
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var canceled bool = false

    // Query: Read-only
    workflow.SetQueryHandler(ctx, "isCanceled", func() (bool, error) {
        return canceled, nil  // ✅ Just read
    })

    // Signal: Can modify state
    cancelChannel := workflow.GetSignalChannel(ctx, "cancel")

    workflow.Go(ctx, func(ctx workflow.Context) {
        cancelChannel.Receive(ctx, nil)
        canceled = true  // ✅ Signal can modify
    })

    // Rest of workflow...
}
```

**Query vs Signal comparison:**

| Aspect | Query | Signal |
|--------|-------|--------|
| **Purpose** | Read state | Modify state |
| **State changes** | ❌ Not allowed | ✅ Allowed |
| **In history** | ❌ No | ✅ Yes |
| **Return value** | ✅ Yes | ❌ No |
| **When called** | Anytime (even after completion) | Only on running workflows |
| **Example** | Get order status | Cancel order |

**Real-world example:**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    state := OrderState{
        Status:     "created",
        Items:      order.Items,
        Total:      order.Total,
        CreatedAt:  workflow.Now(ctx),
    }

    // ✅ Queries: Read-only access to state
    workflow.SetQueryHandler(ctx, "getState", func() (OrderState, error) {
        return state, nil
    })

    workflow.SetQueryHandler(ctx, "getTotal", func() (float64, error) {
        return state.Total, nil
    })

    workflow.SetQueryHandler(ctx, "isComplete", func() (bool, error) {
        return state.Status == "completed", nil
    })

    // ✅ Signals: Can modify state
    updateChannel := workflow.GetSignalChannel(ctx, "updateItems")

    workflow.Go(ctx, func(ctx workflow.Context) {
        var newItems []Item
        updateChannel.Receive(ctx, &newItems)
        state.Items = newItems  // ✅ Signal can modify
        state.Total = calculateTotal(newItems)
    })

    // Workflow logic that updates state
    state.Status = "processing"
    err := workflow.ExecuteActivity(ctx, ProcessOrder, order).Get(ctx, nil)
    state.Status = "completed"
    state.CompletedAt = workflow.Now(ctx)

    return nil
}
```

**Client usage:**

```go
// Query - Get current status
var status string
err := client.QueryWorkflow(ctx, workflowID, "", "getStatus", &status)
fmt.Println("Status:", status)  // ✅ Read-only

// Signal - Modify workflow state
err = client.SignalWorkflow(ctx, workflowID, "", "cancel", nil)
// ✅ Triggers state change in workflow
```

📖 **Review:** [Lesson 6](lesson_6.md#queries)

</details>

---

### Question 11: Signal with Start

**What's the advantage of using SignalWithStart instead of separate Start + Signal calls?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** **SignalWithStart** is **atomic** - it guarantees the workflow starts AND receives the signal, preventing race conditions.

**The problem with separate calls:**

```go
// ❌ Race condition possible
workflowID := "order-12345"

// 1. Start workflow
_, err := client.ExecuteWorkflow(ctx, options, OrderWorkflow, order)

// 2. Send signal
err = client.SignalWorkflow(ctx, workflowID, "", "priority-upgrade", nil)

// ⚠️ PROBLEM: What if workflow finishes between step 1 and 2?
// Signal would fail with "workflow not found" error
```

**Race condition timeline:**
```
Thread 1 (client):          Workflow:
ExecuteWorkflow() ────→     Start
                            Process (very fast!)
                            Complete ✅
SignalWorkflow() ─────→     ❌ Error: workflow already completed
```

**Solution with SignalWithStart:**

```go
// ✅ Atomic operation
signalWithStartRequest := &client.SignalWithStartWorkflowOptions{
    ID:        "order-12345",
    TaskQueue: "orders",
    Signal:    "priority-upgrade",
    SignalArgs: []interface{}{priorityLevel},
}

_, err := client.SignalWithStartWorkflow(
    ctx,
    signalWithStartRequest,
    OrderWorkflow,
    orderParams,
)

// ✅ Guarantees:
// - If workflow doesn't exist: start it AND deliver signal
// - If workflow exists: just deliver signal
// - No race condition possible
```

**How it works:**

**Case 1: Workflow doesn't exist**
```go
SignalWithStart
    ↓
Check if workflow exists (ID: "order-12345")
    ↓
Not found
    ↓
Start workflow + Queue signal
    ↓
Workflow receives signal before processing anything else
```

**Case 2: Workflow already running**
```go
SignalWithStart
    ↓
Check if workflow exists (ID: "order-12345")
    ↓
Found (already running)
    ↓
Just send signal (don't start duplicate)
    ↓
Running workflow receives signal
```

**Real-world use cases:**

**1. Idempotent workflow creation with initial state:**
```go
// Client might call this multiple times (retries, concurrent requests)
// Want to ensure workflow starts exactly once but always gets the signal

func createOrderWithPriority(orderID string, priority int) error {
    signalOpts := &client.SignalWithStartWorkflowOptions{
        ID:         orderID,
        TaskQueue:  "orders",
        Signal:     "set-priority",
        SignalArgs: []interface{}{priority},
    }

    _, err := client.SignalWithStartWorkflow(
        ctx,
        signalOpts,
        OrderWorkflow,
        orderParams,
    )

    // ✅ Safe to call multiple times:
    // - First call: starts workflow + sets priority
    // - Subsequent calls: just updates priority (workflow already running)
    return err
}
```

**2. Event-driven workflow startup:**
```go
// Webhook receives payment notification
// Want to start order workflow OR update existing one

func handlePaymentWebhook(orderID string, payment Payment) error {
    signalOpts := &client.SignalWithStartWorkflowOptions{
        ID:         fmt.Sprintf("order-%s", orderID),
        TaskQueue:  "orders",
        Signal:     "payment-received",
        SignalArgs: []interface{}{payment},
    }

    _, err := client.SignalWithStartWorkflow(ctx, signalOpts, OrderWorkflow, order)

    // ✅ Handles both cases:
    // - Order workflow not started yet: start it with payment
    // - Order workflow running: add payment to existing workflow
    return err
}
```

**3. Lazy workflow initialization:**
```go
// Create workflow on first interaction, not proactively

func updateUserPreference(userID string, pref Preference) error {
    signalOpts := &client.SignalWithStartWorkflowOptions{
        ID:         fmt.Sprintf("user-session-%s", userID),
        TaskQueue:  "user-sessions",
        Signal:     "update-preference",
        SignalArgs: []interface{}{pref},
    }

    // Creates long-running session workflow on first preference update
    // Subsequent updates just signal the existing workflow
    _, err := client.SignalWithStartWorkflow(ctx, signalOpts, UserSessionWorkflow, userID)
    return err
}
```

**Workflow implementation:**
```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var priority int = 0

    // Handle signal sent with start
    priorityChannel := workflow.GetSignalChannel(ctx, "priority-upgrade")

    // Check if signal was sent with start
    selector := workflow.NewSelector(ctx)
    selector.AddReceive(priorityChannel, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &priority)
    })
    selector.AddDefault(func() {
        // No signal sent with start, use default
        priority = 0
    })
    selector.Select(ctx)

    // Continue processing with priority
    if priority > 5 {
        // Fast-track processing
    }
}
```

**Benefits:**

| Aspect | Separate Start + Signal | SignalWithStart |
|--------|-------------------------|-----------------|
| **Atomicity** | ❌ Not atomic | ✅ Atomic |
| **Race conditions** | ⚠️ Possible | ✅ Prevented |
| **Idempotency** | ❌ Need manual handling | ✅ Built-in |
| **Code complexity** | More error handling | Simpler |
| **Use case** | Sequential operations | Concurrent-safe startup |

📖 **Review:** [Lesson 6](lesson_6.md#signal-with-start)

</details>

---

### Question 12: Human-in-the-Loop

**How would you implement a workflow that waits up to 48 hours for human approval before auto-rejecting?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use a **Selector** to wait for either the approval signal OR a 48-hour timer, whichever comes first.

**Implementation:**

```go
func ApprovalWorkflow(ctx workflow.Context, request ApprovalRequest) error {
    // Track state
    var approved bool
    var timedOut bool

    // Setup signal channel for approval
    approvalChannel := workflow.GetSignalChannel(ctx, "approval-decision")

    // Create 48-hour timeout
    timer := workflow.NewTimer(ctx, 48*time.Hour)

    // Wait for either signal or timeout
    selector := workflow.NewSelector(ctx)

    // Option 1: Approval signal received
    selector.AddReceive(approvalChannel, func(c workflow.ReceiveChannel, more bool) {
        var decision ApprovalDecision
        c.Receive(ctx, &decision)
        approved = decision.Approved
        timedOut = false

        workflow.GetLogger(ctx).Info("Approval received", "approved", approved)
    })

    // Option 2: 48 hours passed
    selector.AddFuture(timer, func(f workflow.Future) {
        timedOut = true
        approved = false

        workflow.GetLogger(ctx).Info("Approval timeout - auto-rejecting")
    })

    // Block until one of the above happens
    selector.Select(ctx)

    // Handle result
    if timedOut {
        // Send notification about auto-rejection
        workflow.ExecuteActivity(ctx, NotifyAutoRejection, request).Get(ctx, nil)
        return errors.New("approval timeout - request auto-rejected")
    }

    if !approved {
        // Explicitly rejected
        workflow.ExecuteActivity(ctx, NotifyRejection, request).Get(ctx, nil)
        return errors.New("request rejected by approver")
    }

    // Approved! Continue processing
    workflow.ExecuteActivity(ctx, NotifyApproval, request).Get(ctx, nil)
    err := workflow.ExecuteActivity(ctx, ProcessApprovedRequest, request).Get(ctx, nil)

    return err
}
```

**Client code to send approval:**

```go
// Approver clicks "Approve" button
func handleApproval(workflowID string, approved bool) error {
    decision := ApprovalDecision{
        Approved:  approved,
        ApprovedBy: "manager@company.com",
        ApprovedAt: time.Now(),
        Comments:   "Looks good",
    }

    err := client.SignalWorkflow(
        ctx,
        workflowID,
        "",  // Use latest run
        "approval-decision",
        decision,
    )

    return err
}
```

**Timeline visualization:**

**Scenario 1: Approved within 48 hours**
```
T+0h:    Workflow starts, sets up selector
T+2h:    Manager clicks "Approve"
         ↓
         Signal received → approved = true
         ↓
         Continue processing ✅
```

**Scenario 2: Timeout (no response)**
```
T+0h:    Workflow starts, sets up selector
T+48h:   Timer fires
         ↓
         timedOut = true, approved = false
         ↓
         Send rejection notification
         ↓
         Workflow fails with timeout error ❌
```

**Enhanced version with reminders:**

```go
func ApprovalWorkflowWithReminders(ctx workflow.Context, request ApprovalRequest) error {
    var approved bool
    approvalChannel := workflow.GetSignalChannel(ctx, "approval-decision")

    // Send reminder every 24 hours
    reminderTicker := workflow.NewTicker(ctx, 24*time.Hour)
    defer reminderTicker.Stop()

    // Final deadline: 48 hours
    deadline := workflow.NewTimer(ctx, 48*time.Hour)

    for {
        selector := workflow.NewSelector(ctx)

        // Approval received
        selector.AddReceive(approvalChannel, func(c workflow.ReceiveChannel, more bool) {
            var decision ApprovalDecision
            c.Receive(ctx, &decision)
            approved = decision.Approved
        })

        // Reminder tick (every 24h)
        selector.AddReceive(reminderTicker.Chan(), func(c workflow.ReceiveChannel, more bool) {
            c.Receive(ctx, nil)
            // Send reminder
            workflow.ExecuteActivity(ctx, SendReminderEmail, request).Get(ctx, nil)
        })

        // Final deadline (48h)
        selector.AddFuture(deadline, func(f workflow.Future) {
            approved = false
        })

        selector.Select(ctx)

        // Check if we got approval or deadline
        if approvalChannel.ReceiveAsync(&ApprovalDecision{}) || deadline.IsReady() {
            break
        }
    }

    if !approved {
        return errors.New("approval timeout")
    }

    // Continue processing
    return workflow.ExecuteActivity(ctx, ProcessRequest, request).Get(ctx, nil)
}
```

**Timeline with reminders:**
```
T+0h:    Start → Send initial notification
T+24h:   No response → Send reminder #1
T+48h:   No response → Auto-reject
```

**Key patterns:**

1. **Selector for multiple wait conditions**
2. **Timer for deadlines**
3. **Signal for external events**
4. **Ticker for periodic actions (reminders)**

**Real-world applications:**
- Purchase order approvals
- Expense report reviews
- Deployment approvals
- Contract reviews
- Manual verification steps

📖 **Review:** [Lesson 6](lesson_6.md#human-in-the-loop-patterns)

</details>

---

## 🎯 Scoring Guide

Count how many you got right on the first try:

- **10-12 correct:** 🌟 Excellent! You're ready for real-world Temporal development
- **7-9 correct:** 👍 Strong understanding, review the topics you missed
- **4-6 correct:** 📚 Good progress, revisit lessons for concepts you struggled with
- **0-3 correct:** 🔄 Take another pass through Part 2 lessons

---

## 📚 What's Next?

Once you feel confident with Part 2:

**Ready for real-world application?** Continue to **[Lesson 7: Order Processing Workflow](lesson_7.md)**

Or review specific lessons:
- [Lesson 4: Running Your First Workflow](lesson_4.md)
- [Lesson 5: Error Handling & Retries](lesson_5.md)
- [Lesson 6: Signals & Queries](lesson_6.md)
- [Back to Part 1 Quiz](quiz_part1.md)

---

**Questions or need clarification?** Review the lesson materials and try running the code examples locally. Hands-on practice is the best way to solidify these concepts!

---

_Part 2 Quiz • Temporal Fast Course • Last Updated: November 2025_