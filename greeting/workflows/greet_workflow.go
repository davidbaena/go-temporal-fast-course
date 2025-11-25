package workflows

import (
	"time"

	"go-temporal-fast-course/greeting/activities"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type GreetUserInput struct {
	UserID string
}

type GreetUserOutput struct {
	Message string
	SentAt  time.Time
	Success bool
}

func GreetUser(ctx workflow.Context, input GreetUserInput) (*GreetUserOutput, error) {
	// Configure activity options (timeouts, retries)
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second, // Activity must complete within 10s
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	logger := workflow.GetLogger(ctx)
	logger.Info("GreetUser workflow started", "UserID", input.UserID)

	// Step 1: Get User Details
	var userDetails *activities.UserDetails
	err := workflow.ExecuteActivity(ctx, "GetUserDetails", input.UserID).Get(ctx, &userDetails)
	if err != nil {
		logger.Error("GetUserDetails activity failed", "Error", err)
		return nil, err
	}

	logger.Info("GetUserDetails activity completed", "UserID", input.UserID)

	// Step 2: Create Greeting Message
	currentTime := workflow.Now(ctx)
	hour := currentTime.Hour()

	// Workflow logic
	message := formatMessage(hour, *userDetails)

	// Step 3: Send Greeting
	err = workflow.ExecuteActivity(ctx, "SendGreeting", userDetails.Email, message).Get(ctx, nil)
	if err != nil {
		logger.Error("SendGreeting activity failed", "Error", err)
		return nil, err
	}

	// Step 4: Log Greeting
	sendAt := workflow.Now(ctx)
	err = workflow.ExecuteActivity(ctx, "LogGreeting", input.UserID, message).Get(ctx, nil)
	if err != nil {
		logger.Error("LogGreeting activity failed", "Error", err)
		return nil, err
	}

	logger.Info("GreetUser workflow completed successfully")

	// Log Greeting
	output := GreetUserOutput{
		Message: message,
		SentAt:  sendAt,
		Success: true,
	}

	return &output, nil
}

func formatMessage(hour int, userDetails activities.UserDetails) string {
	var greeting string
	if hour < 12 {
		greeting = "Good Morning"
	} else if hour < 18 {
		greeting = "Good Afternoon"
	} else {
		greeting = "Good Evening"
	}

	message := greeting + ", " + userDetails.FirstName + " " + userDetails.LastName + "!"
	return message
}
