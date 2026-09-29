package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/store"
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
	return subject.Type + ":" + subject.ID
}

func (t *NatsTransport) Close() error {
	if t.nc != nil {
		t.nc.Close()
	}
	return nil
}

// Engine

// node/middleware
func (t *NatsTransport) GetDecision(ctx context.Context, subject shared.Subject) (store.Decision, bool) {
	e, err := t.decisionskv.Get(key(subject))
	if err != nil {
		return store.Decision{}, false
	}
	var d store.Decision
	if err := json.Unmarshal(e.Value(), &d); err != nil {
		return store.Decision{}, false
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
		var d store.Decision
		json.Unmarshal(e.Value(), &d)
		if err := s.Apply(d); err != nil {
			return fmt.Errorf("Error Synchronizing decision %v", err)
		}
	}
	return nil
}

func (t *NatsTransport) PublishEvent(ctx context.Context, req shared.Request) error {
	if req.ID == ""{
		return fmt.Errorf("ShieldMesh: ID required")
	}
	if req.Timestamp.IsZero(){
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
	last := storeMap.LastSeq()
	opt := nats.DeliverNew()

	// if last > 0 {
	// 	opt = nats.DeliverLast(last + 1)
	// }
	// t.js.PullSubscribe(DecisionSubject, )

	_, err := t.js.Subscribe(DecisionSubject, func(msg *natsgo.Msg) {
		// meta, _ := msg.Metadata()
		var d store.Decision
		json.Unmarshal(msg.Data, &d)
		if last > d.Version {
			storeMap.Apply(d)
		}
		msg.Ack()
	}, nats.Durable("node"+t.nodeName), opt, nats.ManualAck())
	return err
}


// // Engine
// RegisterEngine(ctx context.Context, info shieldmesh.EngineInfo)
// Heartbeat(ctx context.Context, engineID string) error
// StartHeartbeat(ctx context.Context, info shieldmesh.EngineInfo)
// ListEngines(ctx context.Context) ([]shieldmesh.EngineInfo, error)
// SubscribeRequests(ctx context.Context, handler func(shieldmesh.Request)) error
// PublishDecision(ctx context.Context, d shieldmesh.Decision) (uint64, error)

