package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go-temporal-fast-course/order-processing/types"
)

// OrderWorkflow implements a complete order processing workflow with:
// - Parallel enrichment activities
// - Signal handlers (approve, cancel, add item)
// - Query handlers (status, items)
// - Saga pattern compensation
// - Workflow versioning
// This integrates concepts from Lessons 2-7
func OrderWorkflow(ctx workflow.Context, orderID string, initialItems []types.LineItem) (string, error) {
	logger := workflow.GetLogger(ctx)

	// Workflow versioning (Lesson 7)
	version := workflow.GetVersion(ctx, "order-workflow-v2", workflow.DefaultVersion, 2)

	status := types.OrderWorkflowStatus{
		OrderID: orderID,
		Stage:   "start",
		Items:   initialItems,
		Version: fmt.Sprintf("v%d", version),
	}

	// Configure activity options with retry policy (Lesson 5)
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:        1 * time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        30 * time.Second,
		MaximumAttempts:        5,
		NonRetryableErrorTypes: []string{"PermanentError", "ValidationError"},
	}

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retryPolicy,
		HeartbeatTimeout:    15 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Register query handlers (Lesson 6)
	err := workflow.SetQueryHandler(ctx, "get-status", func() (types.OrderWorkflowStatus, error) {
		return status, nil
	})
	if err != nil {
		return "", err
	}

	err = workflow.SetQueryHandler(ctx, "get-items", func() ([]types.LineItem, error) {
		return status.Items, nil
	})
	if err != nil {
		return "", err
	}

	// Setup signal channels (Lesson 6)
	sigApprove := workflow.GetSignalChannel(ctx, "approve-payment")
	sigCancel := workflow.GetSignalChannel(ctx, "cancel-order")
	sigAddItem := workflow.GetSignalChannel(ctx, "add-line-item")

	// Step 1: Enrichment - parallel or sequential based on version (Lesson 7)
	status.Stage = "enrichment"
	if version == workflow.DefaultVersion {
		// Sequential enrichment (backward compatibility)
		var invOk bool
		err := workflow.ExecuteActivity(ctx, "FetchInventorySnapshot", status.Items).Get(ctx, &invOk)
		if err != nil {
			return "", err
		}
		status.Enrichment.InventoryOk = invOk
	} else {
		// Parallel enrichment (new version)
		fInventory := workflow.ExecuteActivity(ctx, "FetchInventorySnapshot", status.Items)
		fCustomer := workflow.ExecuteActivity(ctx, "FetchCustomerProfile", orderID)
		fRecs := workflow.ExecuteActivity(ctx, "FetchRecommendations", orderID)

		var invOk bool
		var customerTier string
		var recs []string

		if err := fInventory.Get(ctx, &invOk); err != nil {
			return "", err
		}
		if err := fCustomer.Get(ctx, &customerTier); err != nil {
			return "", err
		}
		if err := fRecs.Get(ctx, &recs); err != nil {
			return "", err
		}

		status.Enrichment.InventoryOk = invOk
		status.Enrichment.CustomerTier = customerTier
		status.Enrichment.Recommendations = recs
	}

	if !status.Enrichment.InventoryOk {
		logger.Warn("Inventory check failed", "orderID", orderID)
		status.LastError = "insufficient inventory"
		return "", fmt.Errorf("insufficient inventory for order %s", orderID)
	}

	// Step 2: Reserve Stock (Lesson 5)
	status.Stage = "reserve"
	err = workflow.ExecuteActivity(ctx, "ReserveStock", orderID, status.Items).Get(ctx, nil)
	if err != nil {
		status.LastError = fmt.Sprintf("reserve failed: %v", err)
		return "", err
	}
	status.Reserved = true
	logger.Info("Stock reserved", "orderID", orderID)

	// Step 3: Await Approval with timeout (Lesson 6)
	status.Stage = "awaiting-approval"
	approvalTimeout := workflow.Now(ctx).Add(15 * time.Minute)
	status.ApprovalDeadline = approvalTimeout

	for !status.PaymentApproved && !status.Cancelled {
		selector := workflow.NewSelector(ctx)
		timerFut := workflow.NewTimer(ctx, time.Until(approvalTimeout))

		selector.AddReceive(sigApprove, func(ch workflow.ReceiveChannel, more bool) {
			var payload types.PaymentApproval
			ch.Receive(ctx, &payload)
			status.PaymentApproved = true
			logger.Info("Approval received", "by", payload.ApprovedBy)
		})

		selector.AddReceive(sigCancel, func(ch workflow.ReceiveChannel, more bool) {
			var payload types.CancelRequest
			ch.Receive(ctx, &payload)
			status.Cancelled = true
			status.LastError = fmt.Sprintf("cancelled: %s", payload.Reason)
			logger.Info("Cancellation received", "reason", payload.Reason)
		})

		selector.AddReceive(sigAddItem, func(ch workflow.ReceiveChannel, more bool) {
			var item types.LineItem
			ch.Receive(ctx, &item)
			status.Items = append(status.Items, item)
			logger.Info("Item added", "sku", item.SKU, "qty", item.Quantity)
		})

		selector.AddFuture(timerFut, func(f workflow.Future) {
			status.Cancelled = true
			status.LastError = "approval timeout"
			logger.Warn("Approval timed out")
		})

		selector.Select(ctx)
	}

	if status.Cancelled {
		// Compensation - release stock (Lesson 5: Saga pattern)
		_ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, "SendCancellationEmail", orderID, status.LastError).Get(ctx, nil)
		status.Stage = "cancelled"
		return fmt.Sprintf("Order %s cancelled (%s)", orderID, status.LastError), nil
	}

	// Step 4: Process Payment with typed errors (Lesson 5)
	status.Stage = "payment"
	err = workflow.ExecuteActivity(ctx, "ProcessPayment", orderID).Get(ctx, nil)
	if err != nil {
		status.LastError = fmt.Sprintf("payment failed: %v", err)
		logger.Error("Payment failed", "error", err)
		// Compensation - release stock
		_ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
		return "", err
	}
	status.Charged = true
	logger.Info("Payment processed", "orderID", orderID)

	// Step 5: Update Order Status
	status.Stage = "status-update"
	err = workflow.ExecuteActivity(ctx, "UpdateOrderStatus", orderID, "COMPLETED").Get(ctx, nil)
	if err != nil {
		status.LastError = fmt.Sprintf("status update failed: %v", err)
		logger.Error("Status update failed", "error", err)
		// Compensation - refund and release
		_ = workflow.ExecuteActivity(ctx, "RefundPayment", orderID).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, "ReleaseStock", orderID).Get(ctx, nil)
		return "", err
	}

	// Step 6: Send Confirmation (non-critical)
	status.Stage = "notify"
	err = workflow.ExecuteActivity(ctx, "SendOrderConfirmation", orderID, "customer@example.com").Get(ctx, nil)
	if err != nil {
		// Non-critical failure - log but continue
		status.LastError = fmt.Sprintf("confirmation failed: %v", err)
		logger.Warn("Confirmation email failed", "error", err)
	}

	status.Stage = "completed"
	result := fmt.Sprintf("Order %s completed (version %s)", orderID, status.Version)
	logger.Info("Workflow completed", "orderID", orderID)

	return result, nil
}
