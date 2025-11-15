# Lesson 7: Full Order Processing Workflow (Integration)

## Learning Objectives
By the end of this lesson you will:
- ✅ Design a production-style Order Processing workflow integrating everything learned
- ✅ Apply signals (approval, cancellation, dynamic line items)
- ✅ Use retries, typed errors, compensation (Saga pattern)
- ✅ Run parallel enrichment activities
- ✅ Structure activities for maintainability and testability
- ✅ Handle workflow versioning and backward compatibility
- ✅ Add observability (logging, metrics tracing hints)
- ✅ Prepare the workflow for real domain integration (payment, inventory, notification)

[← Back to Course Index](course.md) | [← Previous: Lesson 6](lesson_6.md) | [Next: Lesson 8 →](lesson_8.md)

---
## Why This Matters
You've learned individual building blocks. Now we compose them into a real, resilient **Order Processing Workflow**:
1. Validate and enrich order data (parallel)
2. Reserve inventory
3. Await approval (signals + timeout)
4. Process payment (retry + typed errors)
5. Update order status
6. Dispatch notification
7. Handle cancellation at any stage
8. Compensate side-effects on failure

This is the bridge from a demo to something you could deploy in production.

---
## Target Architecture (Concept Map)
```
OrderWorkflow
 ├─ Parallel Enrichment
 │   ├─ FetchCustomerProfileActivity
 │   ├─ FetchInventorySnapshotActivity
 │   └─ FetchRecommendationsActivity (optional)
 ├─ ReserveStockActivity
 ├─ Await Signals Loop
 │   ├─ approve-payment
 │   ├─ cancel-order
 │   └─ add-line-item
 ├─ ProcessPaymentActivity (retry)
 ├─ UpdateOrderStatusActivity
 ├─ SendOrderConfirmationActivity
 └─ Compensation Path
     ├─ ReleaseStockActivity
     ├─ RefundPaymentActivity
     └─ EmitCancellationEmailActivity
```

---
## Workflow Data Structures
```go
type LineItem struct {
    SKU      string
    Quantity int
}

type OrderEnrichment struct {
    CustomerTier   string
    InventoryOk    bool
    Recommendations []string
}

type OrderWorkflowStatus struct {
    OrderID          string
    Stage            string
    Items            []LineItem
    Reserved         bool
    PaymentApproved  bool
    Charged          bool
    Cancelled        bool
    LastError        string
    Enrichment       OrderEnrichment
    ApprovalDeadline time.Time
    Version          string // For workflow versioning
}

// Signals payloads
type PaymentApproval struct { ApprovedBy string }
type CancelRequest struct { Reason string }
```

---
## Versioning Strategy (Forward-Compatible Changes)
When evolving a workflow, add guarded logic:
```go
version := workflow.GetVersion(ctx, "enrichment-parallel-v1", workflow.DefaultVersion, 1)
if version == workflow.DefaultVersion {
    // Old logic (sequential)
} else {
    // New parallel enrichment logic
}
```
This ensures existing executions don’t break when new code is deployed.

---
## Activity Grouping for Maintainability
Group related activities behind interfaces to enable mocking in tests.
```go
type InventoryActivities interface {
    ReserveStock(ctx context.Context, orderID string, items []LineItem) error
    ReleaseStock(ctx context.Context, orderID string) error
    FetchInventorySnapshot(ctx context.Context, items []LineItem) (bool, error)
}

type PaymentActivities interface {
    ProcessPayment(ctx context.Context, orderID string) error
    RefundPayment(ctx context.Context, orderID string) error
}

type NotificationActivities interface {
    SendOrderConfirmation(ctx context.Context, orderID string, email string) error
    SendCancellationEmail(ctx context.Context, orderID string, reason string) error
}

// Register concrete implementations in worker:
// w.RegisterActivity(inventorySvc)
// w.RegisterActivity(paymentSvc)
// w.RegisterActivity(notificationSvc)
```

Register methods by name using `RegisterActivity` (methods become activities if exported).

---
## Retry & Typed Errors Recap
```go
retryPolicy := &workflow.RetryPolicy{
    InitialInterval:    1 * time.Second,
    BackoffCoefficient: 2.0,
    MaximumAttempts:    5,
    NonRetryableErrorTypes: []string{"PermanentError", "ValidationError"},
}

activityOpts := workflow.ActivityOptions{
    StartToCloseTimeout: 30 * time.Second,
    RetryPolicy:         retryPolicy,
    HeartbeatTimeout:    15 * time.Second, // For long-running operations
}
ctx = workflow.WithActivityOptions(ctx, activityOpts)
```

---
## Full Workflow Skeleton (Integrated)
```go
func OrderWorkflow(ctx workflow.Context, orderID string, initialItems []LineItem) (string, error) {
    logger := workflow.GetLogger(ctx)

    // Versioning example
    version := workflow.GetVersion(ctx, "order-workflow-v2", workflow.DefaultVersion, 2)

    status := OrderWorkflowStatus{
        OrderID: orderID,
        Stage:   "start",
        Items:   initialItems,
        Version: fmt.Sprintf("v%d", version),
    }

    // Queries
    if err := workflow.SetQueryHandler(ctx, "get-status", func() (OrderWorkflowStatus, error) { return status, nil }); err != nil { return "", err }
    if err := workflow.SetQueryHandler(ctx, "get-items", func() ([]LineItem, error) { return status.Items, nil }); err != nil { return "", err }

    // Signals
    sigApprove := workflow.GetSignalChannel(ctx, "approve-payment")
    sigCancel  := workflow.GetSignalChannel(ctx, "cancel-order")
    sigAddItem := workflow.GetSignalChannel(ctx, "add-line-item")

    // 1. Enrichment (parallel if new version)
    status.Stage = "enrichment"
    if version == workflow.DefaultVersion {
        // Sequential enrichment fallback
        var invOk bool
        if err := workflow.ExecuteActivity(ctx, "FetchInventorySnapshot", status.Items).Get(ctx, &invOk); err != nil { return "", err }
        status.Enrichment.InventoryOk = invOk
    } else {
        // Parallel enrichment
        fInventory := workflow.ExecuteActivity(ctx, "FetchInventorySnapshot", status.Items)
        fCustomer  := workflow.ExecuteActivity(ctx, "FetchCustomerProfile", orderID)
        fRecs      := workflow.ExecuteActivity(ctx, "FetchRecommendations", orderID)

        var invOk bool
        var customerTier string
        var recs []string

        if err := fInventory.Get(ctx, &invOk); err != nil { return "", err }
        if err := fCustomer.Get(ctx, &customerTier); err != nil { return "", err }
        if err := fRecs.Get(ctx, &recs); err != nil { return "", err }

        status.Enrichment.InventoryOk = invOk
        status.Enrichment.CustomerTier = customerTier
        status.Enrichment.Recommendations = recs
    }

    // 2. Reserve Stock
    status.Stage = "reserve"
    if err := workflow.ExecuteActivity(ctx, "ReserveStock", orderID, status.Items).Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("reserve failed: %v", err)
        return "", err
    }
    status.Reserved = true

    // 3. Await Approval (signals + timeout)
    status.Stage = "awaiting-approval"
    approvalTimeout := workflow.Now(ctx).Add(15 * time.Minute)
    status.ApprovalDeadline = approvalTimeout

    for !(status.PaymentApproved || status.Cancelled) {
        selector := workflow.NewSelector(ctx)
        timerFut := workflow.NewTimer(ctx, time.Until(approvalTimeout))

        selector.AddReceive(sigApprove, func(ch workflow.ReceiveChannel, more bool) {
            var payload PaymentApproval
            ch.Receive(ctx, &payload)
            status.PaymentApproved = true
            logger.Info("Approval received", "by", payload.ApprovedBy)
        })
        selector.AddReceive(sigCancel, func(ch workflow.ReceiveChannel, more bool) {
            var payload CancelRequest
            ch.Receive(ctx, &payload)
            status.Cancelled = true
            status.LastError = fmt.Sprintf("cancelled: %s", payload.Reason)
            logger.Info("Cancellation received", "reason", payload.Reason)
        })
        selector.AddReceive(sigAddItem, func(ch workflow.ReceiveChannel, more bool) {
            var item LineItem
            ch.Receive(ctx, &item)
            status.Items = append(status.Items, item)
            logger.Info("Item added", "sku", item.SKU, "qty", item.Quantity)
        })
        selector.AddFuture(timerFut, func(f workflow.Future) {
            // Timer fired
            status.Cancelled = true
            status.LastError = "approval timeout"
            logger.Warn("Approval timed out")
        })

        selector.Select(ctx)
    }

    if status.Cancelled {
        // Compensation for stock
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
        _ = workflow.ExecuteActivity(ctx, "SendCancellationEmail", orderID, status.LastError).Get(ctx, nil)
        status.Stage = "cancelled"
        return fmt.Sprintf("Order %s cancelled (%s)", orderID, status.LastError), nil
    }

    // 4. Process Payment (with classification)
    status.Stage = "payment"
    if err := workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("payment failed: %v", err)
        // Compensation
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
        return "", err
    }
    status.Charged = true

    // 5. Update Order Status
    status.Stage = "status-update"
    if err := workflow.ExecuteActivity(ctx, "UpdateOrderStatus", orderID, "COMPLETED").Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("status update failed: %v", err)
        // Compensation path
        _ = workflow.ExecuteActivity(ctx, "RefundPayment", orderID).Get(ctx, nil)
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
        return "", err
    }

    // 6. Send Confirmation
    status.Stage = "notify"
    if err := workflow.ExecuteActivity(ctx, "SendOrderConfirmation", orderID, "customer@example.com").Get(ctx, nil); err != nil {
        // Non-critical failure (log but continue)
        status.LastError = fmt.Sprintf("confirmation failed: %v", err)
        logger.Warn("Confirmation email failed", "error", err)
    }

    status.Stage = "completed"
    result := fmt.Sprintf("Order %s completed (version %s)", orderID, status.Version)
    logger.Info("Workflow completed", "orderID", orderID)
    return result, nil
}
```

---
## Observability Enhancements
1. Structured Logs (already used) → Include `orderID`, `stage`.
2. Metrics (optional) using interceptor pattern:
   - Count retries per activity
   - Track duration per stage
3. Tracing: Wrap activities with OpenTelemetry instrumentation.

Pseudo-interceptor idea (not implemented here):
```go
// worker.Options{InterceptorChain: []interceptor.Interceptor{metricsInterceptor, tracingInterceptor}}
```

---
## Resilience Checklist
| Concern | Implemented Strategy |
|---------|----------------------|
| Transient failures | Retry policy with backoff |
| Permanent failures | Typed non-retryable errors |
| Partial success | Compensation activities |
| External input | Signals (approval / cancellation / add item) |
| State visibility | Queries (status, items) |
| Long-running wait | Timer + cancellation path |
| Version evolution | `GetVersion` guard |
| Non-critical side effects | Confirmation email is best-effort |

---
## Testing Strategy (Preview of Lesson 8)
- Unit test enrichment branch (versioned logic) using an in-memory test environment
- Simulate approval signal → assert payment executed
- Simulate timeout → assert cancellation + compensation
- Inject payment failure → assert refund + release
- Query after each stage → verify status progression

---
## Exercise
1. Implement versioned enrichment in your actual `order_workflow.go` using `GetVersion`.
2. Add a `SendOrderConfirmation` activity and mark its failure non-fatal.
3. Introduce a `RefundPayment` and `ReleaseStock` compensation path for payment/status failures.
4. Use signals to add items before approval—ensure items are passed to payment step logic.
5. Add `GetStatus` query and verify via starter program after each major stage.
6. Simulate approval timeout and verify stock release and cancellation email.
7. Export workflow history JSON from UI for a failed run; classify events into categories (enrichment, reserve, approval loop, payment, compensation).

---
## Troubleshooting Table
| Symptom | Possible Cause | Fix |
|---------|----------------|-----|
| Workflow stuck in approval | No signal / timer too long | Send signal or reduce timeout |
| Compensation not triggered | Missing guard flags | Ensure booleans set before compensation |
| Version mismatch errors | Changed version constants incorrectly | Keep stable version keys; increment minor only |
| Items not updated | Signal payload mismatch | Ensure struct fields exported and registered |
| Payment retries too slow | Backoff too aggressive | Tune `MaximumInterval` / `BackoffCoefficient` |

---
## What You've Integrated
✅ Parallel enrichment activities  
✅ Signals + queries + timers  
✅ Retry policy + typed errors  
✅ Compensation logic (Saga pattern)  
✅ Versioning support  
✅ Structured logging + best-effort side-effects  
✅ Resilient orchestration flow  

---
## Ready for Lesson 8?
Lesson 8 will cover **Testing & Best Practices**:
- Unit/integration testing workflows
- Determinism validation
- Mocking activities
- Workflow versioning evolution tests

Say: **"I'm ready for Lesson 8"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 6](lesson_6.md) | [Next: Lesson 8 →](lesson_8.md)

