# Lesson 2: Workflows & Activities

## Learning Objectives

By the end of this lesson, you will:
- ✅ Write your first workflow in Go
- ✅ Create activities that perform real work
- ✅ Understand determinism and why it matters
- ✅ Learn workflow patterns (sequential, parallel)
- ✅ Configure activity options and timeouts

---

## Why Before How: Understanding Determinism

### The Core Challenge

Remember from Lesson 1: workflows are **deterministic** and **durable**.

**Question:** How does Temporal make workflows survive crashes?

**Answer:** Event Sourcing + Replay

When a workflow executes:
1. Every decision point is recorded as an event (e.g., "activity scheduled", "activity completed")
2. Events are stored durably in the database
3. If the worker crashes, Temporal replays the entire workflow from the event history
4. The workflow code re-executes, but activities don't re-run (results come from history)

### Why Determinism Matters

**Replay requires determinism.** Every time the workflow code runs, it must make the same decisions in the same order.

**❌ Non-deterministic (breaks replay):**
```go
// BAD: Random values change on replay
orderID := generateRandomID()

// BAD: Current time changes on replay
if time.Now().Hour() > 17 {
    // After-hours processing
}

// BAD: External calls change on replay
user := database.GetUser(userID)
```

**✅ Deterministic (safe for replay):**
```go
// GOOD: Use workflow-provided time
currentTime := workflow.Now(ctx)

// GOOD: Execute side effects in activities
var user User
err := workflow.ExecuteActivity(ctx, GetUserActivity, userID).Get(ctx, &user)

// GOOD: Pure logic based on inputs
if order.Total > 1000 {
    // High-value order processing
}
```

**The Rule:** Workflows orchestrate; activities execute.

---

## Your First Workflow: GreetUser

Let's start simple. We'll create a workflow that greets a user by:
1. Fetching user details (activity)
2. Formatting a greeting message (workflow logic)
3. Sending the greeting (activity)

### Step 1: Define the Workflow Interface

Create `workflows/greet.go`:

```go
package workflows

import (
    "time"
    
    "go.temporal.io/sdk/workflow"
)

// GreetUserInput is the input to the GreetUser workflow
type GreetUserInput struct {
    UserID string
}

// GreetUserResult is the output of the GreetUser workflow
type GreetUserResult struct {
    Message   string
    SentAt    time.Time
    Success   bool
}

// GreetUser is a workflow that greets a user
// Workflows must:
// - Accept workflow.Context as first parameter
// - Return error (or value + error)
// - Be deterministic (no random numbers, no time.Now(), no external calls)
func GreetUser(ctx workflow.Context, input GreetUserInput) (*GreetUserResult, error) {
    // Workflow execution logic will go here
    return nil, nil
}
```

**Key Points:**
- Workflows accept `workflow.Context` (not `context.Context`!)
- Input/output are strongly typed structs
- Clear, descriptive names

---

### Step 2: Create Activities

Activities do the actual work. Create `activities/greet.go`:

```go
package activities

import (
    "context"
    "fmt"
    "time"
)

// UserDetails represents user information
type UserDetails struct {
    UserID    string
    FirstName string
    LastName  string
    Email     string
}

// GreetActivities contains all greeting-related activities
type GreetActivities struct {
    // In a real app, you'd inject dependencies here:
    // db *sql.DB
    // emailClient *EmailClient
}

// GetUserDetails fetches user information
// Activities must:
// - Accept context.Context as first parameter (standard Go context!)
// - Be idempotent (safe to retry)
// - Return error on failure
func (a *GreetActivities) GetUserDetails(ctx context.Context, userID string) (*UserDetails, error) {
    // Simulate database lookup
    // In a real app: user, err := a.db.Query(...)
    
    // Activities CAN do non-deterministic things:
    // - Database queries
    // - HTTP calls
    // - File I/O
    // - Random numbers
    // - time.Now()
    
    if userID == "" {
        return nil, fmt.Errorf("userID cannot be empty")
    }
    
    // Simulate fetching from database
    return &UserDetails{
        UserID:    userID,
        FirstName: "Alice",
        LastName:  "Johnson",
        Email:     "alice@example.com",
    }, nil
}

// SendGreeting sends a greeting message
func (a *GreetActivities) SendGreeting(ctx context.Context, email string, message string) error {
    // Simulate sending email
    // In a real app: return a.emailClient.Send(email, message)
    
    fmt.Printf("📧 Sending email to %s: %s\n", email, message)
    
    // Simulate network delay
    time.Sleep(100 * time.Millisecond)
    
    return nil
}

// LogGreeting logs the greeting action
func (a *GreetActivities) LogGreeting(ctx context.Context, userID string, message string) error {
    // Activities can write to databases, logs, etc.
    fmt.Printf("📝 Log: User %s greeted with: %s\n", userID, message)
    return nil
}
```

**Key Points:**
- Activities use standard `context.Context`
- They can do **anything** (DB, HTTP, files, random, time.Now())
- Group related activities in a struct (for dependency injection)
- Always design for **idempotency** (same input → same output, safe to retry)

---

### Step 3: Implement the Workflow Logic

Now let's wire everything together in `workflows/greet.go`:

```go
package workflows

import (
    "fmt"
    "time"
    
    "go.temporal.io/sdk/workflow"
    
    "go-temporal-fast-course/activities"
)

// GreetUserInput is the input to the GreetUser workflow
type GreetUserInput struct {
    UserID string
}

// GreetUserResult is the output of the GreetUser workflow
type GreetUserResult struct {
    Message   string
    SentAt    time.Time
    Success   bool
}

// GreetUser is a workflow that greets a user
func GreetUser(ctx workflow.Context, input GreetUserInput) (*GreetUserResult, error) {
    // Configure activity options (timeouts, retries)
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 10 * time.Second, // Activity must complete within 10s
        RetryPolicy: &workflow.RetryPolicy{
            InitialInterval:    1 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    10 * time.Second,
            MaximumAttempts:    3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)
    
    // Get workflow logger (safe for workflows, automatically includes workflow ID, run ID)
    logger := workflow.GetLogger(ctx)
    logger.Info("GreetUser workflow started", "userID", input.UserID)
    
    // Step 1: Get user details (execute activity)
    var userDetails *activities.UserDetails
    err := workflow.ExecuteActivity(ctx, "GetUserDetails", input.UserID).Get(ctx, &userDetails)
    if err != nil {
        logger.Error("Failed to get user details", "error", err)
        return nil, fmt.Errorf("failed to get user details: %w", err)
    }
    
    logger.Info("User details retrieved", "name", userDetails.FirstName)
    
    // Step 2: Format greeting message (pure workflow logic, deterministic)
    // Use workflow.Now() instead of time.Now() for deterministic time
    currentTime := workflow.Now(ctx)
    hour := currentTime.Hour()
    
    var greeting string
    if hour < 12 {
        greeting = "Good morning"
    } else if hour < 18 {
        greeting = "Good afternoon"
    } else {
        greeting = "Good evening"
    }
    
    message := fmt.Sprintf("%s, %s %s! Welcome to our e-commerce store.", 
        greeting, userDetails.FirstName, userDetails.LastName)
    
    // Step 3: Send greeting (execute activity)
    err = workflow.ExecuteActivity(ctx, "SendGreeting", userDetails.Email, message).Get(ctx, nil)
    if err != nil {
        logger.Error("Failed to send greeting", "error", err)
        return nil, fmt.Errorf("failed to send greeting: %w", err)
    }
    
    // Step 4: Log the action (execute activity)
    err = workflow.ExecuteActivity(ctx, "LogGreeting", input.UserID, message).Get(ctx, nil)
    if err != nil {
        // Logging failure is not critical, just log and continue
        logger.Warn("Failed to log greeting", "error", err)
    }
    
    logger.Info("GreetUser workflow completed successfully")
    
    return &GreetUserResult{
        Message: message,
        SentAt:  currentTime,
        Success: true,
    }, nil
}
```

**Key Patterns:**

1. **Activity Options** - Configure timeouts and retries upfront
2. **workflow.ExecuteActivity** - Schedule activity execution
3. **`.Get(ctx, &result)`** - Wait for activity to complete and retrieve result
4. **workflow.Now(ctx)** - Get deterministic current time
5. **workflow.GetLogger(ctx)** - Get workflow-aware logger
6. **Error Handling** - Activities can fail; workflows decide how to handle it

---

## Understanding Activity Execution

When you call `workflow.ExecuteActivity()`:

```go
err := workflow.ExecuteActivity(ctx, "GetUserDetails", userID).Get(ctx, &userDetails)
```

**What happens:**

1. Workflow sends "schedule activity" command to Temporal
2. Temporal records event: `ActivityTaskScheduled`
3. Temporal puts task on task queue
4. Worker polls task queue, receives activity task
5. Worker executes activity code (`GetUserDetails`)
6. Activity returns result (or error)
7. Worker sends result back to Temporal
8. Temporal records event: `ActivityTaskCompleted` (or `ActivityTaskFailed`)
9. Workflow resumes, `.Get()` returns the result

**On replay after crash:**
- Steps 1-9 already happened and are in history
- Workflow code re-runs, but activity doesn't re-execute
- `.Get()` returns the result from history instantly

---

## Activity Options Explained

```go
workflow.ActivityOptions{
    // How long can the activity run before timing out?
    StartToCloseTimeout: 10 * time.Second,
    
    // How long can the activity wait in queue before starting?
    ScheduleToStartTimeout: 5 * time.Second,
    
    // How long from schedule to completion (queue + execution)?
    ScheduleToCloseTimeout: 15 * time.Second,
    
    // Heartbeat timeout (for long-running activities)
    HeartbeatTimeout: 2 * time.Second,
    
    // Retry policy
    RetryPolicy: &workflow.RetryPolicy{
        InitialInterval:    1 * time.Second,  // Wait 1s before first retry
        BackoffCoefficient: 2.0,               // Double wait time each retry (1s, 2s, 4s, 8s)
        MaximumInterval:    30 * time.Second,  // Cap wait time at 30s
        MaximumAttempts:    5,                 // Give up after 5 attempts
        NonRetryableErrorTypes: []string{"ValidationError"}, // Don't retry these
    },
}
```

**Best Practices:**
- Set appropriate timeouts (not too short, not too long)
- Use exponential backoff for retries
- Don't retry validation errors (permanent failures)
- Use heartbeats for long-running activities (we'll cover this in Lesson 5)

---

## Workflow Patterns

### Pattern 1: Sequential Execution (what we just did)

Activities execute one after another:

```go
// Step 1
var result1 Type1
err := workflow.ExecuteActivity(ctx, Activity1, input1).Get(ctx, &result1)

// Step 2 (uses result from step 1)
var result2 Type2
err = workflow.ExecuteActivity(ctx, Activity2, result1).Get(ctx, &result2)

// Step 3
var result3 Type3
err = workflow.ExecuteActivity(ctx, Activity3, result2).Get(ctx, &result3)
```

**Use when:** Each step depends on the previous step's result.

---

### Pattern 2: Parallel Execution

Activities execute concurrently:

```go
// Start all activities (don't wait with .Get() yet)
future1 := workflow.ExecuteActivity(ctx, Activity1, input1)
future2 := workflow.ExecuteActivity(ctx, Activity2, input2)
future3 := workflow.ExecuteActivity(ctx, Activity3, input3)

// Now wait for all to complete
var result1 Type1
var result2 Type2
var result3 Type3

err1 := future1.Get(ctx, &result1)
err2 := future2.Get(ctx, &result2)
err3 := future3.Get(ctx, &result3)

// Handle errors
if err1 != nil || err2 != nil || err3 != nil {
    return nil, fmt.Errorf("one or more activities failed")
}
```

**Use when:** Activities are independent and can run concurrently.

**Example for your e-commerce store:** 
- Fetch user details
- Fetch recommended products
- Fetch recent orders
All can happen in parallel!

---

### Pattern 3: Parallel with workflow.Go (Advanced)

For true parallelism with complex logic:

```go
var result1 Type1
var result2 Type2

// Create a wait group
var wg sync.WaitGroup
wg.Add(2)

// Goroutine 1
workflow.Go(ctx, func(ctx workflow.Context) {
    defer wg.Done()
    workflow.ExecuteActivity(ctx, Activity1, input1).Get(ctx, &result1)
})

// Goroutine 2
workflow.Go(ctx, func(ctx workflow.Context) {
    defer wg.Done()
    workflow.ExecuteActivity(ctx, Activity2, input2).Get(ctx, &result2)
})

// Wait for both
wg.Wait()
```

**Note:** Use `workflow.Go`, not regular `go`! (Determinism!)

---

## Determinism Rules Summary

| ✅ Safe in Workflows | ❌ Unsafe in Workflows |
|---------------------|----------------------|
| `workflow.ExecuteActivity()` | `time.Now()` |
| `workflow.Now(ctx)` | `rand.Int()` |
| `workflow.Sleep(ctx, duration)` | `time.Sleep()` |
| `workflow.GetLogger(ctx)` | Database calls |
| Pure functions with inputs | HTTP calls |
| Conditionals on inputs | File I/O |
| Loops with deterministic bounds | `go func()` (use `workflow.Go` instead) |
| `workflow.GetVersion()` for versioning | Non-deterministic external calls |

**Golden Rule:** If it has side effects or non-deterministic behavior, put it in an activity!

---

## 🎯 Exercise: Extend the GreetUser Workflow

Try modifying the workflow to:

1. **Add a new activity** `GetUserPreferences(userID)` that returns user's preferred language
2. **Modify the greeting** to use the user's language (e.g., "Buenos días" for Spanish)
3. **Execute activities in parallel**: Fetch user details and preferences concurrently

<details>
<summary>💡 Hint (click to expand)</summary>

```go
// Start both activities in parallel
futureDetails := workflow.ExecuteActivity(ctx, "GetUserDetails", input.UserID)
futurePrefs := workflow.ExecuteActivity(ctx, "GetUserPreferences", input.UserID)

// Wait for both
var userDetails *activities.UserDetails
var prefs *activities.UserPreferences
err1 := futureDetails.Get(ctx, &userDetails)
err2 := futurePrefs.Get(ctx, &prefs)

// Then format greeting based on language
var greeting string
switch prefs.Language {
case "es":
    greeting = "Buenos días"
case "fr":
    greeting = "Bonjour"
default:
    greeting = "Good morning"
}
```
</details>

---

## What You've Learned

✅ How to write workflows in Go  
✅ How to create and execute activities  
✅ Why determinism matters (replay mechanism)  
✅ Activity options (timeouts, retries)  
✅ Workflow patterns (sequential, parallel)  
✅ The golden rule: workflows orchestrate, activities execute  
✅ What's safe (and unsafe) in workflows  

---

## Next Steps

In **[Lesson 3: Workers & Task Queues](lesson_3.md)**, we'll:
- Create a worker that runs your workflows
- Understand task queue routing
- Register workflows and activities
- Configure worker options
- Run multiple workers for scaling

But before that, we need to actually **run the code we just wrote**! 

---

## 🚀 Quick Preview: Running Your Workflow

In Lesson 4, we'll do this properly, but here's a sneak peek:

**1. Start Temporal server:**
```bash
docker-compose up
```

**2. Run worker:**
```bash
go run worker/main.go
```

**3. Start workflow:**
```bash
go run starter/main.go
```

**4. Check Temporal UI:**
```
http://localhost:8080
```

Don't worry if this doesn't make sense yet - we'll build this step-by-step in Lessons 3 & 4!

---

## 📝 Key Takeaways

Before moving to Lesson 3, make sure you understand:

1. **What makes workflows deterministic?** (No time.Now(), no external calls, same inputs → same decisions)
2. **What's the difference between workflow.Context and context.Context?** (Workflow context is special, tracked by Temporal)
3. **When do activities execute?** (When worker polls task and runs them)
4. **How does retry work?** (Temporal automatically retries based on RetryPolicy)
5. **Why use workflow.Now() instead of time.Now()?** (Deterministic replay)

---

## 🆘 Questions?

Common questions at this stage:

**Q: Can I call one activity from another activity?**  
A: No! Activities are independent units. If you need orchestration, use a workflow.

**Q: Can I call a workflow from another workflow?**  
A: Yes! Use `workflow.ExecuteChildWorkflow()`. We'll cover this in Lesson 6.

**Q: What if my activity takes 10 minutes?**  
A: Set appropriate timeouts and use heartbeats (Lesson 5).

**Q: Can I pass complex objects between workflows and activities?**  
A: Yes, but they must be serializable (JSON-compatible). Avoid passing functions or channels.

---

**Ready for Lesson 3?** When you're ready to learn about workers and task queues, say:

**"I'm ready for Lesson 3"**

Or if you have questions about anything in Lesson 2, ask away! 🎓

[← Back to Course Index](course.md) | [← Previous: Lesson 1](lesson_1.md)

