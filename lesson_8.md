# Lesson 8: Testing & Best Practices

## Learning Objectives
By the end of this lesson you will:
- ✅ Write unit tests for workflows using Temporal's test environment
- ✅ Mock and stub Activities cleanly (interface-based + function-based)
- ✅ Validate workflow determinism & versioning (`GetVersion` usage)
- ✅ Test Signals, Queries, Timers, and parallel execution
- ✅ Structure test fixtures for readability and reuse
- ✅ Adopt production best practices (timeouts, retries, idempotency, observability)
- ✅ Apply coding guidelines that prevent future non-deterministic breaks

[← Back to Course Index](course.md) | [← Previous: Lesson 7](lesson_7.md) | [Next: Lesson 9 →](lesson_9.md)

---
## Why Before How: Why Test Temporal Workflows?
Temporal's guarantees rely on deterministic workflow logic. A subtle non-deterministic change (e.g., `time.Now()` added later) can break replay for in-flight executions.

Testing verifies:
- Business logic correctness (stage transitions, compensation paths)
- Determinism (stable replay results)
- Signal/Query handling
- Version compatibility (old vs new paths)

---
## Types of Tests
| Test Type | Purpose | Scope | Tooling |
|-----------|---------|-------|---------|
| Unit (Workflow Logic) | Validate orchestrated decisions | Workflow function only | Temporal test environment (`testsuite.WorkflowTestSuite`) |
| Unit (Activity) | Validate side-effect behavior | Single activity method | Standard Go testing / mocks |
| Integration (Workflow + Real Activities) | End-to-end with real implementations | Worker + test env | Temporal test environment with registered activities |
| Determinism / Replay | Ensure history replays identically | Workflow execution history | Temporal test env `env.ReplayWorkflowHistory` |
| Versioning | Guard migrations | `GetVersion` branches | Run both versions against sample inputs |
| Performance / Load (Optional) | Stress concurrency & scaling | Multiple executions | Custom harness / `go test -run ^$ -bench .` |

---
## Testing Tools Overview
Temporal Go SDK provides a powerful test environment:
```go
import "go.temporal.io/sdk/testsuite"

type UnitTestSuite struct { testsuite.WorkflowTestSuite }

func (s *UnitTestSuite) TestOrderWorkflow_ApprovalPath(t *testing.T) {
    env := s.NewTestWorkflowEnvironment()
    // Register workflow + mocked activities
    env.RegisterWorkflow(OrderWorkflow)
    env.RegisterActivity(MockReserveStock)
    // ...
    env.ExecuteWorkflow(OrderWorkflow, "ORDER-123", []LineItem{{SKU: "BOOK-1", Quantity: 1}})
}
```

Key capabilities:
- Override activity implementations
- Send signals during test execution
- Fast virtual time (timers advance instantly)
- Replay history

---
## Setting Up a Test Suite
Create file: `order/order_workflow_test.go` (example sketch)
```go
package order_test

import (
    "testing"
    "time"

    "go.temporal.io/sdk/testsuite"
    "go.temporal.io/sdk/workflow"

    // Assume your package path
    "go-temporal-fast-course/order"
)

type OrderWorkflowSuite struct { testsuite.WorkflowTestSuite }

type ActivitiesMock struct{}

// Activity mocks
func (a *ActivitiesMock) ReserveStock(ctx context.Context, orderID string, items []order.LineItem) error { return nil }
func (a *ActivitiesMock) ReleaseStock(ctx context.Context, orderID string) error { return nil }
func (a *ActivitiesMock) ProcessPayment(ctx context.Context, orderID string) error { return nil }
func (a *ActivitiesMock) RefundPayment(ctx context.Context, orderID string) error { return nil }
func (a *ActivitiesMock) UpdateOrderStatus(ctx context.Context, orderID string, status string) error { return nil }
func (a *ActivitiesMock) SendOrderConfirmation(ctx context.Context, orderID string, email string) error { return nil }
func (a *ActivitiesMock) SendCancellationEmail(ctx context.Context, orderID string, reason string) error { return nil }
func (a *ActivitiesMock) FetchInventorySnapshot(ctx context.Context, items []order.LineItem) (bool, error) { return true, nil }
func (a *ActivitiesMock) FetchCustomerProfile(ctx context.Context, orderID string) (string, error) { return "GOLD", nil }
func (a *ActivitiesMock) FetchRecommendations(ctx context.Context, orderID string) ([]string, error) { return []string{"BOOK-REC"}, nil }

func TestOrderWorkflow_ApprovalPath(t *testing.T) {
    suite := new(OrderWorkflowSuite)
    env := suite.NewTestWorkflowEnvironment()

    // Register workflow & activities
    env.RegisterWorkflow(order.OrderWorkflow)
    mock := &ActivitiesMock{}
    env.RegisterActivity(mock)

    // Start workflow
    orderID := "ORDER-1001"
    items := []order.LineItem{{SKU: "BOOK-1", Quantity: 1}}
    env.ExecuteWorkflow(order.OrderWorkflow, orderID, items)

    // Simulate signal sequence (approval)
    env.SignalWorkflow("approve-payment", order.PaymentApproval{ApprovedBy: "admin"})

    // Wait for completion
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())

    var result string
    require.NoError(t, env.GetWorkflowResult(&result))
    require.Contains(t, result, "completed")
}
```

### Notes:
- Use `require` from `testify` for readability (add dependency if needed).
- Signals can be sent after `ExecuteWorkflow` and before completion.
- Timers auto-fire (virtual time) → No real waiting.

---
## Testing Cancellation Path
```go
func TestOrderWorkflow_CancellationBeforeApproval(t *testing.T) {
    env := testsuite.WorkflowTestSuite{}.NewTestWorkflowEnvironment()
    env.RegisterWorkflow(order.OrderWorkflow)
    env.RegisterActivity(&ActivitiesMock{})

    env.ExecuteWorkflow(order.OrderWorkflow, "ORDER-2002", []order.LineItem{})
    env.SignalWorkflow("cancel-order", order.CancelRequest{Reason: "User changed mind"})

    env.AssertExpectations(t)
    require.True(t, env.IsWorkflowCompleted())

    var result string
    _ = env.GetWorkflowResult(&result)
    require.Contains(t, result, "cancelled")
}
```

---
## Testing Approval Timeout (Timer + No Signal)
```go
func TestOrderWorkflow_ApprovalTimeout(t *testing.T) {
    env := testsuite.WorkflowTestSuite{}.NewTestWorkflowEnvironment()
    env.RegisterWorkflow(order.OrderWorkflow)
    env.RegisterActivity(&ActivitiesMock{})

    env.ExecuteWorkflow(order.OrderWorkflow, "ORDER-3003", []order.LineItem{})

    // Advance virtual time beyond approval timeout
    env.AdvanceTime(16 * time.Minute)

    require.True(t, env.IsWorkflowCompleted())
    var result string
    _ = env.GetWorkflowResult(&result)
    require.Contains(t, result, "cancelled")
}
```

---
## Testing Queries
```go
func TestOrderWorkflow_QueryStatus(t *testing.T) {
    env := testsuite.WorkflowTestSuite{}.NewTestWorkflowEnvironment()
    env.RegisterWorkflow(order.OrderWorkflow)
    env.RegisterActivity(&ActivitiesMock{})

    env.ExecuteWorkflow(order.OrderWorkflow, "ORDER-4004", []order.LineItem{})
    env.SignalWorkflow("approve-payment", order.PaymentApproval{ApprovedBy: "system"})

    // Query midway (before completion maybe) - ensure workflow not yet ended
    var status order.OrderWorkflowStatus
    require.NoError(t, env.QueryWorkflow("get-status", &status))
    require.True(t, status.PaymentApproved)
}
```

---
## Testing Versioning (GetVersion)
Simulate two versions by controlling test path:
```go
func TestOrderWorkflow_Versioning(t *testing.T) {
    env := testsuite.WorkflowTestSuite{}.NewTestWorkflowEnvironment()
    env.RegisterWorkflow(order.OrderWorkflow) // Internally uses GetVersion
    env.RegisterActivity(&ActivitiesMock{})

    env.ExecuteWorkflow(order.OrderWorkflow, "ORDER-5005", []order.LineItem{})
    env.SignalWorkflow("approve-payment", order.PaymentApproval{ApprovedBy: "admin"})
    env.AssertExpectations(t)
}
```

If you need to assert branch, expose version in result or via query.

---
## Determinism & Replay Tests
Save a real workflow history from production (UI export) → replay against current code:
```go
func TestOrderWorkflow_ReplayHistory(t *testing.T) {
    env := testsuite.WorkflowTestSuite{}.NewTestWorkflowEnvironment()
    // Provide exported history JSON path
    err := env.ReplayWorkflowHistory("testdata/order_workflow_history.json")
    require.NoError(t, err)
}
```
**If replay fails:** You've introduced a non-deterministic change—investigate added system calls, time usage, or branching.

---
## Mocking Strategies
| Strategy | When to Use | Pros | Cons |
|----------|-------------|------|------|
| Interface-based mocks | Complex activity sets | Strong abstraction | More boilerplate |
| Function-level stubs | Simple demo / few activities | Fast | Harder to scale |
| Monkey patch libs (avoid) | Rare/emergency | Quick hack | Non-deterministic risk |

Always keep mocks deterministic (no random). If simulating failure, return typed errors explicitly.

---
## Anti-Patterns in Tests
| Anti-Pattern | Risk |
|--------------|------|
| Real `time.Sleep` in workflow tests | Slows suite; time should be virtual |
| Using `time.Now()` in workflow code | Non-deterministic replay failures |
| Random branching in workflow | Replay mismatch |
| Hidden global state mutation | Hard to reason stability |
| Not asserting error types | Missing classification coverage |

---
## Best Practices (Coding & Operational)
### Coding
- Keep workflow code orchestration-only (no side effects)
- Use explicit constants for signal/query names
- Wrap external changes in Activities
- Make compensation paths explicit & test them
- Add comments for version guards (`GetVersion`)

### Operational
- Monitor activity failures & retry exhaustion metrics
- Tag logs with WorkflowID/RunID
- Export and occasionally replay random histories in CI (determinism audit)
- Roll out workflow code changes gradually (blue/green or % workers)
- Keep activity timeouts realistic (short enough to detect failure quickly, long enough for normal execution)

### Idempotency Checklist Recap
| Area | Strategy |
|------|----------|
| Payments | Store charge tokens (skip repeat) |
| Emails | Record send event (skip duplicate) |
| Inventory | Reservation keys with uniqueness constraint |
| Refund | Idempotent API or check existing refund status |

### Observability Baseline
| Metric | Purpose |
|--------|--------|
| `activity_attempts_total` | Retry severity |
| `workflow_duration_seconds` | SLA tracking |
| `compensation_executed_total` | Failure recovery frequency |
| `signal_wait_duration_seconds` | Bottleneck in approval flow |

---
## CI Integration Ideas
1. Run `go test ./...` including workflow tests (fast, in-memory).
2. Add determinism replay job (weekly) using stored histories.
3. Lint for banned calls in workflow files (`time.Now`, `rand.Int`).
4. Include test coverage for each compensation path.
5. Add benchmark for a batch of workflow starts (performance regression guard).

Benchmark skeleton:
```go
func BenchmarkOrderWorkflow(b *testing.B) {
    suite := testsuite.WorkflowTestSuite{}
    for i := 0; i < b.N; i++ {
        env := suite.NewTestWorkflowEnvironment()
        env.RegisterWorkflow(order.OrderWorkflow)
        env.RegisterActivity(&ActivitiesMock{})
        env.ExecuteWorkflow(order.OrderWorkflow, fmt.Sprintf("ORDER-%d", i), nil)
        env.SignalWorkflow("approve-payment", order.PaymentApproval{ApprovedBy: "bench"})
    }
}
```

---
## Exercise
1. Implement tests for: approval path, cancellation path, timeout path.
2. Add typed error simulation in `ProcessPayment` and assert retry attempts (inspect logs or expose counter via mock).
3. Replay a saved history after introducing a safe refactor (rename variable) → confirm success.
4. Introduce an intentional non-determinism (add `time.Now()`), run replay test → observe failure, then fix using `workflow.Now()`.
5. Add a benchmark and compare runs before/after adding parallel enrichment.

---
## Troubleshooting Table
| Symptom | Cause | Fix |
|---------|-------|-----|
| Replay test fails | Non-deterministic change | Replace time/rand with workflow-safe APIs |
| Timer never fires | Used `time.Sleep` instead of `workflow.NewTimer` | Switch to workflow timer |
| Signal test hangs | Signal name mismatch | Use constants & ensure registration order |
| Query returns zero values | Query handler not set before query | Set handler early in workflow |
| Activity mock not called | Unregistered or name mismatch | Ensure `RegisterActivity` with correct receiver |

---
## What You've Learned
✅ Unit & integration testing workflows  
✅ Mocking + activity isolation  
✅ Determinism and replay validation  
✅ Version-aware workflow evolution testing  
✅ Signals, queries, timers coverage  
✅ CI & observability best practices  
✅ Performance and resilience guardrails  

---
## Ready for Lesson 9?
Lesson 9 will cover **Production Deployment**:
- Temporal cluster + persistence strategies
- Worker deployment patterns (Kubernetes / Docker / scaling)
- Metrics, tracing, alerting integration
- Disaster recovery & workflow version migration

Say: **"I'm ready for Lesson 9"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 7](lesson_7.md) | [Next: Lesson 9 →](lesson_9.md)

