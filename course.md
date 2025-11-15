# Temporal Fast Course: Fundamentals to Production

## 📘 Overview

This course teaches you Temporal fundamentals step-by-step for Go developers. Start with core concepts, then progressively build real workflows for order processing - a pattern applicable to any multi-step, fault-tolerant business process.

---

## 🎯 Course Structure

### **Part 1: Foundations** (Lessons 1-3)
Learn core Temporal concepts and architecture

### **Part 2: Building & Running** (Lessons 4-6)
Write, run, and interact with workflows

### **Part 3: Real-World Application** (Lessons 7-9)
Implement order processing and deploy to production

---

## 📚 Lessons

### [**Lesson 1: What is Temporal?**](lesson_1.md) ✅
**Duration:** ~15 minutes  
**Topics:**
- What is Temporal and why use it?
- Core concepts: Workflow, Activity, Worker, Task Queue, Execution
- Architecture overview
- When to use Temporal
- Real-world use cases

**What you'll learn:**
- Understand Temporal's purpose and guarantees
- Grasp the 5 fundamental building blocks
- Identify good use cases for Temporal

---

### [**Lesson 2: Workflows & Activities**](lesson_2.md) ✅
**Duration:** ~30 minutes  
**Topics:**
- Writing your first workflow in Go
- Creating activities
- Understanding determinism
- Workflow patterns (sequential, parallel, child workflows)
- Activity options and timeouts

**What you'll build:**
- A simple "Greet User" workflow
- Activities for sending emails and logging
- A sequential multi-step workflow

---

### [**Lesson 3: Workers & Task Queues**](lesson_3.md) ✅
**Duration:** ~20 minutes  
**Topics:**
- How workers poll and execute tasks
- Task queue routing and isolation
- Worker configuration and scaling
- Registering workflows and activities

**What you'll build:**
- A worker that runs your workflows
- Multiple task queues for different workloads

---

### [**Lesson 4: Running Your First Workflow**](lesson_4.md) ✅
**Duration:** ~30 minutes  
**Topics:**
- Setting up Temporal server with Docker
- Creating a workflow starter (client)
- Starting workflow executions
- Querying workflow status
- Using the Temporal Web UI

**What you'll do:**
- Start Temporal locally with docker-compose
- Run your first workflow end-to-end
- Explore the Web UI and history

---

### [**Lesson 5: Error Handling & Retries**](lesson_5.md) ✅
**Duration:** ~25 minutes  
**Topics:**
- Activity retry policies
- Workflow error handling
- Timeouts (schedule-to-close, start-to-close, etc.)
- Compensating transactions
- Dead letter queues

**What you'll learn:**
- Configure retry behavior
- Handle permanent failures gracefully
- Design for idempotency

---

### [**Lesson 6: Signals & Queries**](lesson_6.md) ✅
**Duration:** ~25 minutes  
**Topics:**
- Sending signals to running workflows
- Querying workflow state
- Human-in-the-loop patterns
- Dynamic workflows

**What you'll build:**
- An approval workflow with signals
- Status queries for real-time updates

---

### [**Lesson 7: Order Processing Workflow (Real Example)**](lesson_7.md) ✅
**Duration:** ~45 minutes  
**Topics:**
- Designing a production-ready order workflow
- Activities: validate, reserve, charge, fulfill, notify
- Saga pattern for rollbacks
- Integration with HTTP API
- Testing strategies

**What you'll build:**
- Complete order processing workflow
- REST endpoints to create and query orders
- Integration tests

---

### [**Lesson 8: Testing & Best Practices**](lesson_8.md) ✅
**Duration:** ~30 minutes  
**Topics:**
- Unit testing workflows and activities
- Integration testing with test server
- Workflow versioning strategies
- Common pitfalls and anti-patterns
- Monitoring and observability

**What you'll learn:**
- Write testable workflow code
- Test without running real Temporal server
- Version workflows safely

---

### [**Lesson 9: Production Deployment**](lesson_9.md) ✅
**Duration:** ~30 minutes  
**Topics:**
- Deploying Temporal to Kubernetes
- Worker deployment patterns
- High availability and scaling
- Security and authentication
- Monitoring and alerting

**What you'll learn:**
- Deploy workers alongside your Go API
- Configure production-ready Temporal cluster
- Monitor workflow health

---

## 🛠️ Prerequisites

Before starting, ensure you have:
- ✅ Go 1.21+ installed
- ✅ Docker Desktop installed
- ✅ Basic Go knowledge (functions, structs, interfaces)
- ✅ Terminal/command line comfort (bash/zsh)
- ✅ Text editor or IDE (GoLand, VS Code)

---

## 📁 Project Structure

As we progress, we'll work with this structure:

```
go-temporal-fast-course/
├── course.md              # This file (course index)
├── README.md              # Getting started guide
├── lesson_1.md           # Lesson 1 content
├── lesson_2.md           # Lesson 2 content
├── ...                   # More lessons
├── docker-compose.yml    # Local Temporal stack
├── go.mod                # Go module definition
├── helloworld/           # Simple example
│   └── helloworld.go
├── order/                # Order workflow
│   └── order_workflow.go
├── worker/               # Worker implementation
│   └── main.go
└── starter/              # Workflow starter (client)
    └── main.go
```

---

## 🎯 Learning Approach

This course follows a **progressive, hands-on approach**:

1. **Why before How**: Understand the problem before learning the solution
2. **Small Steps**: Each lesson builds on the previous one
3. **Code Examples**: Every concept includes Go code you can run
4. **Real Application**: Build a complete order processing workflow
5. **Iterative**: Each lesson adds new capabilities

---

## 🚀 Getting Started

**Ready to begin?** 

👉 Start with **[Lesson 1: What is Temporal?](lesson_1.md)**

Once you've completed Lesson 1, continue to Lesson 2 and so on.

---

## 📊 Progress Tracker

Track your progress as you complete each lesson:

- [ ] Lesson 1: What is Temporal?
- [ ] Lesson 2: Workflows & Activities
- [ ] Lesson 3: Workers & Task Queues
- [ ] Lesson 4: Running Your First Workflow
- [ ] Lesson 5: Error Handling & Retries
- [ ] Lesson 6: Signals & Queries
- [ ] Lesson 7: Order Processing Workflow
- [ ] Lesson 8: Testing & Best Practices
- [ ] Lesson 9: Production Deployment

---

## 🎓 What You'll Build

By the end of this course, you'll have:

### Complete Order Processing Workflow
- ✅ Inventory reservation with compensation
- ✅ Payment processing with intelligent retries
- ✅ Email notifications (async)
- ✅ Human approval with timeouts
- ✅ Cancellation handling
- ✅ Full observability and history

### Production Skills
- ✅ Error handling strategies
- ✅ Testing workflows in isolation
- ✅ Deploying to production
- ✅ Monitoring and alerting
- ✅ Security best practices

---

## 🔗 Quick Links

- [Getting Started Guide](README.md)
- [Lesson 1: What is Temporal?](lesson_1.md)
- [Docker Compose Setup](docker-compose.yml)
- [Temporal Documentation](https://docs.temporal.io/)
- [Go SDK Reference](https://pkg.go.dev/go.temporal.io/sdk)

---

**Ready to master Temporal?** Begin with **[Lesson 1](lesson_1.md)** now! 🚀

---

_Last Updated: November 2025_

