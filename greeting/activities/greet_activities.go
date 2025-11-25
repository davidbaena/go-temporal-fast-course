package activities

import (
	"context"
	"fmt"
	"time"
)

type UserDetails struct {
	UserId    string
	FirstName string
	LastName  string
	Email     string
}

type UserPreferences struct {
	Language string
}

type GreetActivities struct {
}

func (a *GreetActivities) GetUserDetails(ctx context.Context, userId string) (*UserDetails, error) {

	if userId == "" {
		return nil, fmt.Errorf("userId is empty")
	}

	return &UserDetails{
		UserId:    userId,
		FirstName: "John",
		LastName:  "Doe",
		Email:     "jondoe@example.com",
	}, nil
}

func (a *GreetActivities) SendGreeting(ctx context.Context, email string, message string) error {
	if email == "" {
		return fmt.Errorf("email is empty")
	}
	if message == "" {
		return fmt.Errorf("message is empty")
	}

	// Simulate sending email
	fmt.Printf("Sending greeting to %s: %s\n", email, message)

	// Simulate some delay
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (a *GreetActivities) LogGreeting(ctx context.Context, userId string, message string) error {
	fmt.Printf("Logging greeting to %s: %s\n", userId, message)
	return nil
}

func (a *GreetActivities) GetUserPreferencesId(ctx context.Context, userId string) (*UserPreferences, error) {

	if userId == "" {
		return nil, fmt.Errorf("userId is empty")
	}

	return &UserPreferences{
		Language: "ES",
	}, nil
}
