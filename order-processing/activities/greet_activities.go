package activities

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"go-temporal-fast-course/order-processing/types"
)

// GreetActivities contains greeting-related activities for the simple example
type GreetActivities struct{}

// GetUserDetails fetches user information
func (a *GreetActivities) GetUserDetails(ctx context.Context, userID string) (*types.UserDetails, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting user details", "userID", userID)

	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	// Simulate database lookup
	time.Sleep(100 * time.Millisecond)

	return &types.UserDetails{
		UserID:    userID,
		FirstName: "Alice",
		LastName:  "Johnson",
		Email:     "alice@example.com",
	}, nil
}

// GetUserPreferences fetches user preferences
func (a *GreetActivities) GetUserPreferences(ctx context.Context, userID string) (*types.UserPreferences, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting user preferences", "userID", userID)

	// Simulate database lookup
	time.Sleep(80 * time.Millisecond)

	return &types.UserPreferences{
		Language: "en", // Could be "es", "fr", etc.
	}, nil
}

// SendGreeting sends a greeting message
func (a *GreetActivities) SendGreeting(ctx context.Context, email string, message string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending greeting email", "email", email)

	// Simulate sending email
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("📧 Sending email to %s: %s\n", email, message)

	return nil
}

// LogGreeting logs the greeting action
func (a *GreetActivities) LogGreeting(ctx context.Context, userID string, message string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Logging greeting", "userID", userID)

	fmt.Printf("📝 Log: User %s greeted with: %s\n", userID, message)

	return nil
}
