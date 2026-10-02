package shared

import (
	"errors"
	"time"
)

// Action defines the enforcement behavior applied to a Subject.
type Action string
type FailPolicy string
type SubjectsType string

const (
	ActionAllow     Action = "ALLOW"
	ActionBlock     Action = "BLOCK"
	ActionChallenge Action = "CHALLENGE"
)

var (
	FAILOPEN   FailPolicy = "open"
	FAILCLOSED FailPolicy = "closed"
)

var (
	SubjectIP   SubjectsType = "ip"
	SubjectID   SubjectsType = "id"
	SubjectName SubjectsType = "name"
)

var (
	ErrNotFound       = errors.New("Shieldmesh: not found")
	ErrStaleState     = errors.New("Shieldmesh: enforcement state is stale")
	ErrInvalidVersion = errors.New("Shieldmesh: invalid version")
	ErrUnauthorized   = errors.New("Shieldmesh: unauthorized")
)

type Subject struct {
	Type SubjectsType `json:"type"`
	ID   string       `json:"id"`
}

// Request is raw application telemetry pushed asynchronously into the fabric.
type Request struct {
	ID         string         `json:"id"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	RemoteAddr string         `json:"remote_addr"`
	Subject    Subject        `json:"subject"`
	Source     string         `json:"source"` // App component or service name
	Timestamp  time.Time      `json:"timestamp"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type EngineInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"caps"`
	LastSeen     int64    `json:"ts"`
}
