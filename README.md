# 🎓 Temporal Fast Course for Go Developers (~4hours)

Welcome to your interactive Temporal learning experience! This course is designed to teach you Temporal workflows, activities, and best practices through hands-on lessons in Go.

---

## 📚 What is This Course?

This is a **progressive, lesson-based course** that teaches you how to build durable, reliable workflows using Temporal. Each lesson builds on the previous one, starting from fundamentals and progressing to advanced topics.

**What makes this course special:**
- 📖 **Theory + Practice**: Each lesson explains "why before how" with Go code examples
- 🎯 **Goal-Oriented**: Focused on building an order processing workflow
- 📝 **Self-Paced**: Progress at your own speed with exercises and checkpoints
- 🎓 **Flexible Learning**: Two training modes to suit your learning style

---

## 🎓 Two Ways to Learn

This course supports **two different learning modes** depending on your preference:

### 📖 Mode 1: Self-Study (Read at Your Own Pace)

**Best for:** Independent learners who prefer reading documentation and experimenting on their own.

**How it works:**
1. All 9 lessons are already created in the repository
2. Open any lesson file (e.g., `lesson_1.md`, `lesson_2.md`, etc.)
3. Read through the content at your own pace
4. Follow the code examples and run them yourself
5. Complete the exercises independently
6. Move to the next lesson when ready

**Pros:**
- ✅ Complete freedom to skip around
- ✅ No need for AI interaction
- ✅ Can review lessons anytime offline
- ✅ Perfect for quick reference

**Getting started:**
```bash
# Just open the first lesson
open lesson_1.md
# Or start from the course overview
open course.md
```

---

### 🤖 Mode 2: AI-Guided Learning (Interactive Instructor)

**Best for:** Learners who want personalized guidance, real-time explanations, and interactive help.

**How it works:**
1. Use the `temporal_teacher` AI agent in your IDE chat
2. The agent guides you through each lesson interactively
3. Ask questions anytime and get immediate, contextual answers
4. The agent can create code examples, run tests, and validate your work
5. Get personalized explanations tailored to your understanding

**Pros:**
- ✅ Interactive Q&A during learning
- ✅ Personalized explanations and analogies
- ✅ Real-time code assistance
- ✅ Validates your understanding with checkpoints
- ✅ Adapts to your learning pace

**Getting started:**

**Step 1:** Open the Temporal Teacher Mode  
In your IDE chat, make sure you're in **temporal_teacher** mode. This custom agent is configured to guide you through the course.

**Step 2:** Start the Course  
Simply type in the chat:
```
Start the course
```

The AI instructor will:
- Open Lesson 1 for you
- Provide an overview of what you'll learn
- Guide you through each concept
- Answer your questions in real-time

**Step 3:** Progress Through Lessons  
When you finish a lesson and are ready to continue, type:
```
I'm ready for Lesson [number]
```

Example: `I'm ready for Lesson 2`

The instructor will open the next lesson and provide context.

---

## 📚 Course Structure

| Lesson | Topic | Duration | Status |
|--------|-------|----------|--------|
| [Lesson 1](lesson_1.md) | What is Temporal? | 15 min | ✅ |
| [Lesson 2](lesson_2.md) | Workflows & Activities | 30 min | ✅ |
| [Lesson 3](lesson_3.md) | Workers & Task Queues | 20 min | ✅ |
| [Lesson 4](lesson_4.md) | Running Your First Workflow | 30 min | ✅ |
| [Lesson 5](lesson_5.md) | Error Handling & Retries | 25 min | ✅ |
| [Lesson 6](lesson_6.md) | Signals & Queries | 25 min | ✅ |
| [Lesson 7](lesson_7.md) | Full Order Processing (Integration) | 45 min | ✅ |
| [Lesson 8](lesson_8.md) | Testing & Best Practices | 30 min | ✅ |
| [Lesson 9](lesson_9.md) | Production Deployment | 30 min | ✅ |

**Total Time:** ~4 hours

---

## 🚀 Complete Order Processing Implementation

This repository includes a **fully implemented order processing system** in the `order-processing/` directory that brings together all concepts from Lessons 2-7.

## 🎯 What You'll Build

By the end of this course, you'll have built a **complete order processing workflow** that:

1. **Reserves inventory** for an order
2. **Processes payment** with retry logic
3. **Sends confirmation emails** asynchronously
4. **Handles failures gracefully** with compensation logic
5. **Integrates with your existing backend** (auth, book services)

### Features Implemented:

- ✅ **Full Order Workflow** with parallel enrichment
- ✅ **Signal Handlers**: approve-payment, cancel-order, add-line-item
- ✅ **Query Handlers**: get-status, get-items
- ✅ **Saga Pattern**: Compensation for failed transactions
- ✅ **Retry Policies**: Typed errors with smart retries
- ✅ **Workflow Versioning**: Safe evolution with GetVersion
- ✅ **Real Activities**: Inventory, Payment, Notifications, etc.

---

## 🤖 About the Temporal Teacher Agent (Mode 2)

The `temporal_teacher` mode is a custom AI instructor specifically designed for **interactive learning** (Mode 2). It:

- **Guides through lessons**: Opens relevant files and provides context
- **Answers questions**: Ask anything about Temporal, Go, or the project
- **Provides examples**: Creates custom code examples based on your questions
- **Writes code with you**: Creates and edits files in your project
- **Validates your work**: Checks for errors and best practices
- **Remembers context**: Tracks your progress through the course

### When to Use the Agent:
- ❓ You're confused about a concept
- 💻 You need help implementing an exercise
- 🐛 Your code isn't working and you need debugging help
- 📚 You want a different explanation or analogy
- ✅ You want to validate your understanding

### Agent Configuration
The agent is configured in: `.github/agents/temporal_teacher.agent.md`

---

## 🆘 Need Help?

**Lost Context?**  
If you feel like the AI is losing track of where you are in the course, simply ask:
```
Create a compressed summary of my progress
```

The instructor will create `compressed.md` with your current state.

**Stuck on a Concept?**  
Ask specific questions:
```
Can you explain the difference between workflows and activities again?
```

```
Why do workflows need to be deterministic?
```

```
Show me another example of error handling in activities
```

**Want to Review?**  
Ask to open any previous lesson:
```
Can you open Lesson 3 again?
```
---
