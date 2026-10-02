package shared

import (
	"time"
)

// Decision is the authoritative security verdict enforced at the application edge.
type Decision struct {
	Action     Action    `json:"action"`
	Subject    Subject   `json:"subject"`
	Reason     string    `json:"reason"`
	Source     string    `json:"source"` // Policy engine or administrator
	Confidence float64   `json:"confidence"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"` // Mandatory TTL for ephemerality
}

// IsExpired checks if the decision has passed its temporal limit.
func (d Decision) IsExpired(now time.Time) bool {
	if d.ExpiresAt.IsZero() {
		return false
	}
	return now.After(d.ExpiresAt)
}

// IsExpired checks if the decision has passed its temporal limit.
func (d Decision) Active(now time.Time) bool {
	if !d.ExpiresAt.IsZero() && !now.Before(d.ExpiresAt) {
		return false
	}
	return true
}

func (d Decision) Allowed() bool           { return d.Action == ActionAllow }
func (d Decision) Denied() bool            { return d.Action == ActionBlock }
func (d Decision) RequiresChallenge() bool { return d.Action == ActionChallenge }
func (d Decision) Expired() bool           { return !d.ExpiresAt.IsZero() && time.Now().After(d.ExpiresAt) }
