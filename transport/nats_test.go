package transport

import (
	"context"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

func runTestServer() *natsserver.Server {
	opts := &natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
	}
	s, err := natsserver.NewServer(opts)
	if err != nil {
		panic(err)
	}
	s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		panic("NATS server not ready")
	}
	return s
}

func TestNatsPropagationDelay(t *testing.T) {
	s := runTestServer()
	defer s.Shutdown()

	clientURL := s.ClientURL()

	// Node transport
	tr, err := NewNatsTransport(clientURL)
	if err != nil {
		t.Fatalf("Failed to connect node to NATS: %v", err)
	}
	defer tr.Close()

	// Start sync on node
	nodeStore := store.NewMapStore()
	err = tr.SubscribeDecisions(context.Background(), nodeStore)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Wait for subscriptions to initialize
	time.Sleep(200 * time.Millisecond)

	// Publish from engine
	engineTr := tr.(EngineTransport)

	subject := shared.Subject{Type: "ip", ID: "10.0.0.1"}
	dec := shared.Decision{
		Action:  shared.ActionBlock,
		Subject: subject,
		Reason:  "Test propagation",
	}

	start := time.Now()
	_, err = engineTr.PublishDecision(context.Background(), dec)
	if err != nil {
		t.Fatalf("Failed to publish decision: %v", err)
	}

	// Poll nodeStore until it has the decision
	timeout := time.After(2 * time.Second)
	poll := time.NewTicker(1 * time.Millisecond)
	defer poll.Stop()

	var elapsed time.Duration
	found := false

waitLoop:
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for decision propagation")
		case <-poll.C:
			if _, ok := nodeStore.Get(subject); ok {
				elapsed = time.Since(start)
				found = true
				break waitLoop
			}
		}
	}

	if !found {
		t.Fatalf("Decision not found")
	}

	t.Logf("Decision propagated in %v", elapsed)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Propagation delay %v exceeds target of 50ms", elapsed)
	}
}

// BENCHMARK
func BenchmarkNatsPropagation(b *testing.B) {
	s := runTestServer()
	defer s.Shutdown()

	tr, _ := NewNatsTransport(s.ClientURL())
	defer tr.Close()

	nodeStore := store.NewMapStore()
	tr.SubscribeDecisions(context.Background(), nodeStore)
	time.Sleep(100 * time.Millisecond)

	engineTr := tr.(EngineTransport)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subject := shared.Subject{Type: "ip", ID: "10.0.0.1"}
		engineTr.PublishDecision(context.Background(), shared.Decision{
			Action:  shared.ActionBlock,
			Subject: subject,
		})
	}
}
