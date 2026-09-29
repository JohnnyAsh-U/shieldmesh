package transport

import (
	"context"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/store"
)


type Transport interface {
	Close() error
	// Engine
	// RegisterEngine(ctx context.Context, info EngineInfo)
	// Heartbeat(ctx context.Context, engineID string) error
	// StartHeartbeat(ctx context.Context, info EngineInfo)
	// ListEngines(ctx context.Context) ([]EngineInfo, error)
	// SubscribeRequests(ctx context.Context, handler func(Request)) error
	// PublishDecision(ctx context.Context, d Decision) (uint64, error)

	// node/middleware
	SetNodeName(name string)
	PublishEvent(ctx context.Context, req shared.Request) error
	Sync(ctx context.Context, store *store.MapStore) error
	SubscribeDecisions(ctx context.Context, store *store.MapStore) error
	GetDecision(ctx context.Context, subject shared.Subject) (store.Decision, bool)
}
