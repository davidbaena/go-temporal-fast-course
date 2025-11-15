# Lesson 1: What is Temporal?

## Why Learn Temporal?

Imagine you need to process an order:
1. Reserve inventory
2. Charge payment
3. Send confirmation email
4. Schedule delivery

**Challenges without Temporal:**
- What if the payment service is down? You need retry logic.
- What if your server crashes between step 2 and 3? You lose state.
- What if the email takes 5 minutes? Your API times out.
- You need to write custom code for: retries, timeouts, state persistence, failure recovery, monitoring.

**With Temporal:**
- Write your business logic as simple functions
- Temporal handles: retries, timeouts, state persistence, failure recovery
- Your workflow survives server crashes, restarts, and network failures
- You get observability and history out of the box

## What is Temporal?

**Temporal is a durable execution platform.** It guarantees your code runs to completion, even in the face of failures.

**Key concept:** Instead of writing imperative code with error handling everywhere, you write declarative workflows that describe *what should happen*, and Temporal ensures it happens.

---

## Core Concepts (The Foundation)

### 1. **Workflow**
A workflow is a function that orchestrates business logic. It's the "brain" of your process.

**Characteristics:**
- **Deterministic**: Same inputs → same outputs (no random numbers, no system calls)
- **Durable**: Survives crashes, restarts, and deployments
- **Long-running**: Can run for seconds, hours, days, or months
- **Versioned**: Can be updated without breaking in-flight workflows

**Analogy:** A workflow is like a recipe. It describes the steps, but doesn't do the cooking itself.

**Example use cases:**
- Order processing (reserve → charge → fulfill → notify)
- User onboarding (create account → send email → wait for verification → enable features)
- Data pipeline (extract → transform → load → validate)

### 2. **Activity**
An activity is a function that performs actual work (side effects). It's the "hands" of your process.

**Characteristics:**
- **Non-deterministic**: Can make HTTP calls, query databases, send emails
- **Retriable**: Temporal automatically retries on failure
- **Idempotent**: Should be safe to retry (design for this!)
- **Timeout-protected**: Temporal enforces timeouts

**Analogy:** Activities are the actual cooking steps (chop onions, boil water, etc.)

**Example activities:**
- `ChargePayment(orderID, amount)`
- `SendEmail(to, subject, body)`
- `ReserveInventory(productID, quantity)`

### 3. **Worker**
A worker is a process that executes workflows and activities. It polls for tasks and runs your code.

**Characteristics:**
- Runs on your infrastructure (servers, containers, lambdas)
- Registers workflows and activities it can execute
- Polls the Temporal server for work
- Multiple workers can run for horizontal scaling

**Analogy:** Workers are the chefs in your kitchen.

### 4. **Task Queue**
A task queue is a named channel that routes work to workers.

**Characteristics:**
- Workers listen on specific task queues
- You send workflows to task queues when starting them
- Provides isolation and routing (e.g., `order-processing-queue`, `email-queue`)

**Analogy:** Task queues are the order tickets in a restaurant kitchen.

### 5. **Workflow Execution**
A workflow execution is a single run of a workflow with a unique ID.

**Characteristics:**
- Has a unique `WorkflowID` (you provide this, e.g., `order-12345`)
- Stores complete event history (audit trail)
- Can be queried, signaled, or cancelled
- Maintains state automatically

**Analogy:** A workflow execution is one specific order being prepared.

---

## How Temporal Works (High-Level Architecture)

```
┌─────────────────────────────────────────────────────────┐
│                     Your Application                     │
│                                                           │
│  ┌──────────────┐         ┌──────────────┐              │
│  │   Starter    │         │    Worker    │              │
│  │  (Client)    │         │              │              │
│  │              │         │ - Executes   │              │
│  │ - Starts     │         │   Workflows  │              │
│  │   Workflows  │         │ - Executes   │              │
│  │ - Queries    │         │   Activities │              │
│  │   Status     │         │              │              │
│  └──────┬───────┘         └──────┬───────┘              │
│         │                        │                       │
└─────────┼────────────────────────┼───────────────────────┘
          │                        │
          │ gRPC                   │ gRPC (poll for tasks)
          │                        │
┌─────────▼────────────────────────▼───────────────────────┐
│              Temporal Server (Cluster)                    │
│                                                           │
│  - Stores workflow state & history                       │
│  - Routes tasks to workers via task queues               │
│  - Handles retries, timeouts, timers                     │
│  - Provides durability guarantees                        │
│                                                           │
│  ┌─────────────┐   ┌──────────────┐   ┌──────────────┐  │
│  │  Frontend   │   │   History    │   │   Matching   │  │
│  │   Service   │   │   Service    │   │   Service    │  │
│  └─────────────┘   └──────────────┘   └──────────────┘  │
│                                                           │
│  ┌───────────────────────────────────────────────────┐   │
│  │         Database (PostgreSQL / Cassandra)         │   │
│  │         - Workflow histories                       │   │
│  │         - Task queues                              │   │
│  └───────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────┘
```

**Flow:**
1. **Starter** sends a "start workflow" request to Temporal Server
2. **Temporal Server** stores the workflow execution and puts a task on the task queue
3. **Worker** polls the task queue, receives the task
4. **Worker** executes the workflow code
5. Workflow schedules activities → Temporal puts activity tasks on task queue
6. **Worker** polls, receives activity task, executes activity code
7. Activity result is sent back to Temporal → Temporal updates workflow state
8. Workflow continues or completes

---

## Key Guarantees

Temporal provides these guarantees:

1. **At-least-once execution**: Activities will execute at least once (design for idempotency!)
2. **Exactly-once state transitions**: Workflow state changes are exactly once
3. **Durable state**: Workflow state survives crashes and restarts
4. **Event sourcing**: Complete history of everything that happened
5. **Automatic retries**: Configurable retry policies for activities and workflows

---

## When to Use Temporal?

✅ **Use Temporal when:**
- You have multi-step processes that must complete reliably
- You need to coordinate multiple services
- Your process involves waiting (timeouts, human approval, scheduled tasks)
- You need audit trails and observability
- You want to avoid writing custom retry/queue/state management

❌ **Don't use Temporal when:**
- You have simple, single-step operations (use regular functions)
- You need real-time, sub-millisecond latency (use event streams)
- Your logic is purely computational (no side effects)

---

## How Temporal Fits This Project

In `e-commerce platform`, we have:
- ✅ User authentication
- ✅ Product catalog
- ❌ Shopping cart
- ❌ **Order processing (perfect for Temporal!)**

**Order processing workflow:**
1. Validate order
2. Reserve inventory
3. Charge payment
4. Create shipment
5. Send confirmation email
6. Wait for delivery
7. Send delivery notification

This is a **perfect Temporal use case** because:
- Multiple steps with external dependencies (payment gateway, email service, inventory system)
- Needs retries (payment service might be down)
- Long-running (delivery tracking takes days)
- Requires durability (can't lose orders!)

---

## Prerequisites

Before continuing, ensure you have:
- ✅ Go 1.21+ installed
- ✅ Docker Desktop installed (for running Temporal server locally)
- ✅ Basic Go knowledge (functions, structs, interfaces, goroutines)
- ✅ Basic HTTP/REST knowledge
- ✅ Terminal/command line comfort (zsh on macOS)

---

## First Steps: Running Temporal Locally

Let's verify your environment is ready:

```bash
# 1. Check Go version
go version
# Should show: go version go1.21.x or higher

# 2. Check Docker
docker --version
# Should show: Docker version 20.x or higher

# 3. Verify docker-compose
docker-compose --version
# Should show: docker-compose version 1.x or 2.x

# 4. Check if Temporal server is already running
docker ps | grep temporal
# If nothing shows, we'll start it in Lesson 4
```

---

## What You've Learned

✅ What Temporal is and why it exists  
✅ The 5 core concepts: Workflow, Activity, Worker, Task Queue, Workflow Execution  
✅ How Temporal architecture works (high-level)  
✅ Temporal's durability guarantees  
✅ When to use (and not use) Temporal  
✅ How Temporal fits into this project (order processing)  

---

## Next Steps

In **[Lesson 2: Workflows & Activities](lesson_2.md)**, we'll:
- Write your first workflow in Go
- Create activities that do real work
- Understand determinism and why it matters
- Learn workflow patterns (sequential, parallel, child workflows)

---

## 📝 Exercise (Optional)

Before moving on, think about your current project:
1. Identify 2-3 processes that involve multiple steps
2. Which ones have failure points (external APIs, databases)?
3. Which ones need to run reliably even if your server crashes?
4. Write them down in `temporal/use-cases.md`

---

## 🆘 Context Management

If at any point I (the AI) lose context or seem confused, type:
```
"Save progress to temporal/compressed.md"
```

I'll create a compressed summary of what we've covered and our current position in the course.

---

**Ready to continue?** Let me know when you want to start **[Lesson 2: Workflows & Activities](lesson_2.md)**!

[← Back to Course Index](course.md)

