package store

import (
	// "go/types"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
	// "github.com/JohnnyAsh-U/shieldmesh/types"
	// "github.com/JohnnyAsh-U/shieldmesh"
	// "github.com/JohnnyAsh-U/
)



type MapStore struct {
	mu sync.RWMutex
	decisions map[string]Decision
	lastSeq uint64
	lastApplied time.Time
}

func NewMapStore() *MapStore{
	return &MapStore{
		decisions: make(map[string]Decision, 1024),
	}
}

func key(subject shared.Subject) string {
	return subject.Type+":"+subject.ID
}

func (s *MapStore) Apply(d Decision) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.Version <= s.lastSeq {
		return nil
	}

	// Never skip version
	if s.lastSeq != 0 && d.Version != s.lastSeq + 1 {
		return shared.ErrInvalidVersion
	}

	s.decisions[key(d.Subject)] = d
	s.lastApplied = time.Now()
	s.lastSeq = d.Version
	return nil
}

func (s *MapStore) Get(subject shared.Subject) (Decision, bool){
	s.mu.Lock()
	defer s.mu.RUnlock()

	d, ok := s.decisions[key(subject)]

	if !ok {
		return Decision{}, false
	}

	if !d.Active(time.Now()) {
		return Decision{}, false
	}

	return d, true
}

func (s *MapStore) Delete(subject shared.Subject){
	s.mu.Lock()
	delete(s.decisions, key(subject))
	s.mu.Unlock()
}

func (s *MapStore) DeleteExpired() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, v := range s.decisions {
		if v.ExpiresAt.Unix() < now.Unix() {
			delete(s.decisions, key(v.Subject))
			n++
		}
	}
	return n
}


func (s *MapStore) LastSeq() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSeq
}
