package types

// PermanentError represents an error that should not be retried
type PermanentError struct {
	Msg string
}

func (e *PermanentError) Error() string {
	return e.Msg
}

// PaymentTransientError represents a temporary payment error that can be retried
type PaymentTransientError struct {
	Msg string
}

func (e *PaymentTransientError) Error() string {
	return e.Msg
}

// ValidationError represents a validation error that should not be retried
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string {
	return e.Msg
}
