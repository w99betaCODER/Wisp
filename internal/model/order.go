package model

import "time"

// OrderStatus is the lifecycle state of an order.
type OrderStatus string

const (
	OrderPending  OrderStatus = "pending"  // awaiting payment
	OrderPaid     OrderStatus = "paid"     // settled; plan applied to the user
	OrderCanceled OrderStatus = "canceled" // abandoned or refunded
)

// Order records a user's purchase of a plan. When it becomes paid, the plan is
// applied to the user (expiry extended, quota reset). The provider field names
// the payment gateway; "manual" means an admin settled it by hand.
type Order struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	PlanID      string      `json:"plan_id"`
	AmountCents int64       `json:"amount_cents"`
	Currency    string      `json:"currency"`
	Status      OrderStatus `json:"status"`
	Provider    string      `json:"provider"`
	CreatedAt   time.Time   `json:"created_at"`
	PaidAt      *time.Time  `json:"paid_at,omitempty"`
}
