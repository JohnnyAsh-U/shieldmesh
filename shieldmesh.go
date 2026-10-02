package shieldmesh

import (
	"context"
	"net/http"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
	"github.com/google/uuid"
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
	Check(ctx context.Context, ev shared.Request) (shared.Decision, bool)

	// Start the Sync Communication btw the broker and node
	Start(ctx context.Context)

	// Stop the sync communication
	Stop()

	// Middleware wraps an http.Handler with ShieldMesh enforcement
	Middleware(next http.Handler) http.Handler
}

type ShieldMesh struct {
	tr         transport.Transport
	store      *store.MapStore
	failPolicy shared.FailPolicy
	obsChan    chan shared.Request
}

func NewShieldMesh(cfg Config) Shield {
	// Set this node name in the transport
	cfg.Transport.SetNodeName(cfg.Name)

	sm := &ShieldMesh{
		tr:         cfg.Transport,
		store:      store.NewMapStore(),
		failPolicy: cfg.FailPolicy,
		obsChan:    make(chan shared.Request, 10000), // 10k event buffer for backpressure
	}

	// Start the asynchronous observation worker
	go sm.observationWorker()

	return sm
}

func (s *ShieldMesh) observationWorker() {
	// Continuously pull from the buffer and publish out-of-band
	for req := range s.obsChan {
		// Use a detached background context since the original HTTP context might be canceled
		s.tr.PublishEvent(context.Background(), req)
	}
}

func (s *ShieldMesh) Observe(ctx context.Context, req shared.Request) error {
	// Truly non-blocking fire-and-forget
	select {
	case s.obsChan <- req:
		return nil
	default:
		// If the buffer is full (e.g. NATS is down or network partitioned),
		// we drop the telemetry event to protect the application's hot path.
		return nil
	}
}

func (s *ShieldMesh) Check(ctx context.Context, req shared.Request) (shared.Decision, bool) {
	return s.store.Get(req.Subject)
}

func (s *ShieldMesh) Start(ctx context.Context) {
	//Start expired decision deletion goroutine with time ticker when decision is more than 1000

	go func() {
		s.store.DeleteExpired()
	}()

	s.tr.SubscribeDecisions(ctx, s.store)
}

func (s *ShieldMesh) Stop() {
	s.tr.Close()
}

func (s *ShieldMesh) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		clientIP := r.Header.Get("X-Forwarded-For")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Real-IP")
		}
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}

		RequestID := r.Header.Get("X-Request-ID")

		if RequestID == "" {
			RequestID = uuid.NewString()
		}

		req := shared.Request{
			ID:         RequestID, // Or generate one
			Method:     r.Method,
			Path:       r.URL.Path,
			RemoteAddr: clientIP,
			Subject: shared.Subject{
				Type: shared.SubjectIP,
				ID:   clientIP,
			},
			Timestamp: time.Now(),
		}

		// Fire and forget observation
		s.Observe(ctx, req)

		// Enforce local state
		decision, found := s.Check(ctx, req)

		// Apply Failure Policy if not found or expired
		if !found {
			// No decision found, or decision is expired
			if s.failPolicy == shared.FAILCLOSED {
				http.Error(w, "Service Unavailable (Fail Closed)", http.StatusServiceUnavailable)
				return
			}
			// Fail Open: just proceed
			next.ServeHTTP(w, r)
			return
		}

		// Act on decision
		switch decision.Action {
		case shared.ActionBlock:
			http.Error(w, "Forbidden: "+decision.Reason, http.StatusForbidden)
			return
		case shared.ActionChallenge:
			// Example: return 401 or similar challenge
			w.Header().Set("WWW-Authenticate", `Basic realm="ShieldMesh"`)
			http.Error(w, "Challenge required", http.StatusUnauthorized)
			return
		case shared.ActionAllow:
			next.ServeHTTP(w, r)
			return
		}

		// Default fallback
		next.ServeHTTP(w, r)
	})
}
