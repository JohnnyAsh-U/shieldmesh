package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
)

func TestMapStore_ApplyAndGet(t *testing.T) {
	s := NewMapStore()
	subject := shared.Subject{Type: "ip", ID: "127.0.0.1"}

	// Apply a valid decision
	d := shared.Decision{
		Action:    shared.ActionBlock,
		Subject:   subject,
		Version:   1,
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	err := s.Apply(d)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// Get the decision
	fetched, found := s.Get(subject)
	if !found {
		t.Fatalf("expected to find decision")
	}
	if fetched.Action != shared.ActionBlock {
		t.Fatalf("expected BLOCK, got %v", fetched.Action)
	}
}

func TestMapStore_ExpiredDecision(t *testing.T) {
	s := NewMapStore()
	subject := shared.Subject{Type: "ip", ID: "127.0.0.1"}

	d := shared.Decision{
		Action:    shared.ActionBlock,
		Subject:   subject,
		Version:   1,
		ExpiresAt: time.Now().Add(-1 * time.Minute), // Already expired
	}

	s.Apply(d)

	// Get should return false
	_, found := s.Get(subject)
	if found {
		t.Fatalf("expected not to find expired decision")
	}

	// DeleteExpired should remove it
	n := s.DeleteExpired()
	if n != 1 {
		t.Fatalf("expected 1 deletion, got %v", n)
	}
}

// BENCHMARKS
func BenchmarkStore_Get(b *testing.B) {
	s := NewMapStore()
	subject := shared.Subject{Type: "ip", ID: "127.0.0.1"}

	d := shared.Decision{
		Action:    shared.ActionBlock,
		Subject:   subject,
		Version:   1,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	s.Apply(d)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get(subject)
	}
}

func BenchmarkStore_Apply(b *testing.B) {
	s := NewMapStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := shared.Decision{
			Action:    shared.ActionBlock,
			Subject:   shared.Subject{Type: "ip", ID: fmt.Sprintf("127.0.0.%d", i%255)},
			Version:   uint64(i + 1),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		s.Apply(d)
	}
}
