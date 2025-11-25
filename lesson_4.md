# Lesson 4: Running Your First Workflow End-to-End

## Learning Objectives
By the end of this lesson you will:
- ✅ Start a local Temporal development server using the Temporal CLI
- ✅ Run a worker and starter to execute `OrderWorkflow`
- ✅ Inspect workflow execution in the Temporal UI
- ✅ Retrieve workflow results programmatically
- ✅ Troubleshoot common local setup issues

[← Back to Course Index](course.md) | [← Previous: Lesson 3](lesson_3.md) | [Next: Lesson 5 →](lesson_5.md)

---
## Why Before How: Why Run Locally?
You need a reproducible environment to:
- Test workflow registration and determinism
- Inspect execution history for debugging
- Validate retry behavior and failure recovery
- Prototype workflow patterns before production

Running locally lowers feedback cycle time and builds intuition about workflow state transitions.

---
## Stack Overview
We'll use the Temporal CLI to run a local development server:
| Component | Purpose | Port |
|-----------|---------|------|
| `temporal server` | Temporal server (all services in one) | 7233 |
| `temporal ui` | Web UI to inspect workflows | 8233 |
| `sqlite` | Local persistence (temporal.db file) | - |

The Temporal CLI provides:
- All-in-one development server
- Built-in Web UI
- SQLite database for persistence (no PostgreSQL needed)
- Zero configuration required

---
## Start the Stack
Open a terminal at project root and run:
```bash
# Option 1: Run in foreground (recommended for first time)
./start-temporal.sh

# Option 2: Run in background
make start-bg

# Option 3: Use temporal CLI directly
temporal server start-dev --db-filename ./temporal.db
```

**Expected output:**
```
CLI 1.2.0 (Server 1.29.0, UI 2.37.0)

Server:  localhost:7233
UI:      http://localhost:8233
Metrics: http://localhost:55477/metrics
```

---
## Verify Temporal Connectivity
Check if the server is running:
```bash
# Using make
make status

# Or using temporal CLI
temporal workflow list

# Or check the process
pgrep -f "temporal server start-dev"
```

You can also open the Web UI in your browser:
```bash
# Using make
make ui

# Or open directly
open http://localhost:8233
```

---
## Run the Worker
In a new terminal:
```bash
# From project root
go run worker/main.go
```
**Expected Output:** Logs indicating polling for workflow and activity task queues. It should register `OrderWorkflow`.

---
## Start a Workflow Execution
In another terminal:
```bash
go run starter/main.go
```
**Starter Responsibilities:**
- Creates Temporal client
- Starts `OrderWorkflow` with WorkflowID `order_workflow_12345` on queue `order-task-queue`
- Waits synchronously for result

**Possible Outcomes:**
- Success: Logs "Workflow result: Order ORDER-12345 completed successfully"
- Failure (payment step): `ProcessPayment` may fail randomly without retry policy → starter logs fatal.

---
## Add Retries (Optional Stability Upgrade)
Modify `order_workflow.go` to include a `RetryPolicy` in ActivityOptions (see Lesson 3). This reduces random fatal failures.

---
## Inspect Workflow in UI
Open the Temporal Web UI:
```bash
# Using make
make ui

# Or open directly in browser
open http://localhost:8233
```

Navigate to:
- Workflows list → Find WorkflowID `order_workflow_12345`
- Open execution → Review event history:
  - `WorkflowExecutionStarted`
  - `ActivityTaskScheduled` / `ActivityTaskCompleted` for each step
  - `WorkflowExecutionCompleted`

**If UI doesn't load:**
1. Check if Temporal server is running: `make status`
2. Verify the server started successfully: `tail temporal.log`
3. Restart the server: `make restart`

---
## Programmatically Retrieve Workflow Result Later
Instead of waiting immediately, you can:
```go
we, _ := c.ExecuteWorkflow(context.Background(), workflowOptions, order.OrderWorkflow, orderID)
// Store IDs (e.g., in DB)
workflowID := we.GetID()
runID := we.GetRunID()

// Later:
handle := c.GetWorkflow(context.Background(), workflowID, runID)
var result string
_ = handle.Get(context.Background(), &result)
```
This enables asynchronous processing and decouples API responses from workflow completion.

---
## Troubleshooting Matrix
| Symptom | Cause | Fix |
|---------|-------|-----|
| Workflow never runs | Wrong task queue name | Ensure starter + worker use identical string |
| Starter fatal: cannot connect | Temporal not running | Check `make status`, run `make start-bg` |
| UI doesn't load | Server not started | Check logs: `tail temporal.log` or `make logs` |
| Payment step fails randomly | No retries | Add ActivityOptions RetryPolicy |
| "namespace not found" | Server not fully started | Wait a few seconds, retry |
| Connection refused | Wrong port | Ensure using localhost:7233 for server, :8233 for UI |

---
## Exercise
1. Trigger 5 workflow executions with unique IDs:
    ```bash
    for i in $(seq 1 5); do 
      WORKFLOW_ID="order_workflow_$i" go run starter/main.go; 
    done
    ```
2. Observe concurrency in UI.
3. Introduce a failure by forcing `ProcessPayment` to always error; verify retry behavior after adding policy.
4. Capture event history (export JSON from UI) and identify the sequence.

---
## Advanced: Using the Temporal CLI
List workflows:
```bash
temporal workflow list

# Or use make
make list
```
Describe a workflow:
```bash
temporal workflow describe --workflow-id order_workflow_12345

# Or use make
make describe ID=order_workflow_12345
```
Show workflow history:
```bash
temporal workflow show --workflow-id order_workflow_12345

# Or use make
make show ID=order_workflow_12345
```
Terminate a workflow:
```bash
temporal workflow terminate --workflow-id order_workflow_12345 --reason "Testing termination"

# Or use make
make terminate ID=order_workflow_12345
```

---
## What You've Learned
✅ How to start Temporal locally with the Temporal CLI
✅ How workers and starters interact with the server
✅ How to inspect workflow history in UI
✅ How to retrieve results asynchronously
✅ How to troubleshoot common environment issues
✅ How to use the Temporal CLI for workflow management  

---
## Ready for Lesson 5?
Lesson 5 will cover **Error Handling & Retries** in depth:
- Activity failure classification
- Retry policies design
- Heartbeats for long-running tasks
- Compensating transactions (sagas)

Say: **"I'm ready for Lesson 5"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 3](lesson_3.md) | [Next: Lesson 5 →](lesson_5.md)

