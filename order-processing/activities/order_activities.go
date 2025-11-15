package activities

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/activity"

	"go-temporal-fast-course/order-processing/types"
)

// InventoryActivities contains inventory-related activities
type InventoryActivities struct{}

// ReserveStock reserves inventory for an order
func (a *InventoryActivities) ReserveStock(ctx context.Context, orderID string, items []types.LineItem) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Reserving stock", "orderID", orderID, "items", items)

	// Simulate reservation logic
	time.Sleep(100 * time.Millisecond)

	// Simulate occasional transient failures
	if rand.Float32() < 0.1 {
		return fmt.Errorf("temporary inventory system error")
	}

	logger.Info("Stock reserved successfully", "orderID", orderID)
	return nil
}

// ReleaseStock releases reserved inventory (compensation)
func (a *InventoryActivities) ReleaseStock(ctx context.Context, orderID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Releasing stock", "orderID", orderID)

	// Simulate release logic
	time.Sleep(50 * time.Millisecond)

	logger.Info("Stock released successfully", "orderID", orderID)
	return nil
}

// FetchInventorySnapshot checks if items are available in inventory
func (a *InventoryActivities) FetchInventorySnapshot(ctx context.Context, items []types.LineItem) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching inventory snapshot", "items", items)

	// Simulate inventory check
	time.Sleep(200 * time.Millisecond)

	// Simulate inventory availability (90% available)
	available := rand.Float32() > 0.1

	logger.Info("Inventory check complete", "available", available)
	return available, nil
}

// PaymentActivities contains payment-related activities
type PaymentActivities struct{}

// ProcessPayment processes payment for an order
func (a *PaymentActivities) ProcessPayment(ctx context.Context, orderID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing payment", "orderID", orderID)

	// Simulate payment processing
	time.Sleep(300 * time.Millisecond)

	// Simulate different failure scenarios
	r := rand.Float32()
	switch {
	case r < 0.2:
		// Temporary gateway issue (retryable)
		logger.Warn("Payment gateway timeout", "orderID", orderID)
		return &types.PaymentTransientError{Msg: "gateway timeout"}
	case r < 0.25:
		// Permanent card decline (non-retryable)
		logger.Error("Card declined", "orderID", orderID)
		return &types.PermanentError{Msg: "card declined"}
	}

	logger.Info("Payment processed successfully", "orderID", orderID)
	return nil
}

// RefundPayment refunds a payment (compensation)
func (a *PaymentActivities) RefundPayment(ctx context.Context, orderID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Refunding payment", "orderID", orderID)

	// Simulate refund logic
	time.Sleep(200 * time.Millisecond)

	logger.Info("Payment refunded successfully", "orderID", orderID)
	return nil
}

// CustomerActivities contains customer-related activities
type CustomerActivities struct{}

// FetchCustomerProfile fetches customer tier information
func (a *CustomerActivities) FetchCustomerProfile(ctx context.Context, orderID string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching customer profile", "orderID", orderID)

	// Simulate customer lookup
	time.Sleep(150 * time.Millisecond)

	// Simulate customer tiers
	tiers := []string{"Bronze", "Silver", "Gold", "Platinum"}
	tier := tiers[rand.Intn(len(tiers))]

	logger.Info("Customer profile fetched", "tier", tier)
	return tier, nil
}

// RecommendationActivities contains recommendation-related activities
type RecommendationActivities struct{}

// FetchRecommendations fetches product recommendations
func (a *RecommendationActivities) FetchRecommendations(ctx context.Context, orderID string) ([]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching recommendations", "orderID", orderID)

	// Simulate recommendation engine
	time.Sleep(100 * time.Millisecond)

	recommendations := []string{"Product-A", "Product-B", "Product-C"}

	logger.Info("Recommendations fetched", "count", len(recommendations))
	return recommendations, nil
}

// OrderActivities contains order-related activities
type OrderActivities struct{}

// UpdateOrderStatus updates the order status in the database
func (a *OrderActivities) UpdateOrderStatus(ctx context.Context, orderID string, status string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating order status", "orderID", orderID, "status", status)

	// Simulate database update
	time.Sleep(100 * time.Millisecond)

	// Simulate occasional transient failures
	if rand.Float32() < 0.05 {
		return fmt.Errorf("database connection timeout")
	}

	logger.Info("Order status updated successfully", "orderID", orderID, "status", status)
	return nil
}

// NotificationActivities contains notification-related activities
type NotificationActivities struct{}

// SendOrderConfirmation sends order confirmation email
func (a *NotificationActivities) SendOrderConfirmation(ctx context.Context, orderID string, email string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending order confirmation", "orderID", orderID, "email", email)

	// Simulate email sending
	time.Sleep(200 * time.Millisecond)

	// Simulate occasional failures (non-critical)
	if rand.Float32() < 0.1 {
		logger.Warn("Failed to send confirmation email", "orderID", orderID)
		return fmt.Errorf("email service unavailable")
	}

	logger.Info("Order confirmation sent", "orderID", orderID)
	return nil
}

// SendCancellationEmail sends cancellation email
func (a *NotificationActivities) SendCancellationEmail(ctx context.Context, orderID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending cancellation email", "orderID", orderID, "reason", reason)

	// Simulate email sending
	time.Sleep(150 * time.Millisecond)

	logger.Info("Cancellation email sent", "orderID", orderID)
	return nil
}
