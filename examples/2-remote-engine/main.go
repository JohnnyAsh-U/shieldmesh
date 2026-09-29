package main

import (
	"context"
	"log"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/engine"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	// "github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

// This example demonstrates how to create a standalone Intelligence Engine
// that asynchronously monitors telemetry and publishes security decisions.
func main() {
	// 1. Connect to the fabric (NATS JetStream)
	tr, err := transport.NewNatsTransport("nats://localhost:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}

	// 2. Initialize the Engine SDK
	eng, err := engine.NewEngine(tr.(*transport.NatsTransport), shared.EngineInfo{
		Name:         "suspicious-path-detector",
		Capabilities: []string{"path-blocking"},
	})
	if err != nil {
		log.Fatalf("Failed to register engine: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	// 3. Subscribe to all telemetry requests flowing from application nodes
	log.Println("Engine is running... Listening for suspicious behavior.")

	err = eng.Subscribe(ctx, func(req shared.Request) {
		log.Printf("[Event] Engine observed request from IP: %s to Path: %s", req.RemoteAddr, req.Path)

		// 4. Perform security analysis
		// In this simple example, we immediately block any IP trying to access /admin
		if req.Path == "/admin" {
			log.Printf("[Detection] Suspicious access to /admin from %s! Publishing BLOCK decision.", req.RemoteAddr)

			// 5. Publish the decision back to the fabric
			eng.PublishDecision(ctx, shared.Decision{
				Action:    shared.ActionBlock,
				Subject:   req.Subject, // e.g. SubjectType="ip", ID="192.168.1.5"
				Reason:    "Unauthorized access to admin panel",
				ExpiresAt: time.Now().Add(5 * time.Minute), // The block automatically expires in 5 mins
			})
		}
	})

	if err != nil {
		log.Fatal(err)
	}

	// Block forever
	select {}
}
