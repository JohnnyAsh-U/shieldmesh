# 05 - Developer API & SDK

This document defines the Developer Experience (DX) for the Shield Mesh Go SDK.

The guiding philosophy of this SDK is that **application developers describe security intent, while infrastructure engineers define deployment topology.** The core developer API must remain completely oblivious to NATS, JetStream, distributed state stores, or networking protocols.

---

## 1. Application Initialization

This is the standard usage pattern for an application developer adding Shield Mesh to a Go web server. Shield Mesh intentionally avoids embedding security engines directly within the SDK, relying instead on an external transport layer.

```go
package main

import (
	"log"
	"net/http"
	"[github.com/shieldmesh/shieldmesh/pkg/shieldmesh](https://github.com/shieldmesh/shieldmesh/pkg/shieldmesh)"
	"[github.com/shieldmesh/shieldmesh/transports/nats](https://github.com/shieldmesh/shieldmesh/transports/nats)"
)

func main() {
	// 1. Initialize Shield Mesh with a transport and explicit failure policy
	shield, err := shieldmesh.New(shieldmesh.Config{
		Transport:  nats.New(nats.WithURL("nats://localhost:4222")),
		Name:       "node-1",
		FailPolicy: shieldmesh.FailOpen,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer shield.Close()

	// 2. Define standard application logic
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Protected by Shield Mesh"))
	})

	// 3. Wrap the application with Shield Mesh middleware
	handler := shield.Middleware(app)

	log.Println("Listening on :8080")
	http.ListenAndServe(":8080", handler)
}
```

---

## 2. The Core Developer API

Behind the middleware, the SDK exposes two primary operations. Developers can use these explicitly if they need fine-grained control inside their application logic.

### `Observe(ctx, event)`

Used to asynchronously report telemetry to the intelligence plane. This is a fire-and-forget operation that never blocks the application path. The API represents acceptance for asynchronous processing, not a guarantee that a decision has already been produced.

```go
err := shield.Observe(ctx, shieldmesh.Event{
	Type: shieldmesh.EventAuthenticationFailed,
	Subject: shieldmesh.Subject{
		Type: shieldmesh.SubjectIP,
		ID:   "203.0.113.10",
	},
	Resource: "/api/v1/login",
	Metadata: map[string]any{
		"username": "admin",
		"endpoint": "/api/login",
	},
})
```

### `Check(ctx, request)`
Used to explicitly ask the local enforcement state for a security decision. This executes in sub-microsecond time because it only queries local memory; no remote intelligence engine needs to be contacted during normal request evaluation.

```go
decision, err := shield.Check(ctx, shieldmesh.Request{
	Subject: shieldmesh.Subject{
		Type: shieldmesh.SubjectUser,
		ID:   "usr_123",
	},
})

if decision.Action == shieldmesh.ActionBlock {
	http.Error(w, "Forbidden: "+decision.Reason, http.StatusForbidden)
	return
}
```

---

## 3. The Engine SDK Remote Intelligence Engines
Shield Mesh does not embed security intelligence engines inside the SDK. Engines are strictly remote, external components (like a WAF, SIEM, or behavioral detection system) that produce intelligence. ShieldMesh is responsible solely for distributing the resulting decisions to local enforcement points. This separation allows security engines and applications to evolve independently.
<!-- To simple code snippet for the engine be shown here -->

## 4. Scaling to the Fabric (The Topology Invariant)

The most important architectural rule of the SDK is: **Changing deployment topology must not require application security logic to change**.

When a startup grows from a single server to a distributed fleet of 50 microservices, the developer **does not** rewrite their `Check()` or `Observe() `calls. They simply change the initialization parameters to inject a distributed transport.

```go
import (
	"github.com/ashmesh/ashmesh/pkg/shieldmesh"
	"github.com/ashmesh/ashmesh/transports/nats"
)


// The ONLY code change required to scale to a Distributed Fabric
shield, err := shieldmesh.New(shieldmesh.Config{
	Transport: nats.New(
		nats.WithURL("nats://shieldmesh-cluster.internal:4222"), 
	),
	Name:       "node-fabric-01",
	FailPolicy: shieldmesh.FailClosed,
})
```


Because `transports/nats` is an external package that satisfies the internal `Transport` interface, the core application logic remains completely unaware that its events are now routing through a JetStream cluster. Applications can use either NATS or the native Shield Mesh Custom Transport.


## 5. Failure Policy
ShieldMesh provides two explicit request-path failure policies. Applications explicitly choose their policy during initialization. The failure policy is part of the security model rather than an implicit behavior.


### Fail Open
Prioritizes availability if the enforcement state cannot be trusted or determined.
 * State valid → enforce decision
 * State unavailable → allow
 * State corrupted → allow
 * State ambiguous → allow

### Fail Closed
Prioritizes strict security if the enforcement state cannot be trusted or determined.
 * State valid → enforce decision
 * State unavailable → block
 * State corrupted → block
 * State ambiguous → block
