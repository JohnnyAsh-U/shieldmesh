package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

func main() {
	var (
		listenAddr = flag.String("listen", ":8080", "Address to listen on")
		targetURL  = flag.String("target", "http://localhost:9091", "Target backend URL to proxy to (only in proxy mode)")
		natsURL    = flag.String("nats", "nats://localhost:4222", "NATS JetStream URL")
		failPolicy = flag.String("fail-policy", "open", "Failure policy: 'open' or 'closed'")
		nodeName   = flag.String("node", "shieldd-proxy-1", "Name of the enforcement node")
		mode       = flag.String("mode", "proxy", "Running mode: 'proxy' (reverse proxy) or 'auth' (nginx auth_request)")
	)
	flag.Parse()

	log.Printf("Starting shieldd proxy node: %s", *nodeName)
	log.Printf("Connecting to NATS at %s", *natsURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize NATS Transport
	tr, err := transport.NewNatsTransport(*natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer tr.Close()

	// Determine fail policy
	policy := shared.FAILOPEN
	if *failPolicy == "closed" {
		policy = shared.FAILCLOSED
	}

	// Initialize ShieldMesh
	shieldMesh := shieldmesh.NewShieldMesh(shieldmesh.Config{
		Transport:  tr,
		Name:       *nodeName,
		FailPolicy: policy,
	})

	// Start synchronization
	log.Println("Starting local enforcement state synchronization...")
	shieldMesh.Start(ctx)
	defer shieldMesh.Stop()

	var baseHandler http.Handler

	if *mode == "auth" {
		// NGINX auth_request mode: return 200 OK if allowed
		log.Println("Running in NGINX auth_request mode")
		baseHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
	} else {
		// Parse target URL and setup reverse proxy
		target, err := url.Parse(*targetURL)
		if err != nil {
			log.Fatalf("Invalid target URL: %v", err)
		}
		log.Printf("Running in reverse proxy mode forwarding to %s", *targetURL)
		baseHandler = httputil.NewSingleHostReverseProxy(target)
	}

	// Wrap handler with ShieldMesh middleware
	handler := shieldMesh.Middleware(baseHandler)

	// Create server
	server := &http.Server{
		Addr:    *listenAddr,
		Handler: handler,
	}

	// Handle graceful shutdown
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Println("Shutting down gracefully...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		cancel()
	}()

	log.Printf("Listening on %s with fail policy: %s", *listenAddr, *failPolicy)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
