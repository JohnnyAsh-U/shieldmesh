package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/engine"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

// A simple sliding-window rate limiter engine.
// If an IP makes more than 50 requests in 10 seconds, they are blocked for 5 minutes.
func main() {
	tr, err := transport.NewNatsTransport("nats://localhost:4222")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	eng, err := engine.NewEngine(tr.(*transport.NatsTransport), shared.EngineInfo{
		Name:         "rate-limit-engine",
		Capabilities: []string{"rate-limiting"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer eng.Close()

	// Local memory to track request counts
	type state struct {
		count int
		reset time.Time
	}
	counts := make(map[string]*state)
	var mu sync.Mutex

	ctx := context.Background()
	log.Println("Rate Limit Engine running...")

	eng.Subscribe(ctx, func(req shared.Request) {
		ip := req.RemoteAddr
		now := time.Now()

		mu.Lock()
		defer mu.Unlock()

		s, exists := counts[ip]
		if !exists || now.After(s.reset) {
			s = &state{count: 0, reset: now.Add(10 * time.Second)}
			counts[ip] = s
		}

		s.count++

		// If threshold exceeded, issue a BLOCK decision
		if s.count > 50 {
			log.Printf("[Rate Limit] IP %s exceeded 50 req/10s. Issuing 5-min BLOCK.", ip)
			eng.PublishDecision(ctx, shared.Decision{
				Action:    shared.ActionBlock,
				Subject:   req.Subject,
				Reason:    "Rate limit exceeded (50req/10s)",
				ExpiresAt: now.Add(5 * time.Minute),
			})

			// Reset so we don't spam the network with duplicate blocks
			s.count = -1000
		}
	})

	select {} // Block forever
}
