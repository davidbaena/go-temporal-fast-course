package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go-temporal-fast-course/order-processing/types"
)

// GreetUserInput is the input to the GreetUser workflow
type GreetUserInput struct {
	UserID string
}

// GreetUserResult is the output of the GreetUser workflow
type GreetUserResult struct {
	Message string
	SentAt  time.Time
	Success bool
}

// GreetUser is a simple workflow that greets a user
// This demonstrates the basics from Lesson 2
func GreetUser(ctx workflow.Context, input GreetUserInput) (*GreetUserResult, error) {
	// Configure activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	logger := workflow.GetLogger(ctx)
	logger.Info("GreetUser workflow started", "userID", input.UserID)

	// Execute activities in parallel (Lesson 2 pattern)
	futureDetails := workflow.ExecuteActivity(ctx, "GetUserDetails", input.UserID)
	futurePrefs := workflow.ExecuteActivity(ctx, "GetUserPreferences", input.UserID)

	// Wait for both
	var userDetails *types.UserDetails
	var prefs *types.UserPreferences

	err1 := futureDetails.Get(ctx, &userDetails)
	err2 := futurePrefs.Get(ctx, &prefs)

	if err1 != nil {
		logger.Error("Failed to get user details", "error", err1)
		return nil, fmt.Errorf("failed to get user details: %w", err1)
	}

	if err2 != nil {
		logger.Warn("Failed to get user preferences, using defaults", "error", err2)
		prefs = &types.UserPreferences{Language: "en"}
	}

	logger.Info("User details retrieved", "name", userDetails.FirstName)

	// Format greeting message based on language and time
	currentTime := workflow.Now(ctx)
	hour := currentTime.Hour()

	var greeting string
	switch prefs.Language {
	case "es":
		if hour < 12 {
			greeting = "Buenos días"
		} else if hour < 18 {
			greeting = "Buenas tardes"
		} else {
			greeting = "Buenas noches"
		}
	case "fr":
		greeting = "Bonjour"
	default:
		if hour < 12 {
			greeting = "Good morning"
		} else if hour < 18 {
			greeting = "Good afternoon"
		} else {
			greeting = "Good evening"
		}
	}

	message := fmt.Sprintf("%s, %s %s! Welcome to our e-commerce store.",
		greeting, userDetails.FirstName, userDetails.LastName)

	// Send greeting
	err := workflow.ExecuteActivity(ctx, "SendGreeting", userDetails.Email, message).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to send greeting", "error", err)
		return nil, fmt.Errorf("failed to send greeting: %w", err)
	}

	// Log the action (non-critical)
	err = workflow.ExecuteActivity(ctx, "LogGreeting", input.UserID, message).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to log greeting", "error", err)
	}

	logger.Info("GreetUser workflow completed successfully")

	return &GreetUserResult{
		Message: message,
		SentAt:  currentTime,
		Success: true,
	}, nil
}
