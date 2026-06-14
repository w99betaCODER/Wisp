package model

import "time"

// Role identifies an admin's privilege level.
type Role string

const (
	RoleSuper    Role = "super"    // manages everything: nodes, plans, other admins, all users
	RoleReseller Role = "reseller" // manages only the users they own
)

// Admin is a panel operator. The super-admin runs the infrastructure; resellers
// manage only their own users and sell plans.
type Admin struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // bcrypt hash, never serialized
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
