package types

import "time"

// LineItem represents a product in an order
type LineItem struct {
	SKU      string
	Quantity int
}

// OrderEnrichment holds enriched order data
type OrderEnrichment struct {
	CustomerTier    string
	InventoryOk     bool
	Recommendations []string
}

// OrderWorkflowStatus represents the current state of an order workflow
type OrderWorkflowStatus struct {
	OrderID          string
	Stage            string
	Items            []LineItem
	Reserved         bool
	PaymentApproved  bool
	Charged          bool
	Cancelled        bool
	LastError        string
	Enrichment       OrderEnrichment
	ApprovalDeadline time.Time
	Version          string
}

// PaymentApproval is the signal payload for approving payment
type PaymentApproval struct {
	ApprovedBy string
	Timestamp  time.Time
}

// CancelRequest is the signal payload for cancelling an order
type CancelRequest struct {
	Reason string
}

// UserDetails represents user information
type UserDetails struct {
	UserID    string
	FirstName string
	LastName  string
	Email     string
}

// UserPreferences represents user preferences
type UserPreferences struct {
	Language string
}
