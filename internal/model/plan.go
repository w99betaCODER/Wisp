package model

import "time"

// Plan is a purchasable tariff. Buying a plan grants a user access for
// DurationDays and (re)sets their data quota.
type Plan struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`          // e.g. "1 month · 100 GB"
	PriceCents   int64     `json:"price_cents"`   // price in minor units (e.g. cents)
	Currency     string    `json:"currency"`      // ISO 4217, e.g. "USD"
	DurationDays int       `json:"duration_days"` // access granted per purchase
	DataLimit    int64     `json:"data_limit"`    // byte quota; 0 = unlimited
	CreatedAt    time.Time `json:"created_at"`
}
