package transport

import (
	"context"

	"github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
)

type Transport interface {
	Close() error
	SetNodeName(name string)
	PublishEvent(ctx context.Context, req shared.Request) error
	// Sync(ctx context.Context, store *store.MapStore) error
	SubscribeDecisions(ctx context.Context, store *store.MapStore) error
	// GetDecision(ctx context.Context, subject shared.Subject) (shared.Decision, bool)
}

type EngineTransport interface {
	Close() error
	RegisterEngine(ctx context.Context, info shared.EngineInfo) error
	Heartbeat(ctx context.Context, engineID string) error
	ListEngines(ctx context.Context) ([]shared.EngineInfo, error)
	SubscribeRequests(ctx context.Context, handler func(shared.Request)) error
	PublishDecision(ctx context.Context, d shared.Decision) error
}
