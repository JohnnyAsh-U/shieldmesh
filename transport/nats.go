package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/internal/store"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/nats-io/nats.go"
	natsgo "github.com/nats-io/nats.go"
)

// Requests don't have buckets, no storage, just streaming to avoid oom
const (
	RequestStreamName  = "REQUEST_STREAM"
	DecisionStreamName = "DECISION_STREAM"
	RequestSubject     = "shieldmesh.requests.>"
	DecisionSubject    = "shieldmesh.requests.decisions"
	DecisionBucket     = "SHIELDMESH_DECISIONS"
	EnginesBucket      = "SHIELDMESH_ENGINES"
)

type NatsTransport struct {
	nodeName    string
	nc          *natsgo.Conn
	js          natsgo.JetStreamContext
	mu          sync.Mutex
	decisionskv natsgo.KeyValue
	engineskv   natsgo.KeyValue
}

func NewNatsTransport(url string, options ...natsgo.Option) (Transport, error) {
	nc, err := natsgo.Connect(url, options...)
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, err
	}

	// Ensure the Decision KV, only 5 most recent
	decisionKV, err := js.KeyValue(DecisionBucket)
	if err != nil {
		decisionKV, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:  DecisionBucket,
			History: 5,
		})
		if err != nil {
			nc.Close()
			return nil, err
		}
	}

	// Ensure the Engine KV
	enginesKV, err := js.KeyValue(DecisionBucket)
	if err != nil {
		enginesKV, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: EnginesBucket,
		})
		if err != nil {
			nc.Close()
			return nil, err
		}
	}

	// Request Streams
	js.AddStream(&nats.StreamConfig{
		Name:     RequestStreamName,
		Subjects: []string{RequestSubject},
	})

	// Decision Streams
	js.AddStream(&nats.StreamConfig{
		Name:        DecisionStreamName,
		Subjects:    []string{DecisionSubject},
		MaxAge:      7 * 24 * time.Hour,
		AllowMsgTTL: true,
	})

	return &NatsTransport{
		nc:          nc,
		js:          js,
		decisionskv: decisionKV,
		engineskv:   enginesKV,
	}, nil
}

func (t *NatsTransport) SetNodeName(name string) {
	t.nodeName = name
}

func key(subject shared.Subject) string {
	return subject.Type + "_" + subject.ID
}

func (t *NatsTransport) Close() error {
	if t.nc != nil {
		t.nc.Close()
	}
	return nil
}

// Engine

// node/middleware
func (t *NatsTransport) GetDecision(ctx context.Context, subject shared.Subject) (shared.Decision, bool) {
	e, err := t.decisionskv.Get(key(subject))
	if err != nil {
		return shared.Decision{}, false
	}
	var d shared.Decision
	if err := json.Unmarshal(e.Value(), &d); err != nil {
		return shared.Decision{}, false
	}
	return d, true
}

func (t *NatsTransport) Sync(ctx context.Context, s *store.MapStore) error {
	keys, err := t.decisionskv.Keys()
	if err != nil {
		return fmt.Errorf("Error Synchronizing %v", err)
	}

	for _, k := range keys {
		e, err := t.decisionskv.Get(k)
		if err != nil {
			continue
		}
		var d shared.Decision
		json.Unmarshal(e.Value(), &d)
		if err := s.Apply(d); err != nil {
			return fmt.Errorf("Error Synchronizing decision %v", err)
		}
	}
	return nil
}

func (t *NatsTransport) PublishEvent(ctx context.Context, req shared.Request) error {
	if req.ID == "" {
		return fmt.Errorf("ShieldMesh: ID required")
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = t.js.Publish(RequestSubject, payload, natsgo.MsgId(req.ID))
	return err
}

func (t *NatsTransport) SubscribeDecisions(ctx context.Context, storeMap *store.MapStore) error {
	opt := nats.DeliverNew()

	_, err := t.js.Subscribe(DecisionSubject, func(msg *natsgo.Msg) {
		// meta, _ := msg.Metadata()
		var d shared.Decision
		json.Unmarshal(msg.Data, &d)
		storeMap.Apply(d)
		msg.Ack()
	}, nats.Durable("node"+t.nodeName), opt, nats.ManualAck())
	return err
}

// EngineTransport implementation

func (t *NatsTransport) RegisterEngine(ctx context.Context, info shared.EngineInfo) error {
	info.LastSeen = time.Now().Unix()
	payload, _ := json.Marshal(info)
	_, err := t.engineskv.Put(info.ID, payload)
	return err
}

func (t *NatsTransport) Heartbeat(ctx context.Context, engineID string) error {
	entry, err := t.engineskv.Get(engineID)
	if err != nil {
		return err
	}
	var info shared.EngineInfo
	if err := json.Unmarshal(entry.Value(), &info); err != nil {
		return err
	}
	info.LastSeen = time.Now().Unix()
	payload, _ := json.Marshal(info)
	_, err = t.engineskv.Put(info.ID, payload)
	return err
}

func (t *NatsTransport) ListEngines(ctx context.Context) ([]shared.EngineInfo, error) {
	keys, err := t.engineskv.Keys()
	if err != nil {
		if err == natsgo.ErrNoKeysFound {
			return []shared.EngineInfo{}, nil
		}
		return nil, err
	}
	var engines []shared.EngineInfo
	for _, k := range keys {
		if entry, err := t.engineskv.Get(k); err == nil {
			var info shared.EngineInfo
			if err := json.Unmarshal(entry.Value(), &info); err == nil {
				engines = append(engines, info)
			}
		}
	}
	return engines, nil
}

func (t *NatsTransport) SubscribeRequests(ctx context.Context, handler func(shared.Request)) error {
	// Create an ephemeral consumer for engines to process streams of requests
	_, err := t.js.Subscribe(RequestSubject, func(msg *natsgo.Msg) {
		var req shared.Request
		if err := json.Unmarshal(msg.Data, &req); err == nil {
			handler(req)
		}
		msg.Ack()
	}, nats.DeliverNew(), nats.ManualAck())
	return err
}

func (t *NatsTransport) PublishDecision(ctx context.Context, d shared.Decision) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get latest version for this subject to increment (simple global sequence or per-subject? The architecture mentions a global sequence usually, but let's use global JetStream sequence or KV rev).
	// We will use the revision returned by KV as the version.
	payload, err := json.Marshal(d)
	if err != nil {
		return 0, err
	}

	rev, err := t.decisionskv.Put(key(d.Subject), payload)
	if err != nil {
		return 0, err
	}

	// Update the decision with the version and re-publish so it can be streamed
	d.Version = rev
	payload, _ = json.Marshal(d)
	_, err = t.decisionskv.Put(key(d.Subject), payload) 
	if err != nil {
		return 0, err
	}

	// Publish to the stream for synchronization
	_, err = t.js.Publish(DecisionSubject, payload)
	return rev + 1, err
}
