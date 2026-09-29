package main

import (
	"context"
	"log"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/engine"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

// A simple Threat Intelligence engine.
// It cross-references incoming IPs with a known "bad actor" list.
func main() {
	tr, err := transport.NewNatsTransport("nats://localhost:4222")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	eng, err := engine.NewEngine(tr.(*transport.NatsTransport), shared.EngineInfo{
		Name:         "threat-intel-engine",
		Capabilities: []string{"ip-reputation"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer eng.Close()

	// Mock database of known bad IPs (e.g. from an external API or DB)
	badIPs := map[string]bool{
		"198.51.100.14": true,
		"203.0.113.88":  true,
		"10.0.0.5":      true,
	}

	ctx := context.Background()
	log.Println("Threat Intel Engine running...")

	eng.Subscribe(ctx, func(req shared.Request) {
		ip := req.RemoteAddr

		// Instantly issue a block if the IP matches our threat feed
		if badIPs[ip] {
			log.Printf("[Threat Intel] Identified known bad actor IP: %s. Issuing 24h BLOCK.", ip)

			eng.PublishDecision(ctx, shared.Decision{
				Action:    shared.ActionBlock,
				Subject:   req.Subject,
				Reason:    "Known bad actor (IP Reputation)",
				ExpiresAt: time.Now().Add(24 * time.Hour), // Long block for known threats
			})
		}
	})

	select {} // Block forever
}
