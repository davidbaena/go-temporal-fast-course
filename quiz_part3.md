# Quiz: Part 3 Real-World Application

**Test your production-ready Temporal knowledge!**

This quiz covers Lessons 7-9: order processing workflow, testing & best practices, and production deployment.

**Instructions:**
- Try to answer each question before revealing the answer
- Read the explanations even if you get it right - they contain valuable insights
- If you miss a question, review the corresponding lesson

---

## 🛒 Lesson 7: Order Processing Workflow

### Question 1: Workflow Design

**When designing a production order processing workflow, which activities should run in parallel vs sequential?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Activities with **dependencies must run sequentially**, while **independent activities can run in parallel** for better performance.

**Sequential activities (dependencies):**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // MUST be sequential - each depends on previous

    // 1. Validate first
    var validationResult ValidationResult
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, &validationResult)
    if err != nil {
        return err  // Can't continue if invalid
    }

    // 2. Reserve inventory (depends on validation)
    var reservationID string
    err = workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, &reservationID)
    if err != nil {
        return err
    }

    // 3. Charge payment (depends on inventory availability)
    var paymentID string
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &paymentID)
    if err != nil {
        // Compensate: release inventory
        workflow.ExecuteActivity(ctx, ReleaseInventory, reservationID)
        return err
    }

    // 4. Create shipment (depends on successful payment)
    err = workflow.ExecuteActivity(ctx, CreateShipment, order, paymentID).Get(ctx, nil)
    if err != nil {
        // Compensate: refund + release inventory
        workflow.ExecuteActivity(ctx, RefundPayment, paymentID)
        workflow.ExecuteActivity(ctx, ReleaseInventory, reservationID)
        return err
    }

    // ... continue
}
```

**Parallel activities (independent):**

```go
// After successful payment, these can run in parallel
// (none depend on each other)

emailFuture := workflow.ExecuteActivity(ctx, SendConfirmationEmail, order)
slackFuture := workflow.ExecuteActivity(ctx, NotifySlackChannel, order)
analyticsFuture := workflow.ExecuteActivity(ctx, RecordAnalytics, order)
loyaltyFuture := workflow.ExecuteActivity(ctx, UpdateLoyaltyPoints, order)

// Wait for all notifications (don't block on failures)
_ = emailFuture.Get(ctx, nil)       // Log error but continue
_ = slackFuture.Get(ctx, nil)       // Log error but continue
_ = analyticsFuture.Get(ctx, nil)   // Log error but continue
_ = loyaltyFuture.Get(ctx, nil)     // Log error but continue
```

**Decision tree:**

```
Does Activity B need the result of Activity A?
    ↓                           ↓
   YES                         NO
    ↓                           ↓
Sequential                  Can be parallel
```

**Real-world order processing flow:**

```go
func CompleteOrderWorkflow(ctx workflow.Context, order Order) error {
    // Sequential: Critical path with dependencies
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)
    ↓
    err = workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, nil)
    ↓
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    ↓
    err = workflow.ExecuteActivity(ctx, CreateShipment, order).Get(ctx, nil)

    // Parallel: Independent post-processing
    notifications := []workflow.Future{
        workflow.ExecuteActivity(ctx, SendEmail, order),
        workflow.ExecuteActivity(ctx, SendSMS, order),
        workflow.ExecuteActivity(ctx, NotifyWarehouse, order),
        workflow.ExecuteActivity(ctx, UpdateAnalytics, order),
        workflow.ExecuteActivity(ctx, UpdateCRM, order),
    }

    // Wait for all notifications
    for _, f := range notifications {
        _ = f.Get(ctx, nil)  // Don't fail workflow if notification fails
    }

    return nil
}
```

**Performance impact:**

**Sequential only (slow):**
```
Validate (2s) → Reserve (3s) → Charge (5s) → Ship (2s) → Email (1s) → SMS (1s)
Total: 14 seconds
```

**Optimized with parallel (faster):**
```
Validate (2s) → Reserve (3s) → Charge (5s) → Ship (2s) → [Email, SMS, Analytics in parallel] (1s)
Total: 13 seconds (but more importantly, critical path is clear)
```

**Best practices:**

1. **Critical path sequential** - ensures data consistency
2. **Notifications parallel** - faster completion, non-blocking
3. **Error handling per activity** - independent failures don't cascade
4. **Compensation in reverse order** - undo actions properly

📖 **Review:** [Lesson 7](lesson_7.md#designing-workflows)

</details>

---

### Question 2: Saga Pattern

**In a saga pattern, why should compensations run in reverse order?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Compensations should run in **reverse order** to properly undo dependencies - you must undo later steps before earlier ones to maintain consistency.

**Why reverse order matters:**

**Forward operations:**
```
1. Reserve inventory    → Creates reservation_id
2. Charge payment       → Uses order_id, creates payment_id
3. Create shipment      → Uses payment_id, creates shipment_id
4. Update loyalty       → Uses payment_id
```

**Reverse compensations:**
```
4. Revert loyalty       → Uses payment_id (must exist)
3. Cancel shipment      → Uses shipment_id (must exist)
2. Refund payment       → Uses payment_id (must exist)
1. Release inventory    → Uses reservation_id (must exist)
```

**Example - Wrong order (forward compensations):**

```go
// ❌ WRONG: Forward order causes failures
func compensate() {
    // Try to release inventory first
    ReleaseInventory(reservationID)  // ✅ Works

    // Try to refund payment
    RefundPayment(paymentID)  // ✅ Works

    // Try to cancel shipment
    CancelShipment(shipmentID)  // ❌ FAILS!
    // Shipment carrier already has payment record deleted
    // Can't process cancellation without payment reference
}
```

**Example - Correct order (reverse compensations):**

```go
// ✅ CORRECT: Reverse order ensures dependencies exist
func OrderWorkflow(ctx workflow.Context, order Order) error {
    var compensations []func()

    // Helper to add compensation
    addCompensation := func(fn func()) {
        compensations = append(compensations, fn)
    }

    // Defer compensation execution in reverse order
    defer func() {
        if len(compensations) > 0 {
            workflow.GetLogger(ctx).Info("Running compensations", "count", len(compensations))

            // Run in REVERSE order (LIFO - Last In, First Out)
            for i := len(compensations) - 1; i >= 0; i-- {
                compensations[i]()
            }
        }
    }()

    // 1. Reserve inventory
    var reservationID string
    err := workflow.ExecuteActivity(ctx, ReserveInventory, order).Get(ctx, &reservationID)
    if err != nil {
        return err
    }
    addCompensation(func() {
        workflow.ExecuteActivity(ctx, ReleaseInventory, reservationID).Get(ctx, nil)
    })

    // 2. Charge payment
    var paymentID string
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, &paymentID)
    if err != nil {
        return err  // Defer will release inventory
    }
    addCompensation(func() {
        workflow.ExecuteActivity(ctx, RefundPayment, paymentID).Get(ctx, nil)
    })

    // 3. Create shipment
    var shipmentID string
    err = workflow.ExecuteActivity(ctx, CreateShipment, order, paymentID).Get(ctx, &shipmentID)
    if err != nil {
        return err  // Defer will: refund payment, then release inventory
    }
    addCompensation(func() {
        workflow.ExecuteActivity(ctx, CancelShipment, shipmentID).Get(ctx, nil)
    })

    // 4. Update loyalty points
    err = workflow.ExecuteActivity(ctx, AddLoyaltyPoints, order, paymentID).Get(ctx, nil)
    if err != nil {
        return err  // Defer will: cancel shipment, refund, release
    }
    addCompensation(func() {
        workflow.ExecuteActivity(ctx, RemoveLoyaltyPoints, order).Get(ctx, nil)
    })

    // Success! Clear compensations
    compensations = nil
    return nil
}
```

**Execution flow on failure:**

**Success case:**
```
Reserve → Charge → Ship → Loyalty → ✅ Success
(compensations cleared, none executed)
```

**Failure at shipment:**
```
Reserve ✅ → Charge ✅ → Ship ❌ (fails)
    ↓
Compensations run in REVERSE:
    2. Refund payment   ✅ (uses paymentID)
    1. Release inventory ✅ (uses reservationID)
```

**Failure at loyalty:**
```
Reserve ✅ → Charge ✅ → Ship ✅ → Loyalty ❌ (fails)
    ↓
Compensations run in REVERSE:
    3. Cancel shipment   ✅ (uses shipmentID, paymentID still exists)
    2. Refund payment    ✅ (uses paymentID)
    1. Release inventory ✅ (uses reservationID)
```

**Real-world analogy:**

Think of it like getting dressed:
```
Put on: Underwear → Shirt → Pants → Shoes → Jacket
Take off: Jacket → Shoes → Pants → Shirt → Underwear

❌ You can't take off your shirt before your jacket!
✅ Reverse order respects dependencies
```

**Stack data structure:**

```go
// Compensations act like a stack (LIFO)
Stack: []
    ↓
Push(ReleaseInventory)    → [ReleaseInventory]
Push(RefundPayment)       → [ReleaseInventory, RefundPayment]
Push(CancelShipment)      → [ReleaseInventory, RefundPayment, CancelShipment]
    ↓
Pop() → CancelShipment    ← Execute first
Pop() → RefundPayment     ← Execute second
Pop() → ReleaseInventory  ← Execute last
```

**Benefits of reverse order:**

1. ✅ **Preserves referential integrity** - IDs/resources still exist
2. ✅ **Maintains audit trail** - compensations can log against original records
3. ✅ **Reduces partial failures** - each compensation has what it needs
4. ✅ **Easier debugging** - clear undo path mirrors forward path

📖 **Review:** [Lesson 7](lesson_7.md#saga-pattern)

</details>

---

### Question 3: Activity Idempotency

**Why must activities be idempotent, and how do you make them idempotent?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Activities must be **idempotent** (safe to retry) because Temporal may execute them **multiple times** due to retries, worker crashes, or workflow replays.

**Why activities execute multiple times:**

**1. Automatic retries:**
```go
// Activity times out → Temporal retries automatically
attempt 1: timeout after 10s
attempt 2: succeeds (but database already updated from attempt 1?)
```

**2. Worker crashes:**
```go
// Worker crashes mid-execution, Temporal retries on another worker
worker-1: starts activity, crashes
worker-2: retries same activity (did worker-1's database write succeed?)
```

**3. At-least-once delivery:**
```go
// Network partition: activity completes but result lost
activity: completes successfully, writes to database
network: response lost in transit
temporal: no response received → schedules retry
activity: executes again (database already updated!)
```

**❌ Non-idempotent activity (dangerous):**

```go
func AddCreditsActivity(ctx context.Context, userID string, amount int) error {
    // ❌ PROBLEM: If retried, user gets double credits!
    user := db.GetUser(userID)
    user.Credits += amount
    db.SaveUser(user)

    return nil
}

// Execution:
// Attempt 1: Adds 100 credits (balance: 0 → 100) ✅
// Network failure, Temporal retries...
// Attempt 2: Adds 100 credits AGAIN (balance: 100 → 200) ❌❌
// User got 200 credits but should only get 100!
```

**✅ Idempotent activity (safe):**

**Pattern 1: Check-then-act**
```go
func AddCreditsActivity(ctx context.Context, userID string, amount int, txID string) error {
    // ✅ Check if already processed
    if db.TransactionExists(txID) {
        return nil  // Already processed, skip
    }

    // Record the transaction first (atomically)
    tx := Transaction{
        ID:      txID,
        UserID:  userID,
        Amount:  amount,
        Type:    "credit_add",
    }

    // Use database transaction for atomicity
    err := db.Transaction(func(tx *sql.Tx) error {
        // Insert transaction record (unique constraint on txID prevents duplicates)
        if err := tx.Insert(tx); err != nil {
            return err  // Already exists = idempotent
        }

        // Update credits
        _, err := tx.Exec("UPDATE users SET credits = credits + ? WHERE id = ?", amount, userID)
        return err
    })

    return err
}

// Execution:
// Attempt 1: Creates transaction, adds credits ✅
// Retry: Transaction exists, returns early ✅
// User gets exactly 100 credits (idempotent!)
```

**Pattern 2: Use unique constraints**
```go
func CreateOrderActivity(ctx context.Context, orderID string, data OrderData) error {
    order := Order{
        ID:         orderID,  // Unique!
        CustomerID: data.CustomerID,
        Items:      data.Items,
        Total:      data.Total,
    }

    // ✅ Database enforces uniqueness
    err := db.Create(&order)
    if err != nil {
        if db.IsUniqueConstraintError(err) {
            // Already exists, treat as success (idempotent)
            return nil
        }
        return err
    }

    return nil
}
```

**Pattern 3: Set (not increment)**
```go
// ❌ Non-idempotent: INCREMENT
func UpdateViewCountActivity(ctx context.Context, articleID string) error {
    db.Exec("UPDATE articles SET views = views + 1 WHERE id = ?", articleID)
    // Retry = double count!
}

// ✅ Idempotent: SET to specific value
func UpdateViewCountActivity(ctx context.Context, articleID string, newCount int) error {
    db.Exec("UPDATE articles SET views = ? WHERE id = ?", newCount, articleID)
    // Retry = same value, safe!
}
```

**Pattern 4: External API with idempotency keys**
```go
func ChargePaymentActivity(ctx context.Context, orderID string, amount float64) error {
    // ✅ Use idempotency key
    idempotencyKey := fmt.Sprintf("order-%s-payment", orderID)

    // Payment gateway deduplicates by key
    result, err := stripeClient.ChargeWithIdempotencyKey(
        amount,
        idempotencyKey,  // Same key = same result, no double charge
    )

    return err
}
```

**Pattern 5: Read-modify-write with optimistic locking**
```go
func ReserveInventoryActivity(ctx context.Context, productID string, quantity int) error {
    maxRetries := 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        // Read current version
        product := db.GetProduct(productID)

        if product.Available < quantity {
            return errors.New("insufficient inventory")
        }

        // ✅ Update with version check (optimistic locking)
        updated := db.Exec(`
            UPDATE products
            SET available = available - ?,
                version = version + 1
            WHERE id = ? AND version = ?
        `, quantity, productID, product.Version)

        if updated.RowsAffected > 0 {
            return nil  // Success
        }

        // Version mismatch (concurrent update), retry
        time.Sleep(time.Millisecond * 100)
    }

    return errors.New("failed to reserve inventory after retries")
}
```

**Testing idempotency:**

```go
func TestAddCreditsIdempotent(t *testing.T) {
    userID := "user-123"
    txID := "tx-unique-456"

    // Execute activity twice with same parameters
    err1 := AddCreditsActivity(ctx, userID, 100, txID)
    err2 := AddCreditsActivity(ctx, userID, 100, txID)

    assert.NoError(t, err1)
    assert.NoError(t, err2)

    // Verify credits added only once
    user := db.GetUser(userID)
    assert.Equal(t, 100, user.Credits)  // Not 200!
}
```

**Idempotency checklist:**

- ✅ Use unique transaction IDs
- ✅ Check if already processed
- ✅ Use database constraints (unique, foreign key)
- ✅ Use SET instead of INCREMENT
- ✅ Leverage external API idempotency keys
- ✅ Use optimistic locking for read-modify-write
- ✅ Make compensations idempotent too!

**What happens without idempotency:**

- 💰 Double charges to customer credit cards
- 📦 Over-reservation of inventory
- 📧 Duplicate email sends (annoying but not critical)
- 🔢 Incorrect counts and analytics
- 🐛 Data corruption and inconsistencies

📖 **Review:** [Lesson 7](lesson_7.md#idempotency)

</details>

---

### Question 4: HTTP Integration

**How should you expose Temporal workflows to HTTP clients (REST API)?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Create a **thin HTTP layer** that translates REST requests into Temporal workflow executions using the Temporal client.

**Architecture pattern:**

```
HTTP Client → REST API → Temporal Client → Temporal Server → Worker
                 ↓                              ↓               ↓
            (translates)                    (schedules)    (executes)
```

**Implementation:**

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/gorilla/mux"
    "go.temporal.io/sdk/client"
)

type OrderAPI struct {
    temporalClient client.Client
}

// 1. Create order (starts workflow)
func (api *OrderAPI) CreateOrder(w http.ResponseWriter, r *http.Request) {
    var req CreateOrderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Generate workflow ID from business identifier
    workflowID := fmt.Sprintf("order-%s", req.OrderID)

    workflowOptions := client.StartWorkflowOptions{
        ID:        workflowID,
        TaskQueue: "order-processing",
        // Return result immediately without waiting for workflow completion
    }

    // ✅ Start workflow asynchronously
    workflowRun, err := api.temporalClient.ExecuteWorkflow(
        r.Context(),
        workflowOptions,
        OrderWorkflow,
        req.ToOrder(),
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // ✅ Return immediately (don't wait for workflow)
    response := CreateOrderResponse{
        OrderID:    req.OrderID,
        WorkflowID: workflowRun.GetID(),
        RunID:      workflowRun.GetRunID(),
        Status:     "processing",
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)  // 202 Accepted
    json.NewEncoder(w).Encode(response)
}

// 2. Get order status (query workflow)
func (api *OrderAPI) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    orderID := vars["orderID"]
    workflowID := fmt.Sprintf("order-%s", orderID)

    // ✅ Query workflow for current status
    var status OrderStatus
    value, err := api.temporalClient.QueryWorkflow(
        r.Context(),
        workflowID,
        "",  // Use latest run
        "getStatus",
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    if err := value.Get(&status); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

// 3. Cancel order (signal workflow)
func (api *OrderAPI) CancelOrder(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    orderID := vars["orderID"]
    workflowID := fmt.Sprintf("order-%s", orderID)

    // ✅ Send cancellation signal
    err := api.temporalClient.SignalWorkflow(
        r.Context(),
        workflowID,
        "",
        "cancel",
        nil,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    w.WriteHeader(http.StatusNoContent)  // 204 No Content
}

// 4. Approve order (signal workflow)
func (api *OrderAPI) ApproveOrder(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    orderID := vars["orderID"]
    workflowID := fmt.Sprintf("order-%s", orderID)

    var approval ApprovalDecision
    if err := json.NewDecoder(r.Body).Decode(&approval); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // ✅ Send approval signal
    err := api.temporalClient.SignalWorkflow(
        r.Context(),
        workflowID,
        "",
        "approval",
        approval,
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func main() {
    // Create Temporal client
    temporalClient, err := client.Dial(client.Options{
        HostPort: "localhost:7233",
    })
    if err != nil {
        panic(err)
    }
    defer temporalClient.Close()

    api := &OrderAPI{temporalClient: temporalClient}

    // Setup routes
    router := mux.NewRouter()
    router.HandleFunc("/orders", api.CreateOrder).Methods("POST")
    router.HandleFunc("/orders/{orderID}", api.GetOrderStatus).Methods("GET")
    router.HandleFunc("/orders/{orderID}/cancel", api.CancelOrder).Methods("POST")
    router.HandleFunc("/orders/{orderID}/approve", api.ApproveOrder).Methods("POST")

    http.ListenAndServe(":8080", router)
}
```

**REST API usage:**

**Create order:**
```bash
POST /orders
{
  "orderID": "ORD-12345",
  "customerID": "CUST-789",
  "items": [...],
  "total": 99.99
}

Response: 202 Accepted
{
  "orderID": "ORD-12345",
  "workflowID": "order-ORD-12345",
  "runID": "abc-123-def",
  "status": "processing"
}
```

**Get status:**
```bash
GET /orders/ORD-12345

Response: 200 OK
{
  "orderID": "ORD-12345",
  "status": "awaiting_approval",
  "stage": "payment_processed",
  "updatedAt": "2025-01-15T10:30:00Z"
}
```

**Cancel order:**
```bash
POST /orders/ORD-12345/cancel

Response: 204 No Content
```

**Key patterns:**

**1. Async execution (don't block HTTP request):**
```go
// ❌ Don't do this - blocks HTTP request for entire workflow!
workflowRun.Get(ctx, &result)  // Waits for workflow completion

// ✅ Return immediately
return workflowID  // Client can poll for status
```

**2. Idempotent workflow IDs:**
```go
// ✅ Use business identifier
workflowID := fmt.Sprintf("order-%s", orderID)

// If client retries POST, same workflow ID prevents duplicate workflows
```

**3. Proper HTTP status codes:**
```go
CreateOrder:    202 Accepted (async operation started)
GetStatus:      200 OK (query succeeded)
CancelOrder:    204 No Content (signal sent)
NotFound:       404 Not Found (workflow doesn't exist)
InternalError:  500 Internal Server Error
```

**4. Error handling:**
```go
err := client.SignalWorkflow(ctx, workflowID, "", "cancel", nil)
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        http.Error(w, "Order not found", http.StatusNotFound)
        return
    }
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

**Don't do this:**

```go
// ❌ Running workflow logic in HTTP handler
func CreateOrder(w http.ResponseWriter, r *http.Request) {
    // DON'T put workflow logic here!
    order := parseRequest(r)
    validateOrder(order)
    reserveInventory(order)
    chargePayment(order)
    // This defeats the purpose of Temporal!
}

// ✅ Delegate to Temporal
func CreateOrder(w http.ResponseWriter, r *http.Request) {
    order := parseRequest(r)
    client.ExecuteWorkflow(ctx, options, OrderWorkflow, order)
    // Let Temporal handle the orchestration!
}
```

📖 **Review:** [Lesson 7](lesson_7.md#http-integration)

</details>

---

## 🧪 Lesson 8: Testing & Best Practices

### Question 5: Testing Workflows

**What's the best way to unit test a workflow without running a Temporal server?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use Temporal's **test workflow environment** (`testsuite` package) which provides a local in-memory test server.

**Test setup:**

```go
package workflows

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "go.temporal.io/sdk/testsuite"
)

type OrderWorkflowTestSuite struct {
    suite.Suite
    testsuite.WorkflowTestSuite
}

func TestOrderWorkflowTestSuite(t *testing.T) {
    suite.Run(t, new(OrderWorkflowTestSuite))
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_Success() {
    // ✅ Create test environment (in-memory, no server needed)
    env := s.NewTestWorkflowEnvironment()

    // Mock activities
    env.OnActivity(ValidateOrder, mock.Anything, mock.Anything).Return(nil)
    env.OnActivity(ReserveInventory, mock.Anything, mock.Anything).Return("reservation-123", nil)
    env.OnActivity(ChargePayment, mock.Anything, mock.Anything).Return("payment-456", nil)
    env.OnActivity(CreateShipment, mock.Anything, mock.Anything).Return("shipment-789", nil)

    // Execute workflow
    order := Order{
        ID:         "order-123",
        CustomerID: "customer-456",
        Total:      99.99,
    }
    env.ExecuteWorkflow(OrderWorkflow, order)

    // Assert workflow completed
    assert.True(s.T(), env.IsWorkflowCompleted())
    assert.NoError(s.T(), env.GetWorkflowError())

    // Verify all activities were called
    env.AssertExpectations(s.T())
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_PaymentFails() {
    env := s.NewTestWorkflowEnvironment()

    // Mock successful activities
    env.OnActivity(ValidateOrder, mock.Anything, mock.Anything).Return(nil)
    env.OnActivity(ReserveInventory, mock.Anything, mock.Anything).Return("reservation-123", nil)

    // Mock payment failure
    env.OnActivity(ChargePayment, mock.Anything, mock.Anything).Return("", errors.New("insufficient funds"))

    // Mock compensation
    env.OnActivity(ReleaseInventory, mock.Anything, "reservation-123").Return(nil)

    // Execute workflow
    order := Order{ID: "order-123", Total: 99.99}
    env.ExecuteWorkflow(OrderWorkflow, order)

    // Assert workflow failed
    assert.True(s.T(), env.IsWorkflowCompleted())
    assert.Error(s.T(), env.GetWorkflowError())

    // Verify compensation was called
    env.AssertCalled(s.T(), "ReleaseInventory", mock.Anything, "reservation-123")
}

func (s *OrderWorkflowTestSuite) TestOrderWorkflow_SignalHandling() {
    env := s.NewTestWorkflowEnvironment()

    // Mock activities
    env.OnActivity(ValidateOrder, mock.Anything, mock.Anything).Return(nil)

    // Register signal callback
    env.RegisterDelayedCallback(func() {
        // Send cancel signal after workflow starts
        env.SignalWorkflow("cancel", nil)
    }, time.Millisecond*100)

    // Execute workflow
    env.ExecuteWorkflow(OrderWorkflow, Order{ID: "order-123"})

    assert.True(s.T(), env.IsWorkflowCompleted())

    // Verify workflow handled cancellation
    var result OrderResult
    env.GetWorkflowResult(&result)
    assert.Equal(s.T(), "canceled", result.Status)
}
```

**Testing activities (standard Go tests):**

```go
func TestChargePaymentActivity(t *testing.T) {
    // ✅ Activities are just functions, test them directly

    mockPaymentGateway := &MockPaymentGateway{}
    mockPaymentGateway.On("Charge", mock.Anything).Return("payment-123", nil)

    result, err := ChargePaymentActivity(
        context.Background(),
        Order{Total: 99.99},
    )

    assert.NoError(t, err)
    assert.Equal(t, "payment-123", result)
    mockPaymentGateway.AssertExpectations(t)
}

func TestChargePaymentActivity_Idempotency(t *testing.T) {
    // Test idempotency
    ctx := context.Background()
    order := Order{ID: "order-123", Total: 50.0}
    txID := "tx-unique-789"

    // First execution
    result1, err1 := ChargePaymentActivity(ctx, order, txID)
    assert.NoError(t, err1)

    // Second execution (retry)
    result2, err2 := ChargePaymentActivity(ctx, order, txID)
    assert.NoError(t, err2)

    // Should return same result
    assert.Equal(t, result1, result2)

    // Verify only charged once
    balance := getCustomerBalance(order.CustomerID)
    assert.Equal(t, -50.0, balance)  // Not -100!
}
```

**Integration tests (with real Temporal server):**

```go
func TestOrderWorkflow_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // ✅ Connect to real Temporal server (docker-compose)
    client, err := client.Dial(client.Options{
        HostPort: "localhost:7233",
    })
    require.NoError(t, err)
    defer client.Close()

    // Start workflow
    workflowOptions := client.StartWorkflowOptions{
        ID:        "test-order-" + uuid.New().String(),
        TaskQueue: "test-queue",
    }

    workflowRun, err := client.ExecuteWorkflow(
        context.Background(),
        workflowOptions,
        OrderWorkflow,
        Order{ID: "integration-test", Total: 10.0},
    )
    require.NoError(t, err)

    // Wait for completion (with timeout)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var result OrderResult
    err = workflowRun.Get(ctx, &result)
    assert.NoError(t, err)
    assert.Equal(t, "completed", result.Status)
}
```

**Benefits of testsuite approach:**

| Aspect | testsuite | Real Server | Manual Mocks |
|--------|-----------|-------------|--------------|
| **Speed** | ⚡ Fast (in-memory) | Slow (network) | Fast |
| **Isolation** | ✅ Complete | ⚠️ Shared state | ✅ Complete |
| **Setup** | ✅ Simple | ❌ Complex | ✅ Simple |
| **Determinism** | ✅ Deterministic | ⚠️ Timing issues | ✅ Deterministic |
| **CI/CD** | ✅ No dependencies | ❌ Needs server | ✅ No dependencies |

**Test pyramid:**

```
        /\
       /  \  Integration Tests (few, slow, real server)
      /____\
     /      \
    / Unit   \ Workflow Tests (many, fast, testsuite)
   /__________\
  /            \
 /  Activity    \ Activity Tests (most, fastest, direct calls)
/________________\
```

📖 **Review:** [Lesson 8](lesson_8.md#testing-workflows)

</details>

---

### Question 6: Workflow Versioning

**You need to add a new activity to an existing workflow that's already running in production. How do you handle this safely?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use **workflow versioning** with `workflow.GetVersion()` to safely introduce changes without breaking existing executions.

**The problem:**

```go
// Old workflow version (running in production)
func OrderWorkflow(ctx workflow.Context, order Order) error {
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    return err
}

// ❌ If you just add a new activity:
func OrderWorkflow(ctx workflow.Context, order Order) error {
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)
    err = workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)  // NEW!
    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    return err
}

// ⚠️ PROBLEM: Existing workflows will replay with NEW code
// Their history doesn't have CheckFraud event
// Replay will add it, causing non-determinism error!
```

**✅ Solution: Use workflow versioning**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    err := workflow.ExecuteActivity(ctx, ValidateOrder, order).Get(ctx, nil)
    if err != nil {
        return err
    }

    // ✅ Version check
    version := workflow.GetVersion(ctx, "add-fraud-check", workflow.DefaultVersion, 1)

    if version == 1 {
        // New behavior: Check fraud
        err = workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)
        if err != nil {
            return err
        }
    }
    // version == DefaultVersion: skip fraud check (old behavior)

    err = workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    return err
}
```

**How it works:**

**New workflow execution:**
```
GetVersion("add-fraud-check", DefaultVersion, 1)
    ↓
No version in history (new execution)
    ↓
Records version=1 in history
    ↓
Returns 1
    ↓
Executes CheckFraud activity ✅
```

**Old workflow replay (started before code change):**
```
GetVersion("add-fraud-check", DefaultVersion, 1)
    ↓
Check history: no version marker found
    ↓
Returns DefaultVersion (-1)
    ↓
Skips CheckFraud activity ✅
    ↓
Continues with old behavior (deterministic!)
```

**New workflow replay (started after code change):**
```
GetVersion("add-fraud-check", DefaultVersion, 1)
    ↓
Check history: version=1 found
    ↓
Returns 1
    ↓
Executes CheckFraud activity ✅
```

**Multiple versions over time:**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // Version 1: Added fraud check
    v1 := workflow.GetVersion(ctx, "add-fraud-check", workflow.DefaultVersion, 1)
    if v1 == 1 {
        workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)
    }

    workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)

    // Version 2: Added loyalty points
    v2 := workflow.GetVersion(ctx, "add-loyalty", workflow.DefaultVersion, 1)
    if v2 == 1 {
        workflow.ExecuteActivity(ctx, UpdateLoyalty, order).Get(ctx, nil)
    }

    // Version 3: Changed notification system
    v3 := workflow.GetVersion(ctx, "new-notifications", workflow.DefaultVersion, 2)
    if v3 == 1 {
        // Old notification system
        workflow.ExecuteActivity(ctx, SendEmailLegacy, order).Get(ctx, nil)
    } else if v3 == 2 {
        // New notification system
        workflow.ExecuteActivity(ctx, SendEmailV2, order).Get(ctx, nil)
        workflow.ExecuteActivity(ctx, SendSMS, order).Get(ctx, nil)
    }

    return nil
}
```

**Cleanup after old workflows complete:**

```go
// After all old workflows complete (check Temporal UI)
// You can clean up old version checks

// Before cleanup:
v1 := workflow.GetVersion(ctx, "add-fraud-check", workflow.DefaultVersion, 1)
if v1 == 1 {
    workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)
}

// After cleanup (all workflows have v1):
workflow.GetVersion(ctx, "add-fraud-check", 1, 1)  // Min version = Max version
workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)

// Eventually (after another migration):
// Remove GetVersion entirely, just call activity
workflow.ExecuteActivity(ctx, CheckFraud, order).Get(ctx, nil)
```

**Best practices:**

**1. Never remove code that old workflows depend on**
```go
// ❌ DON'T remove old branches too soon
if version == 2 {
    // Only new code
}
// Old workflows with version=1 will fail!

// ✅ Keep old branches until all workflows complete
if version == 1 {
    // Old behavior
} else if version == 2 {
    // New behavior
}
```

**2. Use descriptive change IDs**
```go
// ✅ Good
workflow.GetVersion(ctx, "add-fraud-detection", ...)

// ❌ Bad
workflow.GetVersion(ctx, "v2", ...)
workflow.GetVersion(ctx, "change-1", ...)
```

**3. Increment versions sequentially**
```go
// ✅ Good progression
DefaultVersion → 1 → 2 → 3

// ❌ Don't skip
DefaultVersion → 5 (confusing)
```

**4. Plan for migration**
```go
// Document when you can clean up
// "Can remove after 2025-02-01 when all v0 workflows complete"
v := workflow.GetVersion(ctx, "add-feature-x", workflow.DefaultVersion, 1)
```

**Alternative: Version entire workflow**
```go
// Instead of versioning changes, version the whole workflow
func OrderWorkflowV1(ctx workflow.Context, order Order) error {
    // Old implementation
}

func OrderWorkflowV2(ctx workflow.Context, order Order) error {
    // New implementation with fraud check
}

// Client routes to correct version
if useNewVersion {
    client.ExecuteWorkflow(ctx, options, OrderWorkflowV2, order)
} else {
    client.ExecuteWorkflow(ctx, options, OrderWorkflowV1, order)
}

// Cleaner but requires routing logic in client
```

📖 **Review:** [Lesson 8](lesson_8.md#workflow-versioning)

</details>

---

### Question 7: Common Pitfalls

**Which of these is a workflow anti-pattern?**

A) Storing large data payloads directly in workflow state
B) Using workflow.ExecuteActivity() for external API calls
C) Using workflow.Now() instead of time.Now()
D) Implementing retry logic in activities

<details>
<summary>Click to reveal answer</summary>

**Answer:** A - Storing large data payloads directly in workflow state

**Why it's an anti-pattern:**

**❌ Problem:**
```go
func ProcessVideoWorkflow(ctx workflow.Context, videoData []byte) error {
    // ❌ videoData might be 500MB!
    // This gets stored in workflow history
    // Every event, every replay loads this data

    var processedVideo []byte  // ❌ Another 500MB in state
    err := workflow.ExecuteActivity(ctx, ProcessVideo, videoData).Get(ctx, &processedVideo)

    // ❌ History is now 1GB+
    // Replays are slow, database bloated
}
```

**Consequences:**
- 📈 **Bloated history** - Temporal database grows huge
- 🐌 **Slow replays** - Loading gigabytes from database
- 💰 **Increased costs** - More storage, slower queries
- ⚠️ **Size limits** - Temporal has payload size limits (2MB default)
- 🔥 **Performance degradation** - Queries, signals become slow

**✅ Solution: Store references, not data**

```go
func ProcessVideoWorkflow(ctx workflow.Context, videoID string) error {
    // ✅ Just store the ID (few bytes)

    // Activity fetches data from external storage
    var processedVideoID string
    err := workflow.ExecuteActivity(ctx, ProcessVideo, videoID).Get(ctx, &processedVideoID)

    // ✅ History stays small (just IDs)
    return nil
}

func ProcessVideoActivity(ctx context.Context, videoID string) (string, error) {
    // Fetch large data in activity
    videoData := s3.Download(videoID)  // 500MB

    // Process it
    processedData := processVideo(videoData)

    // Store result externally
    processedID := s3.Upload(processedData)

    // Return just the ID
    return processedID, nil  // ✅ Few bytes
}
```

**Pattern: External storage**

```go
// Workflow: Orchestrate with IDs
func OrderWorkflow(ctx workflow.Context, orderID string) error {
    // ✅ Small: just IDs
    var invoiceID string
    err := workflow.ExecuteActivity(ctx, GenerateInvoice, orderID).Get(ctx, &invoiceID)

    var receiptID string
    err = workflow.ExecuteActivity(ctx, GenerateReceipt, orderID).Get(ctx, &receiptID)

    // Store results metadata in workflow
    result := OrderResult{
        OrderID:   orderID,
        InvoiceID: invoiceID,   // Reference, not data
        ReceiptID: receiptID,   // Reference, not data
    }

    return nil
}

// Activity: Handle large data
func GenerateInvoiceActivity(ctx context.Context, orderID string) (string, error) {
    // Fetch order details from database
    order := db.GetOrder(orderID)

    // Generate PDF (might be large)
    invoicePDF := generatePDF(order)

    // Store in S3/blob storage
    invoiceID := s3.Upload(invoicePDF)

    // Return reference
    return invoiceID, nil
}
```

**Why the others are NOT anti-patterns:**

**B) Using workflow.ExecuteActivity() for external API calls** ✅
```go
// ✅ CORRECT: External calls must be in activities
err := workflow.ExecuteActivity(ctx, CallPaymentAPI, order).Get(ctx, nil)
// This is the RIGHT way to do it!
```

**C) Using workflow.Now() instead of time.Now()** ✅
```go
// ✅ CORRECT: workflow.Now() is deterministic
timestamp := workflow.Now(ctx)  // Safe for replays

// ❌ time.Now() is non-deterministic
timestamp := time.Now()  // Different on every replay!
```

**D) Implementing retry logic in activities** ✅
```go
// ✅ CORRECT: Activities should handle retries
activityOptions := workflow.ActivityOptions{
    RetryPolicy: &temporal.RetryPolicy{
        MaximumAttempts: 3,
    },
}
// This is best practice!
```

**Other common anti-patterns:**

**1. Infinite workflows without Continue-As-New**
```go
// ❌ History grows forever
func MonitoringWorkflow(ctx workflow.Context) error {
    for {
        workflow.Sleep(ctx, time.Minute)
        workflow.ExecuteActivity(ctx, CheckStatus).Get(ctx, nil)
        // After 1 year = 500,000+ events!
    }
}

// ✅ Reset history periodically
func MonitoringWorkflow(ctx workflow.Context) error {
    for i := 0; i < 1000; i++ {
        workflow.Sleep(ctx, time.Minute)
        workflow.ExecuteActivity(ctx, CheckStatus).Get(ctx, nil)
    }
    // Continue as new (fresh history)
    return workflow.NewContinueAsNewError(ctx, MonitoringWorkflow)
}
```

**2. Non-deterministic code in workflows**
```go
// ❌ Random behavior
if rand.Intn(2) == 0 {
    // Branch A
} else {
    // Branch B
}

// ✅ Use SideEffect for randomness
var randomValue int
workflow.SideEffect(ctx, func() interface{} {
    return rand.Intn(2)
}).Get(&randomValue)
```

**3. Blocking on activity completion when you don't need to**
```go
// ❌ Sequential when parallel is fine
workflow.ExecuteActivity(ctx, SendEmail, order).Get(ctx, nil)
workflow.ExecuteActivity(ctx, SendSMS, order).Get(ctx, nil)

// ✅ Parallel for independent work
emailFuture := workflow.ExecuteActivity(ctx, SendEmail, order)
smsFuture := workflow.ExecuteActivity(ctx, SendSMS, order)
emailFuture.Get(ctx, nil)
smsFuture.Get(ctx, nil)
```

📖 **Review:** [Lesson 8](lesson_8.md#common-pitfalls)

</details>

---

### Question 8: Observability

**What metrics should you monitor for a production Temporal deployment?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Monitor **workflow success rate, task queue lag, activity latency, worker capacity, and workflow execution time**.

**Key metrics to track:**

**1. Workflow Metrics**

```yaml
# Success rate
temporal_workflow_completed_total         # Successful completions
temporal_workflow_failed_total            # Failures
temporal_workflow_canceled_total          # Cancellations
temporal_workflow_timeout_total           # Timeouts

# Latency
temporal_workflow_execution_time_seconds  # End-to-end duration
temporal_workflow_task_execution_time     # Decision task latency

# Volume
temporal_workflow_started_total           # New workflows
temporal_workflow_running                 # Currently running
```

**2. Activity Metrics**

```yaml
# Success rate
temporal_activity_completed_total         # Successful completions
temporal_activity_failed_total            # Failures
temporal_activity_timeout_total           # Timeouts

# Latency
temporal_activity_execution_time_seconds  # Activity duration
temporal_activity_schedule_to_start       # Time in queue

# Retries
temporal_activity_retry_total             # Retry attempts
```

**3. Task Queue Metrics**

```yaml
# Backlog
temporal_task_queue_depth                 # Pending tasks
temporal_task_queue_lag_seconds           # Age of oldest task

# Throughput
temporal_task_queue_tasks_dispatched      # Tasks sent to workers
temporal_task_queue_tasks_completed       # Tasks finished
```

**4. Worker Metrics**

```yaml
# Capacity
temporal_worker_task_slots_available      # Free slots
temporal_worker_task_slots_used           # Busy slots

# Throughput
temporal_worker_workflows_executed        # Workflows processed
temporal_worker_activities_executed       # Activities processed

# Health
temporal_worker_errors_total              # Worker errors
temporal_worker_panics_total              # Panics
```

**5. System Metrics**

```yaml
# Resource usage
temporal_server_cpu_usage_percent
temporal_server_memory_usage_bytes
temporal_db_connection_pool_size

# Persistence
temporal_persistence_latency_seconds      # Database latency
temporal_persistence_errors_total         # Database errors
```

**Prometheus example:**

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.temporal.io/sdk/client"
)

// Custom metrics
var (
    orderWorkflowDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "order_workflow_duration_seconds",
            Help:    "Time to complete order workflow",
            Buckets: []float64{1, 5, 10, 30, 60, 300, 600},
        },
    )

    orderWorkflowStatus = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "order_workflow_status_total",
            Help: "Order workflow completions by status",
        },
        []string{"status"},  // Labels: success, failed, canceled
    )

    paymentActivityRetries = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "payment_activity_retries_total",
            Help: "Number of payment activity retries",
        },
    )
)

func init() {
    prometheus.MustRegister(orderWorkflowDuration)
    prometheus.MustRegister(orderWorkflowStatus)
    prometheus.MustRegister(paymentActivityRetries)
}

// Instrument workflow
func OrderWorkflow(ctx workflow.Context, order Order) error {
    startTime := workflow.Now(ctx)
    defer func() {
        duration := workflow.Now(ctx).Sub(startTime).Seconds()
        // Record metric in activity (workflows can't call Prometheus directly)
        workflow.ExecuteActivity(ctx, RecordMetric, "order_duration", duration)
    }()

    err := executeOrderLogic(ctx, order)

    status := "success"
    if err != nil {
        status = "failed"
    }
    workflow.ExecuteActivity(ctx, RecordMetric, "order_status", status)

    return err
}

// Activity that records metrics
func RecordMetricActivity(ctx context.Context, metricName string, value interface{}) error {
    switch metricName {
    case "order_duration":
        orderWorkflowDuration.Observe(value.(float64))
    case "order_status":
        orderWorkflowStatus.WithLabelValues(value.(string)).Inc()
    }
    return nil
}

// Expose metrics endpoint
http.Handle("/metrics", promhttp.Handler())
http.ListenAndServe(":9090", nil)
```

**Alerting rules:**

```yaml
# Alert: High failure rate
- alert: HighWorkflowFailureRate
  expr: |
    rate(temporal_workflow_failed_total[5m])
    /
    rate(temporal_workflow_completed_total[5m])
    > 0.1
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Workflow failure rate > 10%"

# Alert: Task queue backlog
- alert: TaskQueueBacklog
  expr: temporal_task_queue_depth > 1000
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Task queue has {{ $value }} pending tasks"

# Alert: Worker capacity
- alert: WorkerCapacityLow
  expr: |
    temporal_worker_task_slots_available
    /
    (temporal_worker_task_slots_available + temporal_worker_task_slots_used)
    < 0.2
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Worker capacity < 20%"

# Alert: Activity retries spike
- alert: HighActivityRetries
  expr: rate(temporal_activity_retry_total[5m]) > 10
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Activity retries spiking"
```

**Grafana dashboard panels:**

```
1. Workflow Success Rate (gauge)
   - (completed / (completed + failed)) * 100

2. Active Workflows (graph)
   - temporal_workflow_running over time

3. Task Queue Depth (graph)
   - temporal_task_queue_depth by queue

4. Worker Utilization (heatmap)
   - task_slots_used / total_slots by worker

5. P95 Workflow Duration (graph)
   - histogram_quantile(0.95, temporal_workflow_execution_time_seconds)

6. Top Failed Workflows (table)
   - Group by workflow type, count failures
```

**Logging best practices:**

```go
func OrderWorkflow(ctx workflow.Context, order Order) error {
    logger := workflow.GetLogger(ctx)

    // Structured logging
    logger.Info("Order workflow started",
        "orderID", order.ID,
        "customerID", order.CustomerID,
        "total", order.Total,
    )

    err := workflow.ExecuteActivity(ctx, ChargePayment, order).Get(ctx, nil)
    if err != nil {
        logger.Error("Payment failed",
            "orderID", order.ID,
            "error", err,
        )
        return err
    }

    logger.Info("Order completed successfully",
        "orderID", order.ID,
        "duration", workflow.Now(ctx).Sub(startTime),
    )

    return nil
}
```

📖 **Review:** [Lesson 8](lesson_8.md#monitoring-observability)

</details>

---

## 🚀 Lesson 9: Production Deployment

### Question 9: Worker Deployment

**What's the recommended way to deploy workers in production?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Deploy workers as **stateless containers** in Kubernetes alongside your application, with **horizontal autoscaling** and **proper resource limits**.

**Deployment architecture:**

```yaml
# Kubernetes Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-worker
spec:
  replicas: 3  # Start with 3, autoscale based on load
  selector:
    matchLabels:
      app: order-worker
  template:
    metadata:
      labels:
        app: order-worker
    spec:
      containers:
      - name: worker
        image: myapp/order-worker:v1.2.3
        env:
        - name: TEMPORAL_HOST
          value: "temporal-frontend.temporal:7233"
        - name: TASK_QUEUE
          value: "order-processing"
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
```

**Horizontal Pod Autoscaler:**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: order-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: order-worker
  minReplicas: 3
  maxReplicas: 20
  metrics:
  # Scale based on CPU
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  # Scale based on custom metric (task queue depth)
  - type: External
    external:
      metric:
        name: temporal_task_queue_depth
        selector:
          matchLabels:
            queue: "order-processing"
      target:
        type: AverageValue
        averageValue: "100"
```

**Worker code with graceful shutdown:**

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    temporalClient, err := client.Dial(client.Options{
        HostPort: os.Getenv("TEMPORAL_HOST"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer temporalClient.Close()

    // Create worker
    w := worker.New(temporalClient, os.Getenv("TASK_QUEUE"), worker.Options{
        MaxConcurrentActivityExecutionSize:     50,  // Tune based on workload
        MaxConcurrentWorkflowTaskExecutionSize: 10,
        EnableSessionWorker:                    true,
    })

    // Register workflows and activities
    w.RegisterWorkflow(OrderWorkflow)
    w.RegisterActivity(ChargePaymentActivity)
    w.RegisterActivity(SendEmailActivity)

    // Health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
        // Check if worker is ready (connected to Temporal)
        w.WriteHeader(http.StatusOK)
    })
    go http.ListenAndServe(":8080", nil)

    // Start worker
    err = w.Start()
    if err != nil {
        log.Fatal(err)
    }

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down worker gracefully...")
    w.Stop()  // ✅ Finishes current tasks, doesn't accept new ones
    log.Println("Worker stopped")
}
```

**Deployment patterns:**

**1. Sidecar pattern (worker + API in same pod):**
```yaml
spec:
  containers:
  - name: api
    image: myapp/api:v1.2.3
    ports:
    - containerPort: 8080
  - name: worker
    image: myapp/worker:v1.2.3
    # Shares pod resources with API
```

**Pros:**
- ✅ Simple deployment (one artifact)
- ✅ Shared configuration

**Cons:**
- ❌ Can't scale independently
- ❌ API and worker compete for resources

**2. Separate deployments (recommended):**
```yaml
# api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-api
spec:
  replicas: 5  # Scale based on HTTP traffic

---

# worker-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-worker
spec:
  replicas: 3  # Scale based on task queue depth
```

**Pros:**
- ✅ Independent scaling
- ✅ Better resource allocation
- ✅ Isolated failures

**Cons:**
- ❌ More complex deployment

**3. Multi-queue workers:**
```go
func main() {
    // Worker handles multiple task queues
    w1 := worker.New(client, "high-priority", worker.Options{
        MaxConcurrentActivityExecutionSize: 100,
    })
    w1.RegisterWorkflow(OrderWorkflow)
    w1.Start()

    w2 := worker.New(client, "low-priority", worker.Options{
        MaxConcurrentActivityExecutionSize: 20,
    })
    w2.RegisterWorkflow(OrderWorkflow)
    w2.Start()

    // One worker process, two queues with different capacity
}
```

**Rolling updates:**

```bash
# Zero-downtime deployment
kubectl rollout status deployment/order-worker

# Old workers finish their tasks
# New workers start accepting tasks
# Gradual transition
```

**Best practices:**

1. ✅ **Stateless workers** - no local state, can restart anytime
2. ✅ **Health checks** - liveness and readiness probes
3. ✅ **Graceful shutdown** - finish current tasks before stopping
4. ✅ **Resource limits** - prevent resource starvation
5. ✅ **Horizontal scaling** - add more workers for load
6. ✅ **Separate deployments** - scale API and workers independently
7. ✅ **Version management** - use workflow versioning for compatibility

📖 **Review:** [Lesson 9](lesson_9.md#worker-deployment)

</details>

---

### Question 10: High Availability

**How do you ensure high availability for Temporal workflows in production?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Use **multiple Temporal server replicas**, **database replication**, **multiple worker instances**, and **multi-region deployment** for critical workloads.

**High availability architecture:**

```
                    Load Balancer
                         |
        +----------------+----------------+
        |                |                |
   Temporal         Temporal         Temporal
   Frontend-1       Frontend-2       Frontend-3
   (active)         (active)         (active)
        |                |                |
        +----------------+----------------+
                         |
                   History Service
                    (sharded)
                         |
        +----------------+----------------+
        |                |                |
    Database          Database         Database
    (primary)        (replica-1)      (replica-2)
```

**1. Temporal Server HA:**

```yaml
# temporal-frontend deployment (multiple replicas)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: temporal-frontend
spec:
  replicas: 3  # Multiple instances
  template:
    spec:
      containers:
      - name: frontend
        image: temporalio/server:latest
        env:
        - name: SERVICES
          value: "frontend"
        - name: NUM_HISTORY_SHARDS
          value: "512"  # Distribute load
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"

---

# temporal-history deployment (multiple shards)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: temporal-history
spec:
  replicas: 5  # Multiple history service instances
  template:
    spec:
      containers:
      - name: history
        image: temporalio/server:latest
        env:
        - name: SERVICES
          value: "history"
```

**2. Database HA (PostgreSQL example):**

```yaml
# Primary-replica setup
apiVersion: v1
kind: Service
metadata:
  name: postgres-primary
spec:
  selector:
    role: primary
  ports:
  - port: 5432

---

apiVersion: v1
kind: Service
metadata:
  name: postgres-replica
spec:
  selector:
    role: replica
  ports:
  - port: 5432

# Temporal reads from replicas, writes to primary
```

**3. Worker HA:**

```yaml
# Multiple worker replicas across availability zones
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-worker
spec:
  replicas: 5  # Spread across AZs
  template:
    spec:
      affinity:
        podAntiAffinity:  # Don't schedule on same node
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values:
                - order-worker
            topologyKey: "kubernetes.io/hostname"
      topologySpreadConstraints:  # Spread across zones
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        labelSelector:
          matchLabels:
            app: order-worker
```

**4. Multi-region deployment (for DR):**

```
Region 1 (Primary)              Region 2 (Standby)
┌─────────────────┐             ┌─────────────────┐
│ Temporal Cluster│             │ Temporal Cluster│
│   + Workers     │             │   + Workers     │
│   + Database    │──replicate─→│   + Database    │
└─────────────────┘             └─────────────────┘
     (active)                        (passive)

Failover:
Region 2 becomes active if Region 1 fails
```

**Client configuration for HA:**

```go
// Connect with automatic failover
temporalClient, err := client.Dial(client.Options{
    HostPort: "temporal-frontend:7233",  // Load balanced endpoint
    ConnectionOptions: client.ConnectionOptions{
        MaxIdleConns:        100,
        MaxConnsPerHost:     100,
        IdleConnTimeout:     90 * time.Second,
        TLSConfig:           tlsConfig,
    },
    // Automatic retry on connection failures
    RetryPolicy: &client.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,
        MaximumAttempts:    5,
    },
})
```

**Failure scenarios:**

**Worker failure:**
```
Worker-1 crashes while processing activity
    ↓
Temporal detects heartbeat timeout
    ↓
Activity rescheduled to Worker-2
    ↓
Workflow continues (zero data loss)
```

**Temporal frontend failure:**
```
Frontend-1 crashes
    ↓
Load balancer routes to Frontend-2
    ↓
Client retry succeeds
    ↓
No workflow disruption
```

**Database failure:**
```
Primary database crashes
    ↓
Automatic failover to replica
    ↓
Replica promoted to primary
    ↓
Temporal reconnects
    ↓
Workflows resume (durable history preserved)
```

**Monitoring HA:**

```yaml
# Alerts for HA issues
- alert: TemporalFrontendDown
  expr: up{job="temporal-frontend"} == 0
  for: 1m
  severity: critical

- alert: InsufficientWorkers
  expr: count(up{job="order-worker"} == 1) < 2
  for: 5m
  severity: warning

- alert: DatabaseReplicationLag
  expr: pg_replication_lag_seconds > 60
  for: 5m
  severity: warning
```

**HA checklist:**

- ✅ **Multiple Temporal frontend replicas** (min 3)
- ✅ **Database replication** (primary + replicas)
- ✅ **Multiple worker instances** (min 2 per queue)
- ✅ **Load balancing** for frontend access
- ✅ **Health checks** and automatic restart
- ✅ **Resource isolation** (separate namespaces/clusters for different workloads)
- ✅ **Backup and restore** strategy
- ✅ **Multi-zone deployment** (distribute across AZs)
- ✅ **Disaster recovery plan** (multi-region for critical workloads)

**RTO/RPO goals:**

```
Recovery Time Objective (RTO): < 5 minutes
- Automatic failover to standby workers/frontends

Recovery Point Objective (RPO): Zero
- All workflow history durably persisted
- No workflow data loss on failures
```

📖 **Review:** [Lesson 9](lesson_9.md#high-availability)

</details>

---

### Question 11: Security

**What security measures should you implement for production Temporal?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Implement **TLS encryption**, **authentication/authorization**, **network policies**, **secrets management**, and **audit logging**.

**1. TLS Encryption (mTLS):**

```go
// Client with TLS
tlsConfig, err := credentials.LoadClientTLSConfig(
    "certs/ca.crt",       // CA certificate
    "certs/client.crt",   // Client certificate
    "certs/client.key",   // Client private key
)

temporalClient, err := client.Dial(client.Options{
    HostPort:      "temporal:7233",
    Namespace:     "production",
    ConnectionOptions: client.ConnectionOptions{
        TLS: tlsConfig,  // ✅ Encrypted connection
    },
})
```

**Server-side TLS (Temporal config):**

```yaml
# temporal-server config.yaml
global:
  tls:
    frontend:
      server:
        certFile: /certs/server.crt
        keyFile: /certs/server.key
        clientCAFiles:
          - /certs/ca.crt
        requireClientAuth: true  # mTLS (mutual authentication)
    internode:
      server:
        certFile: /certs/internode.crt
        keyFile: /certs/internode.key
      client:
        serverName: temporal-cluster
```

**2. Authentication (using authorizer plugin):**

```go
// Custom authorizer
type CustomAuthorizer struct{}

func (a *CustomAuthorizer) Authorize(
    ctx context.Context,
    claims *authorization.Claims,
    target *authorization.CallTarget,
) (authorization.Result, error) {
    // Extract user from JWT
    user := claims.Subject

    // Check permissions
    if target.APIName == "StartWorkflowExecution" {
        if !hasPermission(user, "workflow:write") {
            return authorization.Result{Decision: authorization.DecisionDeny}, nil
        }
    }

    return authorization.Result{Decision: authorization.DecisionAllow}, nil
}

// Configure server with authorizer
serverOptions := &temporal.ServerOptions{
    Authorizer: &CustomAuthorizer{},
}
```

**3. Network Policies (Kubernetes):**

```yaml
# Allow only specific services to access Temporal
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: temporal-access
spec:
  podSelector:
    matchLabels:
      app: temporal-frontend
  policyTypes:
  - Ingress
  ingress:
  # Allow from API pods
  - from:
    - podSelector:
        matchLabels:
          app: order-api
    ports:
    - protocol: TCP
      port: 7233
  # Allow from worker pods
  - from:
    - podSelector:
        matchLabels:
          app: order-worker
    ports:
    - protocol: TCP
      port: 7233
  # Deny all other traffic
```

**4. Secrets Management:**

```yaml
# Store credentials in Kubernetes secrets
apiVersion: v1
kind: Secret
metadata:
  name: temporal-secrets
type: Opaque
data:
  db-password: <base64-encoded>
  stripe-api-key: <base64-encoded>
  jwt-secret: <base64-encoded>

---

# Reference in worker deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-worker
spec:
  template:
    spec:
      containers:
      - name: worker
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: temporal-secrets
              key: db-password
        - name: STRIPE_API_KEY
          valueFrom:
            secretKeyRef:
              name: temporal-secrets
              key: stripe-api-key
```

**Access secrets in activities (not workflows!):**

```go
// ❌ DON'T store secrets in workflow code
func OrderWorkflow(ctx workflow.Context, order Order) error {
    apiKey := "sk_live_secret123"  // ❌ Visible in history!
}

// ✅ Access secrets in activities
func ChargePaymentActivity(ctx context.Context, order Order) error {
    // Read from environment (injected from secret)
    apiKey := os.Getenv("STRIPE_API_KEY")  // ✅ Not in workflow history

    // Use secret
    stripeClient := stripe.New(apiKey)
    return stripeClient.Charge(order.Total)
}
```

**5. Data encryption at rest:**

```yaml
# Encrypt workflow payloads
apiVersion: v1
kind: ConfigMap
metadata:
  name: temporal-encryption
data:
  config.yaml: |
    encryption:
      dataConverter:
        encryptionKeyId: "key-2025-01"
        kmsProvider: "aws-kms"
        kmsKeyArn: "arn:aws:kms:us-east-1:123:key/abc"
```

**Custom data converter for encryption:**

```go
import (
    "go.temporal.io/sdk/converter"
    "go.temporal.io/sdk/worker"
)

// Encrypt workflow inputs/outputs
type EncryptedDataConverter struct {
    encryptionKey []byte
}

func (dc *EncryptedDataConverter) ToPayload(value interface{}) (*commonpb.Payload, error) {
    // Serialize
    data, err := json.Marshal(value)
    if err != nil {
        return nil, err
    }

    // Encrypt
    encrypted := encrypt(data, dc.encryptionKey)

    return &commonpb.Payload{
        Metadata: map[string][]byte{
            "encoding": []byte("encrypted"),
        },
        Data: encrypted,
    }, nil
}

// Use in worker
w := worker.New(client, "queue", worker.Options{
    DataConverter: &EncryptedDataConverter{
        encryptionKey: loadKeyFromKMS(),
    },
})
```

**6. Audit logging:**

```go
// Log all workflow operations
type AuditLogger struct {
    logger *log.Logger
}

func (a *AuditLogger) LogWorkflowExecution(workflowID, user string, action string) {
    a.logger.Printf(
        "action=%s workflow_id=%s user=%s timestamp=%s",
        action,
        workflowID,
        user,
        time.Now().UTC(),
    )
}

// In workflow starter
func CreateOrder(ctx context.Context, order Order) error {
    auditLogger.LogWorkflowExecution(order.ID, getCurrentUser(ctx), "start")

    _, err := client.ExecuteWorkflow(ctx, options, OrderWorkflow, order)

    return err
}
```

**7. RBAC (Role-Based Access Control):**

```yaml
# Define roles
roles:
  - name: workflow-admin
    permissions:
      - workflow:*
      - activity:*
      - signal:*
      - query:*

  - name: workflow-viewer
    permissions:
      - workflow:read
      - query:execute

  - name: workflow-operator
    permissions:
      - workflow:start
      - workflow:cancel
      - signal:send

# Assign to users
users:
  - email: admin@company.com
    role: workflow-admin
  - email: support@company.com
    role: workflow-viewer
  - email: api@company.com
    role: workflow-operator
```

**Security checklist:**

- ✅ **TLS encryption** for all connections
- ✅ **mTLS** for client authentication
- ✅ **Authorization** plugin for access control
- ✅ **Network policies** to restrict access
- ✅ **Secrets management** (never hardcode)
- ✅ **Encrypt sensitive data** in payloads
- ✅ **Audit logging** for compliance
- ✅ **Regular security updates** (patch Temporal server)
- ✅ **Principle of least privilege** (minimal permissions)
- ✅ **Secure defaults** (deny by default)

📖 **Review:** [Lesson 9](lesson_9.md#security)

</details>

---

### Question 12: Scaling

**Your workflow execution time increases from 1 second to 30 seconds during peak hours. What should you check first?**

<details>
<summary>Click to reveal answer</summary>

**Answer:** Check **task queue depth** and **worker capacity** - you likely need to scale workers horizontally.

**Diagnostic process:**

**1. Check task queue metrics:**

```bash
# Temporal CLI
temporal operator task-queue describe --task-queue order-processing

# Look for:
- Backlog size: 5000 tasks pending  # ⚠️ HIGH!
- Pollers: 3 workers polling        # ⚠️ TOO FEW!
```

**Grafana query:**
```promql
# Task queue depth
temporal_task_queue_depth{queue="order-processing"}

# If graph shows sustained growth → need more workers
```

**2. Check worker capacity:**

```promql
# Worker slot utilization
temporal_worker_task_slots_used /
(temporal_worker_task_slots_used + temporal_worker_task_slots_available)

# If > 90% → workers are saturated, scale up
```

**3. Scale workers:**

```bash
# Kubernetes: Scale deployment
kubectl scale deployment order-worker --replicas=10

# Or use HPA (recommended)
kubectl autoscale deployment order-worker \
  --min=3 \
  --max=20 \
  --cpu-percent=70
```

**Performance investigation checklist:**

| Symptom | Root Cause | Solution |
|---------|------------|----------|
| **High execution time** | Task queue backlog | Scale workers |
| **Activities timing out** | Downstream service slow | Increase timeouts, optimize service |
| **Workflow tasks slow** | Complex workflow logic | Simplify workflow, move logic to activities |
| **Database slow queries** | Temporal DB overloaded | Scale database, add indexes |
| **Network latency** | Worker-server distance | Deploy workers closer to Temporal |

**Detailed metrics analysis:**

```promql
# 1. Check queue lag
histogram_quantile(0.95,
  rate(temporal_task_schedule_to_start_latency_seconds_bucket[5m])
)
# P95 > 5s → queue backlog

# 2. Check activity execution time
histogram_quantile(0.95,
  rate(temporal_activity_execution_latency_seconds_bucket{
    activity_type="ChargePayment"
  }[5m])
)
# P95 increased → downstream service issue

# 3. Check worker throughput
rate(temporal_worker_task_execution_total[5m])
# Throughput flat despite queue growth → need more workers
```

**Scaling strategies:**

**1. Horizontal scaling (add workers):**
```yaml
# Before: 3 workers, 50 slots each = 150 capacity
replicas: 3
maxConcurrentActivityExecutionSize: 50

# After: 10 workers, 50 slots each = 500 capacity
replicas: 10
maxConcurrentActivityExecutionSize: 50
```

**2. Vertical scaling (increase slots per worker):**
```yaml
# Before
maxConcurrentActivityExecutionSize: 50

# After (if workers have spare CPU/memory)
maxConcurrentActivityExecutionSize: 100
```

**3. Optimize activity performance:**
```go
// Before: Sequential database queries (slow)
func ProcessOrderActivity(ctx context.Context, order Order) error {
    customer := db.GetCustomer(order.CustomerID)      // 100ms
    inventory := db.GetInventory(order.ProductID)     // 100ms
    pricing := db.GetPricing(order.ProductID)         // 100ms
    // Total: 300ms per activity
}

// After: Parallel queries (fast)
func ProcessOrderActivity(ctx context.Context, order Order) error {
    var customer Customer
    var inventory Inventory
    var pricing Pricing

    err := parallel.Run(
        func() error { return db.GetCustomer(order.CustomerID, &customer) },
        func() error { return db.GetInventory(order.ProductID, &inventory) },
        func() error { return db.GetPricing(order.ProductID, &pricing) },
    )
    // Total: 100ms per activity (3x faster!)
}
```

**4. Use caching:**
```go
var pricingCache = cache.New(5*time.Minute, 10*time.Minute)

func GetPricingActivity(ctx context.Context, productID string) (Pricing, error) {
    // Check cache first
    if cached, found := pricingCache.Get(productID); found {
        return cached.(Pricing), nil
    }

    // Cache miss, fetch from database
    pricing := db.GetPricing(productID)
    pricingCache.Set(productID, pricing, cache.DefaultExpiration)

    return pricing, nil
}
```

**5. Batch processing:**
```go
// Before: Process one order at a time
for _, order := range orders {
    workflow.ExecuteActivity(ctx, ProcessOrder, order).Get(ctx, nil)
}

// After: Batch orders
workflow.ExecuteActivity(ctx, ProcessOrderBatch, orders).Get(ctx, nil)

func ProcessOrderBatchActivity(ctx context.Context, orders []Order) error {
    // Single database transaction for all orders
    // Much faster than individual transactions
}
```

**When NOT to scale workers:**

If you see:
- ❌ Task queue depth is low (< 10)
- ❌ Worker CPU usage is low (< 30%)
- ❌ Activity execution time is high (external service slow)

Then scaling won't help. Instead:
- 🔧 Optimize the slow external service
- 🔧 Increase activity timeouts
- 🔧 Add retries with backoff
- 🔧 Use caching

**Quick diagnostic script:**

```bash
#!/bin/bash
echo "=== Task Queue Health ==="
temporal operator task-queue describe --task-queue $QUEUE

echo "=== Worker Metrics ==="
kubectl top pods -l app=order-worker

echo "=== Recent Workflow Executions ==="
temporal workflow list --query 'ExecutionStatus="Running"' --limit 10

# If backlog > 1000 and worker CPU > 80%:
echo "⚠️ Scale workers!"
kubectl scale deployment order-worker --replicas=10
```

📖 **Review:** [Lesson 9](lesson_9.md#scaling)

</details>

---

## 🎯 Scoring Guide

Count how many you got right on the first try:

- **10-12 correct:** 🌟 Excellent! You're ready to run Temporal in production
- **7-9 correct:** 👍 Strong foundation, review edge cases and best practices
- **4-6 correct:** 📚 Good grasp of basics, study production patterns more
- **0-3 correct:** 🔄 Revisit Part 3 lessons and practice with real deployments

---

## 📚 What's Next?

Congratulations on completing Part 3!

**Continue learning:**
- **[Lesson 10: Compensation & Saga Patterns](lesson_10.md)** - Advanced patterns
- **Review earlier material:**
  - [Part 1 Quiz](quiz_part1.md)
  - [Part 2 Quiz](quiz_part2.md)
- **[Back to Course Index](course.md)**

**Practice:**
- Deploy a workflow to production
- Implement comprehensive testing
- Set up monitoring and alerting
- Practice disaster recovery scenarios

---

**Ready for production?** You now have the knowledge to design, test, and deploy production-ready Temporal workflows! 🚀

---

_Part 3 Quiz • Temporal Fast Course • Last Updated: November 2025_