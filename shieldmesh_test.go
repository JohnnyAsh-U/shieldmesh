package shieldmesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
)

// MockTransport implements transport.Transport
type MockTransport struct{}

func (m *MockTransport) Close() error                                               { return nil }
func (m *MockTransport) SetNodeName(name string)                                    {}
func (m *MockTransport) PublishEvent(ctx context.Context, req shared.Request) error { return nil }
func (m *MockTransport) Sync(ctx context.Context, store *store.MapStore) error      { return nil }
func (m *MockTransport) SubscribeDecisions(ctx context.Context, store *store.MapStore) error {
	return nil
}
func (m *MockTransport) GetDecision(ctx context.Context, subject shared.Subject) (shared.Decision, bool) {
	return shared.Decision{}, false
}

func TestShieldMesh_Middleware_FailOpen(t *testing.T) {
	sm := NewShieldMesh(Config{
		Transport:  &MockTransport{},
		Name:       "test-node",
		FailPolicy: shared.FAILOPEN,
	})

	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for FAILOPEN, got %v", w.Result().StatusCode)
	}
}

func TestShieldMesh_Middleware_FailClosed(t *testing.T) {
	sm := NewShieldMesh(Config{
		Transport:  &MockTransport{},
		Name:       "test-node",
		FailPolicy: shared.FAILCLOSED,
	})

	handler := sm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for FAILCLOSED, got %v", w.Result().StatusCode)
	}
}

// BENCHMARKS
func BenchmarkShieldMesh_Observe(b *testing.B) {
	sm := NewShieldMesh(Config{
		Transport:  &MockTransport{},
		Name:       "test-node",
		FailPolicy: shared.FAILOPEN,
	})
	ctx := context.Background()
	req := shared.Request{ID: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Observe(ctx, req)
	}
}

func BenchmarkShieldMesh_Check(b *testing.B) {
	sm := NewShieldMesh(Config{
		Transport:  &MockTransport{},
		Name:       "test-node",
		FailPolicy: shared.FAILOPEN,
	})
	ctx := context.Background()
	req := shared.Request{
		Subject: shared.Subject{Type: "ip", ID: "127.0.0.1"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Check(ctx, req)
	}
}
