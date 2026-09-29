package main

import (
	"context"
	"log"
	"net/http"

	"github.com/JohnnyAsh-U/shieldmesh"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

// This example demonstrates how to protect a standard Go HTTP application
// by wrapping it with the Shield Mesh Middleware.
func main() {
	// 1. Connect to your transport layer (NATS JetStream)
	// For this example to work, you need a NATS server running locally: nats-server -js
	tr, err := transport.NewNatsTransport("nats://localhost:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v. Please ensure nats-server is running.", err)
	}
	defer tr.Close()

	// 2. Initialize Shield Mesh
	shield := shieldmesh.NewShieldMesh(shieldmesh.Config{
		Transport:  tr,
		Name:       "example-api-node",
		FailPolicy: shared.FAILOPEN, // We use FailOpen so the app stays up even if NATS is down
	})

	// 3. Start local state synchronization in the background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shield.Start(ctx)
	defer shield.Stop()

	// 4. Define your actual application logic
	myApp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello! You successfully reached the protected application.\n"))
	})

	// 5. Wrap your application with the Shield Mesh middleware
	protectedApp := shield.Middleware(myApp)

	log.Println("Protected Application listening on :8080")
	if err := http.ListenAndServe(":8080", protectedApp); err != nil {
		log.Fatal(err)
	}
}
