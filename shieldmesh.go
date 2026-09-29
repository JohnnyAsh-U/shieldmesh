package shieldmesh

import (
	"context"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/store"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

type Config struct {
	//Transport for the fabric distributed engines
	Transport transport.Transport

	//name of the node
	Name string

	//Policy to follow If local cache isn't available
	FailPolicy shared.FailPolicy
}

type Shield interface {
	// Used to asynchronously report telemetry to the intelligence plane. This is a fire-and-forget operation that never blocks the application path.
	Observe(ctx context.Context, req shared.Request) error

	// Used to explicitly ask the local enforcement state for a security decision. This executes in sub-microsecond time because it only queries local memory.
	Check(ctx context.Context, ev shared.Request) (store.Decision, bool)

	// Start the Sync Communication btw the broker and node
	Start(ctx context.Context)

	// Stop the sync communication
	Stop()
}

type ShieldMesh struct {
	tr    transport.Transport
	store *store.MapStore
}

func NewShieldMesh(cfg Config) Shield {
	// Set this node name in the transport
	cfg.Transport.SetNodeName(cfg.Name)

	return &ShieldMesh{
		tr:    cfg.Transport,
		store: store.NewMapStore(),
	}
}

func (s *ShieldMesh) Observe(ctx context.Context, req shared.Request) error {
	return s.tr.PublishEvent(ctx, req)
}

func (s *ShieldMesh) Check(ctx context.Context, req shared.Request) (store.Decision, bool) {
	return s.store.Get(req.Subject)
}

func (s *ShieldMesh) Start(ctx context.Context) {
	//Start expired decision deletion goroutine with time ticker when decision is more than 1000

	go func() {
		s.store.DeleteExpired()
	}()

	//go routine to sync and subscribe
	go func() {
		s.tr.Sync(ctx, s.store)
	}()

	s.tr.SubscribeDecisions(ctx, s.store)
}

func (s *ShieldMesh) Stop() {
	s.tr.Close()
}
