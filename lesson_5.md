# Lesson 5: Error Handling & Retries

## Learning Objectives
By the end of this lesson you will:
- ✅ Differentiate transient vs permanent failures
- ✅ Configure robust Activity retry policies (backoff, max attempts, non-retryable errors)
- ✅ Handle workflow-level errors gracefully
- ✅ Use heartbeats for long-running Activities
- ✅ Implement compensating actions (Saga pattern) for partial failures
- ✅ Design idempotent Activities
- ✅ Apply error classification in your existing `OrderWorkflow`

[← Back to Course Index](course.md) | [← Previous: Lesson 4](lesson_4.md) | [Next: Lesson 6 →](lesson_6.md)

---
## Why Before How: Failure Is Normal
In distributed systems, failures happen:
- Network hiccups
- DB contention
- API rate limits
- Service restarts
- Partial state changes

Instead of pretending failure is exceptional, **Temporal bakes resilience into the programming model**:
- Automatic retries with exponential backoff
- Durable state so you can resume
- Clear separation between orchestration (workflow) and execution (activity)

If you design Activities to be idempotent and classify errors correctly, Temporal handles the heavy lifting.

---
## Failure Types
| Type | Description | Handling Strategy |
|------|-------------|-------------------|
| Transient | Temporary issue (timeout, network error) | Retry automatically |
| Permanent | Won't succeed by retrying (validation error, insufficient funds) | Mark non-retryable |
| Business Logic | Known domain rule violation | Convert to typed error |
| Infrastructure | DB down, API unreachable | Retry with backoff, alert if exceeded |

### Transient vs Permanent Example
- Transient: Payment gateway returns 503 (service unavailable)
- Permanent: Card declined (won't succeed even with retries)

Mark permanent errors as non-retryable to avoid waste and speed failure recovery.

---
## Activity Retry Policy Deep Dive
Inside a workflow:
```go
retryPolicy := &workflow.RetryPolicy{
    InitialInterval:    1 * time.Second,
    BackoffCoefficient: 2.0,         // 1s, 2s, 4s, 8s...
    MaximumInterval:    30 * time.Second,
    MaximumAttempts:    5,           // Total attempts (1 original + 4 retries)
    NonRetryableErrorTypes: []string{"PermanentError", "ValidationError"},
}

ao := workflow.ActivityOptions{
    StartToCloseTimeout: 20 * time.Second,
    RetryPolicy:         retryPolicy,
}
ctx = workflow.WithActivityOptions(ctx, ao)
```

**Best Practices:**
- Always set a reasonable `StartToCloseTimeout`
- Use exponential backoff (avoid hammering services)
- Limit attempts to avoid indefinite retries
- Define clear non-retryable types

---
## Typed Errors for Non-Retryable Failures
```go
// In activities package
type PermanentError struct { Msg string }
func (e *PermanentError) Error() string { return e.Msg }

// Usage in activity
if cardDeclined {
    return "", &PermanentError{Msg: "card declined"}
}
```
Add `PermanentError` to `NonRetryableErrorTypes` in retry policy.

**Why typed errors?** Temporal inspects the error type name for non-retryable classification.

---
## Improving `ProcessPayment` Activity
Current code (simplified):
```go
if rand.Float32() < 0.5 {
    return "", fmt.Errorf("payment processing failed for order %s", orderID)
}
```
Issue: Random failure lacks classification → all failures look the same.

Refactor:
```go
type PaymentTransientError struct{ Msg string }
func (e *PaymentTransientError) Error() string { return e.Msg }

func ProcessPayment(ctx context.Context, orderID string) (string, error) {
    logger := activity.GetLogger(ctx)

    // Simulated payment logic
    r := rand.Float32()
    switch {
    case r < 0.3:
        // Temporary gateway issue
        return "", &PaymentTransientError{Msg: "gateway timeout"}
    case r < 0.4:
        // Permanent card decline
        return "", &PermanentError{Msg: "card declined"}
    }

    logger.Info("Payment processed", "orderID", orderID)
    return fmt.Sprintf("Payment processed for order %s", orderID), nil
}
```
Configure workflow retry policy to NOT retry `PermanentError`.

---
## Workflow-Level Error Handling
```go
var paymentResult string
err := workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, &paymentResult)
if err != nil {
    // Extract details
    var appErr *workflow.ApplicationError
    if errors.As(err, &appErr) {
        if appErr.Type() == "PermanentError" {
            // Skip retries, trigger compensation
            return "", fmt.Errorf("payment permanently failed: %w", err)
        }
        // Otherwise transient
    }
    return "", fmt.Errorf("payment failed after retries: %w", err)
}
```
**Note:** Temporal wraps activity errors in `ApplicationError`—use `errors.As()` to inspect.

---
## Compensation (Saga Pattern)
If a later step fails, undo prior side-effects.

Order flow steps:
1. Reserve stock
2. Process payment
3. Update status

If step 3 fails after step 1 & 2 succeeded, you might:
- Refund payment
- Release stock

**Note:** This lesson introduces basic compensation concepts. For a comprehensive deep dive into the Saga pattern, complex compensation scenarios, and real-world examples, see [Lesson 10: Compensation & Saga Patterns Deep Dive](lesson_10.md).

Saga in workflow:
```go
var reserved bool
var charged bool

// Reserve stock
err := workflow.ExecuteActivity(ctx, "ReserveStock", orderID).Get(ctx, nil)
if err != nil { return "", err }
reserved = true

// Charge payment
err = workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, nil)
if err != nil {
    if reserved {
        // Compensate reserved stock
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
    }
    return "", err
}
charged = true

// Update status
err = workflow.ExecuteActivity(ctx, "UpdateOrderStatus", orderID).Get(ctx, nil)
if err != nil {
    // Compensation chain
    if charged {
        _ = workflow.ExecuteActivity(ctx, "RefundPayment", orderID).Get(ctx, nil)
    }
    if reserved {
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
    }
    return "", err
}
```
**Key:** Compensation should itself be idempotent (refund only once, stock release safe if already released).

---
## Idempotency Design Checklist
| Aspect | Guideline |
|--------|-----------|
| Database Writes | Use upsert / unique constraints |
| Payment Charges | Store charge ID, skip if exists |
| Email Sending | Store email log; skip if already sent for workflow run |
| Stock Reservation | Use reservation key; releasing twice is no-op |
| Logging | Safe (append) |

Store idempotency keys in a durable store if needed.

---
## Heartbeats (Long-Running Activities)
Use when activity may run > a few seconds and you want early detection of worker death.
```go
func LongRunningReport(ctx context.Context, reportID string) (string, error) {
    for i := 0; i < 10; i++ {
        // Do chunk
        time.Sleep(5 * time.Second)
        // Heartbeat progress
        activity.RecordHeartbeat(ctx, i)
        if ctx.Err() != nil { return "", ctx.Err() }
    }
    return "done", nil
}
```
Configure `HeartbeatTimeout` in activity options:
```go
ao := workflow.ActivityOptions{ HeartbeatTimeout: 10 * time.Second }
```
If heartbeats stop > timeout, Temporal retries the activity from start (design for resumability or use progress via details returned in heartbeat).

Retrieve heartbeat details when retrying:
```go
var lastProgress int
_ = activity.GetHeartbeatDetails(ctx, &lastProgress)
```

---
## Aggregating Errors
If multiple activities run in parallel:
```go
f1 := workflow.ExecuteActivity(ctx, "A1")
f2 := workflow.ExecuteActivity(ctx, "A2")

var e1, e2 error
_ = f1.Get(ctx, nil)
_ = f2.Get(ctx, nil)
// Collect errors for reporting
```
Consider failing fast vs aggregating all errors depending on business requirement.

---
## Putting It Together: Enhanced Order Workflow Skeleton
```go
func OrderWorkflow(ctx workflow.Context, orderID string) (string, error) {
    retryPolicy := &workflow.RetryPolicy{
        InitialInterval:    1 * time.Second,
        BackoffCoefficient: 2.0,
        MaximumAttempts:    5,
        NonRetryableErrorTypes: []string{"PermanentError", "ValidationError"},
    }
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy:         retryPolicy,
        HeartbeatTimeout:    10 * time.Second, // For long-running ops later
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    logger := workflow.GetLogger(ctx)
    logger.Info("OrderWorkflow started", "orderID", orderID)

    var reserved, charged bool

    // Reserve stock
    if err := workflow.ExecuteActivity(ctx, "ReserveStock", orderID).Get(ctx, nil); err != nil {
        return "", fmt.Errorf("reserve failed: %w", err)
    }
    reserved = true

    // Process payment
    if err := workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, nil); err != nil {
        // Compensation if needed
        if reserved { _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil) }
        return "", fmt.Errorf("payment failed: %w", err)
    }
    charged = true

    // Update order status
    if err := workflow.ExecuteActivity(ctx, "UpdateOrderStatus", orderID).Get(ctx, nil); err != nil {
        // Compensation chain
        if charged { _ = workflow.ExecuteActivity(ctx, "RefundPayment", orderID).Get(ctx, nil) }
        if reserved { _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil) }
        return "", fmt.Errorf("status update failed: %w", err)
    }

    logger.Info("OrderWorkflow completed", "orderID", orderID)
    return fmt.Sprintf("Order %s completed", orderID), nil
}
```

---
## Exercise
1. Refactor `ProcessPayment` with typed errors and update retry policy.
2. Add `ReleaseStock` and `RefundPayment` activities (make them idempotent).
3. Simulate transient vs permanent failures and observe behavior (UI history shows retries for transient only).
4. Add a long-running dummy `GenerateInvoice` activity with heartbeats every 2s, kill the worker mid-execution, restart it—confirm retry.
5. Export workflow execution history JSON for a failed run and classify each event.

---
## Troubleshooting Table
| Symptom | Possible Cause | Mitigation |
|---------|----------------|------------|
| Activity retries forever | No max attempts | Set `MaximumAttempts` |
| Compensation fails | Compensation activity not idempotent | Add idempotency keys |
| Permanent error still retries | Type name mismatch | Ensure `NonRetryableErrorTypes` matches `Error()` type name |
| Heartbeat timeout triggers too early | Timeout too short | Increase `HeartbeatTimeout` |
| Workflow stuck | Waiting on parallel futures silently | Add logging after each `.Get()` |

---
## What You've Learned
✅ Error classification (transient vs permanent)  
✅ Retry policy configuration  
✅ Typed non-retryable errors  
✅ Saga compensation pattern  
✅ Idempotent activity design  
✅ Heartbeats for long-running tasks  
✅ Enhanced robust workflow structure  

---
## Ready for Lesson 6?
Lesson 6 will cover **Signals & Queries**:
- Sending data into running workflows
- Querying workflow state without waiting
- Human-in-the-loop approval flows
- Dynamic workflow adaptation

Say: **"I'm ready for Lesson 6"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 4](lesson_4.md) | [Next: Lesson 6 →](lesson_6.md)

