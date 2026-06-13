// Package model holds the core domain types shared across the panel.
package model

import "time"

// User represents a single VPN account managed by the panel.
//
// One User maps to one client entry inside Xray. The UUID is what the
// VPN client actually authenticates with (for VLESS/VMess); Email is the
// human-friendly identifier Xray uses to tag traffic statistics.
type User struct {
	ID        string     `json:"id"`                   // panel-internal primary key
	Email     string     `json:"email"`                // identifier used inside Xray
	UUID      string     `json:"uuid"`                 // VLESS client UUID
	Enabled   bool       `json:"enabled"`              // false = access revoked
	DataLimit int64      `json:"data_limit"`           // byte quota; 0 = unlimited
	Used      int64      `json:"used"`                 // bytes consumed so far
	CreatedAt time.Time  `json:"created_at"`           // when the account was created
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = never expires
}
