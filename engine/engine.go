package engine

import (
	"context"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
	// "github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

type Engine struct {
	tr   transport.EngineTransport
	info shared.EngineInfo
}

func NewEngine(tr transport.EngineTransport, info shared.EngineInfo) (*Engine, error) {
	if info.ID == "" {
		// generate or require ID
		info.ID = "engine-" + time.Now().Format("20060102150405")
	}

	err := tr.RegisterEngine(context.Background(), info)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		tr:   tr,
		info: info,
	}

	go e.heartbeat()

	return e, nil
}

func (e *Engine) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		e.tr.Heartbeat(context.Background(), e.info.ID)
	}
}

// Subscribe listens to the incoming telemetry requests
func (e *Engine) Subscribe(ctx context.Context, handler func(req shared.Request)) error {
	return e.tr.SubscribeRequests(ctx, handler)
}

// PublishDecision publishes a security decision back to the fabric
func (e *Engine) PublishDecision(ctx context.Context, decision shared.Decision) (uint64, error) {
	// Stamp the decision with the engine source
	if decision.Source == "" {
		decision.Source = e.info.Name
	}
	return e.tr.PublishDecision(ctx, decision)
}

func (e *Engine) Close() error {
	return e.tr.Close()
}
