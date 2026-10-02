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
)

// Requests don't have buckets, no storage, just streaming to avoid oom
const (
	RequestStreamName  = "REQUEST_STREAM"
	DecisionStreamName = "DECISION_STREAM"
	RequestSubject     = "shieldmesh.requests.>"
	DecisionSubject    = "shieldmesh.decisions"
	EnginesBucket      = "SHIELDMESH_ENGINES"
)

type NatsTransport struct {
	nodeName  string
	nc        *nats.Conn
	js        nats.JetStreamContext
	mu        sync.Mutex
	engineskv nats.KeyValue
}

func NewNatsTransport(url string, options ...nats.Option) (Transport, error) {
	nc, err := nats.Connect(url, options...)
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, err
	}

	// Ensure the Engine KV
	enginesKV, err := js.KeyValue(EnginesBucket)
	if err != nil {
		enginesKV, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: EnginesBucket,
			TTL:    1 * time.Minute, //time for the engines to signal their existence
		})
		if err != nil {
			nc.Close()
			return nil, err
		}
	}

	// Request Streams
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     RequestStreamName,
		Subjects: []string{RequestSubject},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour,
	})
	if err == nats.ErrStreamNameAlreadyInUse {
		_, err = js.UpdateStream(&nats.StreamConfig{
			Name:     RequestStreamName,
			Subjects: []string{RequestSubject},
			Storage:  nats.FileStorage,
			MaxAge:   24 * time.Hour,
		})
	}
	if err != nil {
		return nil, err
	}

	// Decision Streams
	_, err = js.AddStream(&nats.StreamConfig{
		Name:        DecisionStreamName,
		Subjects:    []string{DecisionSubject},
		Storage:     nats.FileStorage,
		MaxAge:      7 * 24 * time.Hour,
		AllowMsgTTL: true,
	})
	if err == nats.ErrStreamNameAlreadyInUse {
		_, err = js.UpdateStream(&nats.StreamConfig{
			Name:        DecisionStreamName,
			Subjects:    []string{DecisionSubject},
			MaxAge:      7 * 24 * time.Hour,
			Storage:     nats.FileStorage,
			AllowMsgTTL: true,
		})
	}
	if err != nil {
		return nil, err
	}

	return &NatsTransport{
		nc:        nc,
		js:        js,
		engineskv: enginesKV,
	}, nil
}

func (t *NatsTransport) SetNodeName(name string) {
	t.nodeName = name
}

func (t *NatsTransport) Close() error {
	if t.nc != nil {
		t.nc.Close()
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
	_, err = t.js.Publish(RequestSubject, payload, nats.MsgId(req.ID))
	return err
}

func (t *NatsTransport) SubscribeDecisions(ctx context.Context, storeMap *store.MapStore) error {
	opt := nats.DeliverNew()

	_, err := t.js.Subscribe(DecisionSubject, func(msg *nats.Msg) {
		meta, err := msg.Metadata()
		if err == nil {
			var d shared.Decision
			json.Unmarshal(msg.Data, &d)
			storeMap.Apply(d, meta.Sequence.Stream)
		}
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
		if err == nats.ErrNoKeysFound {
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
	_, err := t.js.Subscribe(RequestSubject, func(msg *nats.Msg) {
		var req shared.Request
		if err := json.Unmarshal(msg.Data, &req); err == nil {
			handler(req)
		}
		msg.Ack()
	}, nats.Durable("engine"+t.nodeName), nats.ManualAck())
	return err
}

func (t *NatsTransport) PublishDecision(ctx context.Context, d shared.Decision) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	// We will use the revision returned by KV as the version.
	payload, err := json.Marshal(d)
	if err != nil {
		return err
	}
	// Publish to the stream for synchronization
	_, err = t.js.Publish(DecisionSubject, payload)
	return err
}
