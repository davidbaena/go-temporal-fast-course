# Order Processing - Temporal Workflow Implementation

This directory contains a complete implementation of order processing workflows using Temporal, integrating concepts from Lessons 2-7 of the course.

## 📁 Project Structure

```
order-processing/
├── activities/              # Activity implementations
│   ├── order_activities.go  # Order-related activities
│   └── greet_activities.go  # Simple greeting example
├── workflows/               # Workflow definitions
│   ├── order_workflow.go    # Complete order processing workflow
│   └── greet_workflow.go    # Simple greeting workflow
├── types/                   # Shared types and errors
│   ├── types.go            # Domain types and DTOs
│   └── errors.go           # Custom error types
├── worker/                  # Worker process
│   └── main.go             # Worker main entry point
├── starter/                 # Workflow starter/client
│   └── main.go             # Client to start workflows
└── README.md               # This file
```

## 🎯 What's Implemented

### OrderWorkflow Features

From the course lessons, this implementation includes:

- **Lesson 2**: Workflows & Activities
  - Sequential and parallel activity execution
  - Deterministic workflow logic
  - Activity retry policies

- **Lesson 3**: Workers & Task Queues
  - Worker configuration with identity
  - Activity and workflow registration
  - Task queue routing

- **Lesson 5**: Error Handling & Retries
  - Typed errors (PermanentError, ValidationError)
  - Retry policies with exponential backoff
  - Saga pattern for compensation (refunds, stock release)

- **Lesson 6**: Signals & Queries
  - Signals: `approve-payment`, `cancel-order`, `add-line-item`
  - Queries: `get-status`, `get-items`
  - Timeout handling with selectors

- **Lesson 7**: Production Patterns
  - Workflow versioning with `GetVersion`
  - Parallel enrichment activities
  - Comprehensive error handling
  - Observability with structured logging

### Activities Implemented

**Inventory Activities:**
- `ReserveStock` - Reserve inventory for an order
- `ReleaseStock` - Release reserved inventory (compensation)
- `FetchInventorySnapshot` - Check inventory availability

**Payment Activities:**
- `ProcessPayment` - Process payment with failure simulation
- `RefundPayment` - Refund payment (compensation)

**Customer Activities:**
- `FetchCustomerProfile` - Fetch customer tier information

**Recommendation Activities:**
- `FetchRecommendations` - Fetch product recommendations

**Order Activities:**
- `UpdateOrderStatus` - Update order status in database

**Notification Activities:**
- `SendOrderConfirmation` - Send order confirmation email
- `SendCancellationEmail` - Send cancellation notification

## 🚀 Quick Start

### Prerequisites

1. **Start Temporal server** (from project root):
   ```bash
   docker-compose up -d
   ```

2. **Verify Temporal is running**:
   ```bash
   docker ps
   # Should show temporal, postgresql, and temporal-ui containers
   ```

3. **Access Temporal UI**:
   ```
   http://localhost:8080
   ```

### Running the Order Workflow

#### 1. Start the Worker

In one terminal:
```bash
cd order-processing
go run worker/main.go
```

Expected output:
```
Worker starting on task queue: order-task-queue
Worker identity: order-worker-<hostname>
```

#### 2. Start an Order Workflow

In another terminal:

**Option A: With manual approval (interactive)**
```bash
cd order-processing
ASYNC=true go run starter/main.go
```

This starts the workflow and waits for you to send signals.

**Option B: With auto-approval (automated)**
```bash
cd order-processing
AUTO_APPROVE=true go run starter/main.go
```

This automatically approves the payment after 2 seconds.

### Running the Greet Workflow (Simple Example)

```bash
# Start worker (if not already running)
go run worker/main.go

# In another terminal, start greet workflow
WORKFLOW_TYPE=greet go run starter/main.go
```

## 🎮 Interacting with Workflows

### Using Signals

**Approve Payment:**
```bash
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<timestamp> \
  -n approve-payment \
  -i '{"ApprovedBy":"admin"}'
```

**Cancel Order:**
```bash
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<timestamp> \
  -n cancel-order \
  -i '{"Reason":"customer requested"}'
```

**Add Line Item:**
```bash
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<timestamp> \
  -n add-line-item \
  -i '{"SKU":"ITEM-999","Quantity":3}'
```

### Using Queries

**Get Order Status:**
```bash
docker exec -it temporal-admin-tools tctl workflow query \
  -w order-workflow-ORDER-<timestamp> \
  -qt get-status
```

**Get Order Items:**
```bash
docker exec -it temporal-admin-tools tctl workflow query \
  -w order-workflow-ORDER-<timestamp> \
  -qt get-items
```

## 🔧 Configuration

Configure via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `ORDER_TASK_QUEUE` | `order-task-queue` | Task queue name |
| `WORKFLOW_TYPE` | `order` | Workflow to run (`order` or `greet`) |
| `ORDER_ID` | `ORDER-<timestamp>` | Order identifier |
| `USER_ID` | `user-123` | User ID for greet workflow |
| `ASYNC` | `false` | Start workflow without waiting |
| `AUTO_APPROVE` | `false` | Auto-approve payment after 2s |

Example:
```bash
TEMPORAL_HOST=temporal.example.com:7233 \
ORDER_TASK_QUEUE=production-orders \
AUTO_APPROVE=true \
go run starter/main.go
```

## 📊 Order Workflow Flow

```
OrderWorkflow
 ├─ 1. Parallel Enrichment (v2)
 │   ├─ FetchCustomerProfile
 │   ├─ FetchInventorySnapshot
 │   └─ FetchRecommendations
 │
 ├─ 2. ReserveStock
 │
 ├─ 3. Await Approval (with signals)
 │   ├─ approve-payment → Continue
 │   ├─ cancel-order → Compensate & Exit
 │   ├─ add-line-item → Update items
 │   └─ timeout (15min) → Cancel
 │
 ├─ 4. ProcessPayment (with retries)
 │
 ├─ 5. UpdateOrderStatus
 │
 └─ 6. SendOrderConfirmation (best-effort)
```

### Compensation (Saga Pattern)

If any step fails after stock reservation:
- **After Reserve**: Release stock
- **After Payment**: Refund payment + Release stock
- **On Cancel**: Release stock + Send cancellation email

## 🧪 Testing the Workflow

### Test Scenarios

**Scenario 1: Successful Order**
```bash
# Start workflow with auto-approve
AUTO_APPROVE=true go run starter/main.go
```
Expected: Order completes successfully

**Scenario 2: Payment Failure**
```bash
# Run multiple times - payment fails ~20% of the time
# Observe automatic retries in UI
go run starter/main.go
```

**Scenario 3: Order Cancellation**
```bash
# Start async
ASYNC=true go run starter/main.go

# Cancel immediately
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<id> \
  -n cancel-order \
  -i '{"Reason":"test cancellation"}'
```

**Scenario 4: Approval Timeout**
```bash
# Start async and don't approve (wait 15 minutes)
ASYNC=true go run starter/main.go
# Workflow will auto-cancel after 15 minutes
```

**Scenario 5: Dynamic Items**
```bash
# Start async
ASYNC=true go run starter/main.go

# Add items before approving
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<id> \
  -n add-line-item \
  -i '{"SKU":"EXTRA-001","Quantity":1}'

# Then approve
docker exec -it temporal-admin-tools tctl workflow signal \
  -w order-workflow-ORDER-<id> \
  -n approve-payment \
  -i '{"ApprovedBy":"admin"}'
```

## 🔍 Observability

### Viewing Workflow History

1. **Temporal UI**: http://localhost:8080
   - Navigate to Workflows
   - Click on your workflow ID
   - View complete event history

2. **Using tctl**:
   ```bash
   docker exec -it temporal-admin-tools tctl workflow show \
     -w order-workflow-ORDER-<id>
   ```

### Logs

The worker outputs structured logs showing:
- Activity execution start/complete
- Workflow progress through stages
- Error details and retry attempts

## 🎓 Lesson Integration

This implementation demonstrates concepts from:

- ✅ **Lesson 2**: Workflows & Activities
- ✅ **Lesson 3**: Workers & Task Queues  
- ✅ **Lesson 5**: Error Handling & Retries
- ✅ **Lesson 6**: Signals & Queries
- ✅ **Lesson 7**: Production Patterns

## 📝 Next Steps

After running this implementation:

1. **Lesson 8**: Testing & Best Practices
   - Unit test workflows with test environment
   - Mock activities for testing
   - Workflow versioning evolution

2. **Lesson 9**: Production Deployment
   - Deploy to Kubernetes
   - Configure observability
   - Setup monitoring and alerts

## 🐛 Troubleshooting

**Worker can't connect:**
```bash
# Check Temporal is running
docker ps | grep temporal

# Check connectivity
docker exec temporal-admin-tools tctl namespace list
```

**Workflow stuck in approval:**
- Send approval signal manually
- Check signal name matches exactly: `approve-payment`
- Verify workflow ID is correct

**Activities failing:**
- Check worker logs for detailed error messages
- Review retry policy configuration
- Some failures are intentional for testing (payment fails ~20%)

## 📚 Related Files

- Course overview: `../course.md`
- Lesson 2: `../lesson_2.md` (Workflows & Activities)
- Lesson 3: `../lesson_3.md` (Workers)
- Lesson 5: `../lesson_5.md` (Error Handling)
- Lesson 6: `../lesson_6.md` (Signals & Queries)
- Lesson 7: `../lesson_7.md` (Order Workflow)
- Docker setup: `../docker-compose.yml`
