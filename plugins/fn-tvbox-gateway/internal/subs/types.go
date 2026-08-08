package subs

import "time"

// Kind values align with bao-tv-box SubscriptionKind.
const (
	KindSingle    = "single"
	KindWarehouse = "warehouse"
	KindLive      = "live"
)

// Health values.
const (
	HealthUnknown  = "unknown"
	HealthHealthy  = "healthy"
	HealthError    = "error"
)

// Subscription is one managed subscription (top-level or warehouse child).
type Subscription struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Kind         string     `json:"kind"`
	Enabled      bool       `json:"enabled"`
	HealthStatus string     `json:"healthStatus"`
	LastSyncAt   string     `json:"lastSyncAt,omitempty"`
	ParentID     string     `json:"parentId,omitempty"`
	LastError    *LastError `json:"lastError,omitempty"`
}

// LastError is a per-subscription failure snapshot.
type LastError struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// ProbeResult is returned by kind detection.
type ProbeResult struct {
	OK           bool   `json:"ok"`
	DetectedKind string `json:"detectedKind"`
	SourceCount  int    `json:"sourceCount"`
	Name         string `json:"name,omitempty"`
	Message      string `json:"message,omitempty"`
}
