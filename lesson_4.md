# Lesson 4: Running Your First Workflow End-to-End

## Learning Objectives
By the end of this lesson you will:
- ✅ Start a local Temporal stack (PostgreSQL + Temporal + UI) using your existing docker-compose
- ✅ Run a worker and starter to execute `OrderWorkflow`
- ✅ Inspect workflow execution in the Temporal UI
- ✅ Retrieve workflow results programmatically
- ✅ Troubleshoot common local setup issues (DB wait, UI 500, missing task queue)

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
Your `docker-compose.yaml` already defines these services:
| Service | Purpose | Port |
|---------|---------|------|
| `postgresql` | Temporal persistence store | 5432 |
| `temporal` | Temporal server (frontend + matching + history) | 7233 |
| `temporal-admin-tools` | CLI utilities container (tctl) | - |
| `temporal-ui` | Web UI to inspect workflows | 8080 |

Image versions in compose:
- Server: `temporalio/auto-setup:1.29.1` (auto-creates schema)
- UI: `temporalio/ui:latest`

---
## Start the Stack
Open a terminal at project root and run:
```bash
# Start everything (detached)
docker-compose up -d

# Check container status
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

# Tail Temporal server logs (optional for debugging)
docker logs -f temporal | head -n 50
```
**Expected:** Temporal logs should show successful schema setup and readiness.

---
## Common Startup Issue: "Waiting for PostgreSQL to startup"
If Temporal logs repeatedly show that message:
1. Verify Postgres healthy:
    ```bash
    docker logs temporal-postgresql | head -n 40
    ```
2. Ensure Postgres volume is writable (your compose has `- /var/lib/postgresql/data` without a host path → it's an anonymous volume, which is fine). If permission issues arise, try:
    ```bash
    docker-compose down -v
    docker-compose up -d
    ```
3. Confirm env var `DB=postgres12` matches image capabilities. Using Postgres 15 + `DB=postgres12` is acceptable; Temporal uses compatibility mode. If failure persists, set `DB=postgres` instead:
    ```yaml
    environment:
      - DB=postgres
    ```
4. Network name is defined; verify container can resolve host:
    ```bash
    docker exec temporal ping -c1 postgresql
    ```

---
## Verify Temporal Connectivity
Run a quick `tctl` command from admin tools:
```bash
docker exec -it temporal-admin-tools tctl namespace list
```
You should see the default namespace (often `default`). If not, auto-setup may have failed—check Temporal logs.

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
Open:
```
http://localhost:8080
```
Navigate to:
- Workflows list → Find WorkflowID `order_workflow_12345`
- Open execution → Review event history:
  - `WorkflowExecutionStarted`
  - `ActivityTaskScheduled` / `ActivityTaskCompleted` for each step
  - `WorkflowExecutionCompleted`

**If UI shows 500:**
1. Refresh browser or clear cache
2. Confirm UI container logs:
    ```bash
    docker logs temporal-ui | head -n 80
    ```
3. Ensure `TEMPORAL_ADDRESS=temporal:7233` matches service name and port
4. Verify Temporal service is reachable from UI container:
    ```bash
    docker exec temporal-ui nc -z temporal 7233 || echo 'Cannot reach server'
    ```
5. If image compatibility issues persist, pin a specific UI version:
    ```yaml
    image: temporalio/ui:2.14.0
    ```

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
| Starter fatal: cannot connect | Temporal not healthy | Check `docker ps`, server logs |
| UI 500 errors | UI can't reach server | Check env, pin version, test connectivity |
| Payment step fails randomly | No retries | Add ActivityOptions RetryPolicy |
| Long startup wait | Postgres not ready | Recreate volumes, validate DB env vars |
| "namespace not found" | Auto-setup failed | Re-run docker-compose, check auto-setup logs |

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
## Advanced: Using `tctl` CLI
List workflows:
```bash
docker exec -it temporal-admin-tools tctl workflow list
```
Describe a workflow:
```bash
docker exec -it temporal-admin-tools tctl workflow describe -w order_workflow_12345
```
Terminate a workflow:
```bash
docker exec -it temporal-admin-tools tctl workflow terminate -w order_workflow_12345 --reason "Testing termination"
```

---
## What You've Learned
✅ How to start Temporal locally with docker-compose  
✅ How workers and starters interact with the server  
✅ How to inspect workflow history in UI  
✅ How to retrieve results asynchronously  
✅ How to troubleshoot common environment issues  
✅ How to use `tctl` for inspection and management  

---
## Ready for Lesson 5?
Lesson 5 will cover **Error Handling & Retries** in depth:
- Activity failure classification
- Retry policies design
- Heartbeats for long-running tasks
- Compensating transactions (sagas)

Say: **"I'm ready for Lesson 5"** when prepared.

[← Back to Course Index](course.md) | [← Previous: Lesson 3](lesson_3.md) | [Next: Lesson 5 →](lesson_5.md)

