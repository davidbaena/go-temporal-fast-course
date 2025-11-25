# Lesson 10: Compensation & Saga Patterns Deep Dive

## Learning Objectives
By the end of this lesson you will:
- ✅ Master the Saga pattern for distributed transactions
- ✅ Understand forward recovery vs backward recovery (compensation)
- ✅ Design complex multi-step compensatable workflows
- ✅ Implement compensation with Temporal's built-in features
- ✅ Handle partial failures in long-running business processes
- ✅ Build idempotent compensation activities
- ✅ Apply compensation best practices in production scenarios

[← Back to Course Index](course.md) | [← Previous: Lesson 9](lesson_9.md)

---
## Why Before How: The Distributed Transaction Problem

In traditional monolithic systems with a single database, you have ACID transactions:
```sql
BEGIN TRANSACTION;
  INSERT INTO orders ...;
  UPDATE inventory ...;
  INSERT INTO payments ...;
COMMIT;  -- All or nothing
```

**But in distributed systems with microservices:**
- Order service has its own DB
- Inventory service has its own DB
- Payment service has its own DB
- No shared transaction coordinator

**The Problem:**
1. Reserve inventory → ✅ Success
2. Charge payment → ✅ Success
3. Create shipping label → ❌ Failure

Now you have **partial state**: inventory reserved, payment charged, but no shipment. You can't rollback across services!

**The Solution: Saga Pattern**
Instead of distributed transactions, use a sequence of local transactions with compensating actions:
- Each step is a local transaction
- If a step fails, execute compensation activities for all completed steps
- Workflows are **eventually consistent**, not immediately consistent

---
## Saga Pattern Overview

### Two Types of Sagas

#### 1. **Choreography-based Saga**
Services publish events, others listen and react. Decentralized.
- **Pros**: Loose coupling
- **Cons**: Hard to track state, complex debugging

#### 2. **Orchestration-based Saga** (Temporal's approach)
Central orchestrator (workflow) coordinates all steps.
- **Pros**: Clear flow, easy to debug, visible state
- **Cons**: Orchestrator is a single point of failure (but Temporal handles this)

**Temporal implements orchestration-based Sagas naturally.**

### Saga Terminology
| Term | Definition |
|------|------------|
| **Transaction** (T) | A compensatable activity (e.g., `ChargePayment`) |
| **Compensation** (C) | The undo operation (e.g., `RefundPayment`) |
| **Pivot Transaction** | The point of no return—after this, you cannot abort |
| **Forward Recovery** | Retry until success (no compensation) |
| **Backward Recovery** | Compensate completed steps (rollback) |

---
## Compensation Strategies

### 1. **Backward Recovery (Classic Saga)**
Undo completed steps in reverse order.

Example: E-commerce order
```
Reserve Inventory → Charge Payment → Ship Order
     ↓                    ↓                ↓
Release Inventory ← Refund Payment ← Cancel Shipment
```

### 2. **Forward Recovery**
Keep retrying failed steps until they succeed (no undo).

Use when:
- Operations are critical and must eventually succeed
- Compensation is not possible
- Example: Health records update (can't "undo" a medical record)

### 3. **Hybrid Approach**
Some steps compensate, others retry indefinitely.

Example:
- Payment processing: Retry (transient failures)
- Inventory reservation: Compensate (business decision to cancel)

---
## Designing Compensatable Workflows

### Key Principles

#### 1. **Idempotency is Critical**
Both activities AND compensations must be idempotent.

**Activity idempotency:**
```go
func ChargePayment(ctx context.Context, orderID string, amount float64) (string, error) {
    // Check if already charged
    existing, err := db.GetPaymentByOrderID(orderID)
    if err == nil && existing.Status == "completed" {
        return existing.TransactionID, nil  // Already charged
    }

    // Process payment
    txID, err := paymentGateway.Charge(amount)
    if err != nil { return "", err }

    // Store idempotency record
    db.SavePayment(orderID, txID, "completed")
    return txID, nil
}
```

**Compensation idempotency:**
```go
func RefundPayment(ctx context.Context, orderID string) error {
    // Check if already refunded
    payment, err := db.GetPaymentByOrderID(orderID)
    if err != nil { return err }
    if payment.Status == "refunded" {
        return nil  // Already refunded, no-op
    }

    // Process refund
    err = paymentGateway.Refund(payment.TransactionID)
    if err != nil { return err }

    // Update status
    db.UpdatePaymentStatus(orderID, "refunded")
    return nil
}
```

#### 2. **Compensation Order**
Execute compensations in **reverse order** of successful transactions.

```
Success order:     T1 → T2 → T3 → [T4 fails]
Compensation order:      C3 ← C2 ← C1
```

#### 3. **Parallel vs Sequential**
**Sequential operations**: Easy to compensate
```go
T1 → T2 → T3  (if T3 fails, compensate T2 then T1)
```

**Parallel operations**: Track which completed
```go
     ┌─ T1 ─┐
     ├─ T2 ─┤
     └─ T3 ─┘
```
Compensate all that succeeded, ignore those that failed.

---
## Temporal Implementation Patterns

### Pattern 1: Manual Compensation Tracking

```go
func OrderSagaWorkflow(ctx workflow.Context, order Order) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &workflow.RetryPolicy{
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    logger := workflow.GetLogger(ctx)

    // Track compensation state
    var compensations []func() error

    // Step 1: Validate order
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)
    if err != nil {
        logger.Error("Validation failed", "error", err)
        return err
    }

    // Step 2: Reserve inventory
    var reservationID string
    err = workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, &reservationID)
    if err != nil {
        logger.Error("Inventory reservation failed", "error", err)
        return err
    }
    // Add compensation
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, ReleaseInventory, reservationID).Get(ctx, nil)
    })

    // Step 3: Charge payment
    var paymentID string
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &paymentID)
    if err != nil {
        logger.Error("Payment failed, compensating", "error", err)
        compensate(ctx, compensations)
        return err
    }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, RefundPayment, paymentID).Get(ctx, nil)
    })

    // Step 4: Create shipment
    var shipmentID string
    err = workflow.ExecuteActivity(ctx, CreateShipment, order).Get(ctx, &shipmentID)
    if err != nil {
        logger.Error("Shipment failed, compensating", "error", err)
        compensate(ctx, compensations)
        return err
    }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, CancelShipment, shipmentID).Get(ctx, nil)
    })

    // Step 5: Send confirmation (point of no return)
    err = workflow.ExecuteActivity(ctx, SendConfirmation, order).Get(ctx, nil)
    if err != nil {
        // This is non-critical, log but don't fail
        logger.Warn("Confirmation email failed", "error", err)
    }

    logger.Info("Order completed successfully", "orderID", order.ID)
    return nil
}

// Helper: Execute compensations in reverse order
func compensate(ctx workflow.Context, compensations []func() error) {
    logger := workflow.GetLogger(ctx)

    // Execute in reverse
    for i := len(compensations) - 1; i >= 0; i-- {
        err := compensations[i]()
        if err != nil {
            // Log but continue compensating
            logger.Error("Compensation failed", "step", i, "error", err)
        }
    }
}
```

### Pattern 2: Structured Saga with Defer

```go
type SagaStep struct {
    Name         string
    Execute      func() error
    Compensate   func() error
}

func OrderSagaWithDefer(ctx workflow.Context, order Order) error {
    ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
    ctx = workflow.WithActivityOptions(ctx, ao)

    var compensations []func() error

    // Ensure compensation on failure
    var sagaErr error
    defer func() {
        if sagaErr != nil {
            // Execute compensations in reverse
            for i := len(compensations) - 1; i >= 0; i-- {
                _ = compensations[i]()
            }
        }
    }()

    // Step 1: Reserve inventory
    var reservationID string
    sagaErr = workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, &reservationID)
    if sagaErr != nil { return sagaErr }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, ReleaseInventory, reservationID).Get(ctx, nil)
    })

    // Step 2: Charge payment
    var paymentID string
    sagaErr = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &paymentID)
    if sagaErr != nil { return sagaErr }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, RefundPayment, paymentID).Get(ctx, nil)
    })

    // Step 3: Create shipment
    sagaErr = workflow.ExecuteActivity(ctx, CreateShipment, order).Get(ctx, nil)
    if sagaErr != nil { return sagaErr }

    return nil
}
```

### Pattern 3: Parallel Compensations

```go
func ParallelSagaWorkflow(ctx workflow.Context, order Order) error {
    ao := workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second}
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Execute parallel activities
    futureInventory := workflow.ExecuteActivity(ctx, ReserveInventory, order)
    futureWarehouse := workflow.ExecuteActivity(ctx, ReserveWarehouseSpace, order)
    futureDriver := workflow.ExecuteActivity(ctx, AssignDriver, order)

    // Wait for all
    var invID, whID, driverID string
    errInv := futureInventory.Get(ctx, &invID)
    errWH := futureWarehouse.Get(ctx, &whID)
    errDriver := futureDriver.Get(ctx, &driverID)

    // Check if any failed
    if errInv != nil || errWH != nil || errDriver != nil {
        // Compensate all that succeeded
        var compensations []workflow.Future

        if errInv == nil {
            compensations = append(compensations,
                workflow.ExecuteActivity(ctx, ReleaseInventory, invID))
        }
        if errWH == nil {
            compensations = append(compensations,
                workflow.ExecuteActivity(ctx, ReleaseWarehouse, whID))
        }
        if errDriver == nil {
            compensations = append(compensations,
                workflow.ExecuteActivity(ctx, UnassignDriver, driverID))
        }

        // Wait for all compensations (don't fail if they error)
        for _, f := range compensations {
            _ = f.Get(ctx, nil)
        }

        return fmt.Errorf("parallel operations failed: inv=%v wh=%v driver=%v",
            errInv, errWH, errDriver)
    }

    // All succeeded, continue...
    return nil
}
```

---
## Real-World Example: Travel Booking Saga

Book flight, hotel, and rental car—all must succeed or all cancel.

```go
type BookingRequest struct {
    CustomerID string
    FlightID   string
    HotelID    string
    CarID      string
}

type BookingResult struct {
    FlightConfirmation string
    HotelConfirmation  string
    CarConfirmation    string
}

func TravelBookingSaga(ctx workflow.Context, req BookingRequest) (*BookingResult, error) {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 2 * time.Minute,
        RetryPolicy: &workflow.RetryPolicy{
            MaximumAttempts: 3,
            NonRetryableErrorTypes: []string{"InsufficientInventory", "InvalidRequest"},
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    result := &BookingResult{}
    var compensations []func() error
    var err error

    // Book flight
    err = workflow.ExecuteActivity(ctx, BookFlight, req.FlightID).Get(ctx, &result.FlightConfirmation)
    if err != nil {
        return nil, fmt.Errorf("flight booking failed: %w", err)
    }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, CancelFlight, result.FlightConfirmation).Get(ctx, nil)
    })

    // Book hotel
    err = workflow.ExecuteActivity(ctx, BookHotel, req.HotelID).Get(ctx, &result.HotelConfirmation)
    if err != nil {
        compensate(ctx, compensations)
        return nil, fmt.Errorf("hotel booking failed: %w", err)
    }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, CancelHotel, result.HotelConfirmation).Get(ctx, nil)
    })

    // Book rental car
    err = workflow.ExecuteActivity(ctx, BookCar, req.CarID).Get(ctx, &result.CarConfirmation)
    if err != nil {
        compensate(ctx, compensations)
        return nil, fmt.Errorf("car booking failed: %w", err)
    }
    compensations = append(compensations, func() error {
        return workflow.ExecuteActivity(ctx, CancelCar, result.CarConfirmation).Get(ctx, nil)
    })

    // All succeeded
    return result, nil
}

func compensate(ctx workflow.Context, compensations []func() error) {
    logger := workflow.GetLogger(ctx)
    for i := len(compensations) - 1; i >= 0; i-- {
        if err := compensations[i](); err != nil {
            logger.Error("Compensation failed", "index", i, "error", err)
            // Continue compensating even if one fails
        }
    }
}
```

**Activities:**
```go
func BookFlight(ctx context.Context, flightID string) (string, error) {
    // Idempotency: check if already booked for this workflow execution
    executionID := activity.GetInfo(ctx).WorkflowExecution.ID

    existing, _ := db.GetBooking(executionID, "flight")
    if existing != nil {
        return existing.ConfirmationCode, nil
    }

    // Book flight
    confirmation, err := flightAPI.Reserve(flightID)
    if err != nil {
        return "", err
    }

    // Store
    db.SaveBooking(executionID, "flight", confirmation)
    return confirmation, nil
}

func CancelFlight(ctx context.Context, confirmation string) error {
    // Idempotency: check if already cancelled
    booking, _ := db.GetBookingByConfirmation(confirmation)
    if booking != nil && booking.Status == "cancelled" {
        return nil
    }

    // Cancel
    err := flightAPI.Cancel(confirmation)
    if err != nil {
        return err
    }

    // Update
    db.UpdateBookingStatus(confirmation, "cancelled")
    return nil
}
```

---
## Advanced: Nested Sagas

Sometimes a compensation itself is a saga!

Example: Cancelling an order might require:
1. Process refund
2. Return inventory to warehouse
3. Update analytics

If step 2 fails, you need to compensate step 1 (reverse the refund attempt).

```go
func CompensateOrder(ctx workflow.Context, orderID string) error {
    // This compensation is itself a saga
    var subCompensations []func() error

    // Step 1: Refund payment
    err := workflow.ExecuteActivity(ctx, RefundPayment, orderID).Get(ctx, nil)
    if err != nil { return err }
    subCompensations = append(subCompensations, func() error {
        return workflow.ExecuteActivity(ctx, RechargePayment, orderID).Get(ctx, nil)
    })

    // Step 2: Return inventory
    err = workflow.ExecuteActivity(ctx, ReturnInventory, orderID).Get(ctx, nil)
    if err != nil {
        compensate(ctx, subCompensations)
        return err
    }

    // Both succeeded
    return nil
}
```

---
## Compensation Best Practices

### 1. **Limit Compensation Complexity**
- Keep compensation activities simple
- Avoid nested sagas when possible
- Use forward recovery if compensation is too complex

### 2. **Timeouts for Compensations**
Don't let compensations hang forever:
```go
compensationCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
    StartToCloseTimeout: 1 * time.Minute,  // Shorter than normal activities
})
```

### 3. **Alerting on Compensation Failures**
```go
func compensate(ctx workflow.Context, compensations []func() error) {
    for i := len(compensations) - 1; i >= 0; i-- {
        if err := compensations[i](); err != nil {
            // Send alert to ops team
            _ = workflow.ExecuteActivity(ctx, SendAlert,
                fmt.Sprintf("Compensation failed at step %d: %v", i, err)).Get(ctx, nil)
        }
    }
}
```

### 4. **Semantic Lock Pattern**
Prevent concurrent operations on the same resource:
```go
func ReserveInventory(ctx context.Context, orderID, sku string) error {
    // Acquire semantic lock
    locked, err := db.TryLock(sku, orderID, 5*time.Minute)
    if !locked {
        return fmt.Errorf("inventory locked by another order")
    }

    // Reserve
    err = db.DecrementStock(sku, 1)
    return err
}

func ReleaseInventory(ctx context.Context, orderID, sku string) error {
    // Release lock
    _ = db.ReleaseLock(sku, orderID)

    // Return stock
    return db.IncrementStock(sku, 1)
}
```

### 5. **Pivot Transaction Strategy**
Define a "point of no return" after which you use forward recovery instead of compensation:

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // Before pivot: compensate on failure
    if err := reserveAndCharge(ctx, order); err != nil {
        return err  // Compensation handled inside
    }

    // PIVOT POINT: Order confirmed to customer
    _ = workflow.ExecuteActivity(ctx, SendOrderConfirmation, order).Get(ctx, nil)

    // After pivot: forward recovery only (retry until success)
    retryUntilSuccess(ctx, CreateShipment, order)
    retryUntilSuccess(ctx, NotifyWarehouse, order)

    return nil
}

func retryUntilSuccess(ctx workflow.Context, activity interface{}, input interface{}) {
    infiniteRetry := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
        StartToCloseTimeout: 5 * time.Minute,
        RetryPolicy: &workflow.RetryPolicy{
            MaximumAttempts: 0,  // Infinite retries
            BackoffCoefficient: 2.0,
            InitialInterval: 1 * time.Second,
            MaximumInterval: 10 * time.Minute,
        },
    })

    _ = workflow.ExecuteActivity(infiniteRetry, activity, input).Get(infiniteRetry, nil)
}
```

---
## Testing Sagas

### Unit Test: Verify Compensation Logic
```go
func TestOrderSaga_CompensationOnPaymentFailure(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    // Mock activities
    env.OnActivity(ReserveInventory, mock.Anything).Return("inv-123", nil)
    env.OnActivity(ChargePayment, mock.Anything).Return("", errors.New("payment failed"))
    env.OnActivity(ReleaseInventory, "inv-123").Return(nil)

    env.ExecuteWorkflow(OrderSagaWorkflow, Order{ID: "order-1"})

    require.True(t, env.IsWorkflowCompleted())
    require.Error(t, env.GetWorkflowError())

    // Verify compensation was called
    env.AssertCalled(t, "ReleaseInventory", "inv-123")
}
```

### Integration Test: Real Compensation
```go
func TestSaga_Integration(t *testing.T) {
    // Start workflow
    we, err := c.ExecuteWorkflow(context.Background(), options, OrderSagaWorkflow, order)
    require.NoError(t, err)

    // Wait for completion
    var result string
    err = we.Get(context.Background(), &result)

    // Verify database state
    inventory := db.GetInventoryReservation(order.SKU)
    assert.Nil(t, inventory, "inventory should be released after failure")

    payment := db.GetPayment(order.ID)
    assert.Equal(t, "refunded", payment.Status)
}
```

---
## Monitoring & Observability

### Metrics to Track
```go
func OrderSagaWithMetrics(ctx workflow.Context, order Order) error {
    workflow.GetMetricsHandler(ctx).Counter("saga_started").Inc(1)

    err := executeSaga(ctx, order)

    if err != nil {
        workflow.GetMetricsHandler(ctx).Counter("saga_compensated").Inc(1)
    } else {
        workflow.GetMetricsHandler(ctx).Counter("saga_completed").Inc(1)
    }

    return err
}
```

### Structured Logging
```go
logger := workflow.GetLogger(ctx)
logger.Info("Saga step completed",
    "step", "reserve_inventory",
    "orderID", order.ID,
    "reservationID", reservationID)

logger.Error("Saga step failed, compensating",
    "step", "charge_payment",
    "orderID", order.ID,
    "error", err,
    "compensations_count", len(compensations))
```

---
## Common Pitfalls

| Pitfall | Problem | Solution |
|---------|---------|----------|
| Non-idempotent compensation | Refunding twice, releasing stock twice | Add idempotency checks |
| Missing compensation steps | Forgetting to compensate a step | Use structured saga pattern |
| Compensation order wrong | Compensating in wrong order causes deadlock | Always reverse order |
| Ignoring compensation failures | Compensation fails silently | Log, alert, retry compensation |
| Over-compensating | Compensating non-critical steps | Mark pivot point, use forward recovery after |

---
## Summary

✅ **Saga pattern** solves distributed transactions with compensation
✅ **Temporal workflows** naturally implement orchestration-based sagas
✅ **Idempotency** is critical for both activities and compensations
✅ **Reverse order** compensation prevents inconsistencies
✅ **Pivot transactions** separate compensation from forward recovery
✅ **Testing** sagas ensures correct compensation logic
✅ **Monitoring** compensation failures prevents silent errors

---
## Exercise

Build a **bank transfer saga**:

1. **Workflow**: `TransferMoneySaga`
   - Debit from Account A
   - Credit to Account B
   - Send notification

2. **Compensation**:
   - If credit fails: refund debit
   - If notification fails: log but don't compensate (non-critical)

3. **Requirements**:
   - Both debit and credit must be idempotent
   - Test compensation when credit fails
   - Add a 3-second delay between steps to simulate processing
   - Log all compensation actions

4. **Bonus**:
   - Add a fraud check activity that can permanently fail
   - Implement parallel debits from multiple accounts
   - Add metrics for compensation rate

---
## What's Next?

You've completed the core Temporal course! Optional advanced topics:
- Lesson 11: Schedules & Cron Workflows
- Lesson 12: Search Attributes & Visibility
- Lesson 13: Data Converters & Encryption
- Lesson 14: Nexus & Service Integration

Or start building production workflows with your new knowledge!

[← Back to Course Index](course.md) | [← Previous: Lesson 9](lesson_9.md)