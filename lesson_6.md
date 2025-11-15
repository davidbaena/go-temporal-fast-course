# Lesson 6: Signals & Queries

## Learning Objectives
By the end of this lesson you will:
- ✅ Understand what Signals and Queries are in Temporal and why they exist
- ✅ Add reactive behavior to workflows (human-in-the-loop, dynamic input)
- ✅ Expose workflow state safely via queries
- ✅ Design signal handlers that are deterministic
- ✅ Handle cancellations and approvals in long-running workflows
- ✅ Implement Signals & Queries for the existing `OrderWorkflow`

[← Back to Course Index](course.md) | [← Previous: Lesson 5](lesson_5.md) | [Next: Lesson 7 →](lesson_7.md)

---
## Why Before How: Need for Interaction
Workflows can run for minutes, hours, days. Real systems need to:
- Inject new data mid-execution (e.g., add item, approve payment)
- Cancel or pause processing gracefully
- Request current state without waiting for completion

Traditional approaches (polling DB tables, ad-hoc websockets) are brittle. **Temporal gives first-class primitives:**
- **Signal** → Asynchronous, durable event sent to a running workflow to change its behavior.
- **Query** → Read-only snapshot of workflow state without causing side effects.

---
## Concepts
| Feature | Purpose | Characteristics |
|---------|---------|-----------------|
| Signal | External input to workflow | Durable, ordered per sender, processed deterministically |
| Query | Read-only state access | No history mutation, immediate response, cannot block on activities |

**Determinism Reminder:** Signal handling logic must be deterministic (no random, no external calls). Persist external work via activities if needed.

---
## Use Cases
| Use Case | Signal Example | Query Example |
|----------|----------------|---------------|
| Human Approval | `ApprovePayment` | `GetStatus` → pending vs approved |
| Dynamic Update | `AddLineItem` | `GetOrderDetail` |
| Cancellation | `CancelOrder` | `GetStatus` → cancelled |
| Escalation | `EscalateOrder` | `GetProgress` |
| Retry Trigger | `ForceRetryPayment` | `GetLastError` |

---
## Designing Signal Interfaces
Tips:
- Use clear, versioned names (e.g., `approve-payment-v1`)
- Keep payload small (IDs, flags); fetch large data in activity after signal
- Accumulate signals in workflow state (e.g., slice, map)
- Avoid heavy processing inline—schedule an activity instead

Signal contract example:
```go
// Approval signal payload
type PaymentApproval struct {
    OrderID    string
    ApprovedBy string
    Timestamp  time.Time // Use workflow.Now() inside workflow when recording
}
```

---
## Adding Signals to `OrderWorkflow`
### Scenario
Original flow: Reserve → Pay → Update status.
Enhancement: Require explicit approval before charging payment & allow cancellation.

### Signal Names
| Signal | Name | Payload |
|--------|------|---------|
| Approve Payment | `approve-payment` | `PaymentApproval` |
| Cancel Order | `cancel-order` | `CancelRequest` |
| Add Line Item | `add-line-item` | `LineItem` |

### Query Names
| Query | Name | Returns |
|-------|------|---------|
| Get Status | `get-status` | `OrderWorkflowStatus` |
| Get Items | `get-items` | `[]LineItem` |

### Workflow State Additions
```go
type LineItem struct {
    SKU      string
    Quantity int
}

type OrderWorkflowStatus struct {
    OrderID        string
    Reserved       bool
    PaymentApproved bool
    Charged        bool
    Cancelled      bool
    Items          []LineItem
    LastError      string
    Stage          string // e.g., "reservations", "awaiting-approval", "payment", "completed", "cancelled"
}
```

---
## Workflow Skeleton with Signals & Queries
```go
func OrderWorkflow(ctx workflow.Context, orderID string) (string, error) {
    // Activity options + retry policy omitted for brevity
    logger := workflow.GetLogger(ctx)

    status := OrderWorkflowStatus{OrderID: orderID, Stage: "reservations"}
    var approvalChan = workflow.GetSignalChannel(ctx, "approve-payment")
    var cancelChan   = workflow.GetSignalChannel(ctx, "cancel-order")
    var addItemChan  = workflow.GetSignalChannel(ctx, "add-line-item")

    // Register queries (must be before any blocking loops ideally)
    if err := workflow.SetQueryHandler(ctx, "get-status", func() (OrderWorkflowStatus, error) {
        return status, nil
    }); err != nil { return "", err }

    if err := workflow.SetQueryHandler(ctx, "get-items", func() ([]LineItem, error) {
        return status.Items, nil
    }); err != nil { return "", err }

    // 1. Reserve stock (activity)
    if err := workflow.ExecuteActivity(ctx, "ReserveStock", orderID).Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("reserve failed: %v", err)
        return "", err
    }
    status.Reserved = true
    status.Stage = "awaiting-approval"
    logger.Info("Stock reserved; awaiting approval", "orderID", orderID)

    // 2. Await approval OR cancellation while still accepting line item changes
    for !(status.PaymentApproved || status.Cancelled) {
        // Use selector to handle multiple signal channels
        selector := workflow.NewSelector(ctx)

        selector.AddReceive(approvalChan, func(c workflow.ReceiveChannel, more bool) {
            var approval PaymentApproval
            c.Receive(ctx, &approval)
            status.PaymentApproved = true
            status.Stage = "payment"
            logger.Info("Payment approval received", "by", approval.ApprovedBy)
        })

        selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
            var cancelReq CancelRequest
            c.Receive(ctx, &cancelReq)
            status.Cancelled = true
            status.Stage = "cancelled"
            logger.Info("Cancellation received", "reason", cancelReq.Reason)
        })

        selector.AddReceive(addItemChan, func(c workflow.ReceiveChannel, more bool) {
            var item LineItem
            c.Receive(ctx, &item)
            status.Items = append(status.Items, item)
            logger.Info("Line item added", "sku", item.SKU, "qty", item.Quantity)
        })

        // Block until one signal arrives
        selector.Select(ctx)
    }

    if status.Cancelled {
        // Compensation for reserved stock
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
        logger.Info("Order cancelled; stock released", "orderID", orderID)
        return fmt.Sprintf("Order %s cancelled", orderID), nil
    }

    // 3. Process payment (after approval)
    if err := workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("payment failed: %v", err)
        // Optional compensation
        _ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
        return "", err
    }
    status.Charged = true

    // 4. Update status
    if err := workflow.ExecuteActivity(ctx, "UpdateOrderStatus", orderID).Get(ctx, nil); err != nil {
        status.LastError = fmt.Sprintf("status update failed: %v", err)
        return "", err
    }
    status.Stage = "completed"

    return fmt.Sprintf("Order %s completed", orderID), nil
}
```

### Determinism Notes
- `selector.Select(ctx)` is deterministic—replay processes events in historical order.
- Signal arrival order is preserved.
- All modifications are pure workflow state changes.

---
## Sending Signals (Client Side)
```go
c, _ := client.Dial(client.Options{})
// Send approval
_ = c.SignalWorkflow(context.Background(), "order_workflow_12345", "", "approve-payment", PaymentApproval{ApprovedBy: "admin"})
// Add item
_ = c.SignalWorkflow(context.Background(), "order_workflow_12345", "", "add-line-item", LineItem{SKU: "BOOK-999", Quantity: 2})
// Cancel
_ = c.SignalWorkflow(context.Background(), "order_workflow_12345", "", "cancel-order", CancelRequest{Reason: "User request"})
```
Run these while the workflow is in the polling loop awaiting approval.

### Querying State
```go
handle := c.GetWorkflow(context.Background(), "order_workflow_12345", "")
var st OrderWorkflowStatus
_ = handle.Query(context.Background(), "get-status", &st)
fmt.Println("Status:", st.Stage, "Approved:", st.PaymentApproved)
```
Queries are instantaneous and never change workflow history.

---
## Designing for Multiple Signals
If many signals may arrive quickly:
- Collect them in a buffered channel pattern inside the workflow using a slice
- Periodically batch process (e.g., every N signals or after a timer)
- Avoid unbounded state growth—apply limits (e.g., max 500 line items)

Example Batching Pattern:
```go
if len(status.Items) >= 500 {
    // Optionally reject new items via separate signal outcome tracking
}
```

---
## Handling Time + Signals
Combine timers and signals:
```go
deadline := workflow.Now(ctx).Add(30 * time.Minute)
for !status.PaymentApproved {
    selector := workflow.NewSelector(ctx)
    approvalChan := workflow.GetSignalChannel(ctx, "approve-payment")
    timerFuture := workflow.NewTimer(ctx, time.Until(deadline))

    selector.AddReceive(approvalChan, func(ch workflow.ReceiveChannel, more bool) { /* ... */ })
    selector.AddFuture(timerFuture, func(f workflow.Future) {
        // Timeout triggered
        status.Stage = "approval-timeout"
    })

    selector.Select(ctx)
    if status.Stage == "approval-timeout" { break }
}
```
**Note:** Use workflow timers, not `time.Sleep`.

---
## Edge Cases & Strategies
| Edge Case | Strategy |
|-----------|----------|
| Duplicate approval signals | Ignore after first (check flag) |
| Cancellation after payment charged | Run compensation (refund + release stock) |
| Rapid burst of add-item signals | Batch process; optional throttling |
| Query handler panics | Return wrapped error; ensure handler pure |
| Missing signal name | Define constants; avoid magic strings |

Constants pattern:
```go
const (
    SignalApprovePayment = "approve-payment"
    SignalCancelOrder    = "cancel-order"
    SignalAddLineItem    = "add-line-item"
    QueryGetStatus       = "get-status"
)
```

---
## Testing Signals & Queries (Preview for Lesson 8)
Temporal test environment allows sending signals to a test workflow execution and asserting state changes:
- Start test environment
- Execute workflow up to waiting state
- Send signals
- Query state
- Advance virtual time (timer tests)

---
## Exercise
1. Add `CancelRequest` signal handling to your real `OrderWorkflow`.
2. Implement a `GetStatus` query returning reserved/approved/charged flags.
3. Start a workflow; before approval send two `add-line-item` signals—verify order items updated.
4. Cancel the order and observe compensation in history.
5. Re-run but approve payment first; ensure cancellation signal after payment triggers refund path.
6. Add a timeout (timer) for payment approval (15s) and verify auto-cancellation on expiry.

---
## Troubleshooting Table
| Symptom | Cause | Mitigation |
|---------|-------|-----------|
| Query returns stale data | Forgot to mutate state | Ensure handler returns current struct reference |
| Signal ignored | Wrong workflow ID / RunID | Use correct IDs; omit RunID for latest |
| Panic in query handler | Non-deterministic logic | Keep handler pure (no external calls) |
| Workflow stuck waiting | Signal never sent | Validate sender code executed successfully |
| Approval processed twice | No idempotency check | Guard with boolean flag |

---
## What You've Learned
✅ Purpose and mechanics of Signals & Queries  
✅ Deterministic signal handling with selector  
✅ Exposing workflow state safely via queries  
✅ Combining timers + signals for deadlines  
✅ Designing idempotent reactive workflows  
✅ Applying interactive patterns to existing order flow  

---
## Ready for Lesson 7?
Lesson 7 will build a full **Order Processing Workflow** integrating everything so far:
- Signals for approval/cancellation
- Retries and compensation
- Parallel enrichment steps
- Activity design & idempotency

Say: **"I'm ready for Lesson 7"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 5](lesson_5.md) | [Next: Lesson 7 →](lesson_7.md)

