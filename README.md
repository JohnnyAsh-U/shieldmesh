# Shield Mesh

**Distributed security intelligence. Local enforcement.**

Shield Mesh is a distributed security decision fabric that connects remote security intelligence engines to applications through low-latency local enforcement.


It separates security intelligence from the application request path:

- Applications emit security telemetry asynchronously.
- Remote security engines analyze that telemetry.
- Engines publish security decisions through the Shield Mesh transport.
- Application nodes maintain local enforcement state.
- Requests are evaluated locally without requiring a network round trip to an intelligence engine.

The goal is simple:

 Let security intelligence be distributed without putting the intelligence plane on the request path.

---

## Architecture


                         Security Intelligence Plane

              ┌──────────────────────────────────────┐
              │          Remote Engines              │
              │                                      │
              │  WAF · Threat Intel · SIEM · ML     │
              │  Behavioral Detection · Custom      │
              └──────────────────┬───────────────────┘
                                 │
                                 │ Decisions
                                 ▼
                        ┌─────────────────┐
                        │  Shield Mesh    │
                        │    Transport    │
                        │                 │
                        │ NATS / Custom   │
                        └────────┬────────┘
                                 │
                                 │ Synchronization
                                 ▼
                    ┌────────────────────────┐
                    │   Local Enforcement    │
                    │         State          │
                    └───────────┬────────────┘
                                │
                              Check()
                                │
                                ▼
                         ┌──────────────┐
                         │ Application  │
                         └──────────────┘

Application ── Observe() ──► Intelligence Plane
Application ── Check() ────► Local Enforcement State

The two planes are intentionally separated.

**Intelligence plane**

Responsible for:

- collecting security telemetry
- analyzing events
- generating security decisions
- distributing decisions
- retaining decision history for synchronization


**Enforcement plane**

Responsible for:

- maintaining local decision state
- evaluating requests
- enforcing decisions
- continuing to operate when the intelligence plane is unavailable

The enforcement plane does not query a remote engine for every request.

---

## Core API


Shield Mesh exposes two primary operations.

### "Observe"

Send security telemetry to the intelligence plane.
```
err := shield.Observe(ctx, request)
```

"Observe" is asynchronous by design. It does not wait for a security engine to analyze the event or generate a decision.

Conceptually:

```
Application
    │
    │ Observe()
    ▼
Shield Mesh
    │
    │ asynchronous
    ▼
Intelligence Plane

The API represents acceptance for asynchronous processing, not a guarantee that a decision has already been produced.
```

---

### "Check"

Evaluate a request against locally available enforcement state.

```
decision, err := shield.Check(ctx, request)
```

The request path uses local state:

```
Request
   │
   ▼
Check()
   │
   ├── Allow
   ├── Block
   └── Challenge
No remote intelligence engine needs to be contacted during normal request evaluation.
```

---

## Decisions

Shield Mesh currently has three enforcement actions:
```
ALLOW
BLOCK
CHALLENGE
```
A decision contains the information required to determine whether and how it should be enforced.

Conceptually:
```
type Decision struct {
    Subject   string
    Action    Action
    Version   uint64

    Version  string
    Reason    string
    Confidence float64

    IssuedAt  time.Time
    ExpiresAt time.Time
}
```
Decisions are versioned so that enforcement nodes can determine which state they have applied and recover from missed updates.

---

## Remote Intelligence Engines

Shield Mesh does not embed security intelligence engines inside the SDK.

Engines are external components.

Examples include:

- Web Application Firewalls
- threat-intelligence systems
- SIEM systems
- behavioral detection systems
- anomaly detection systems
- machine-learning systems
- custom security engines

The engine produces intelligence; Shield Mesh distributes the resulting decisions to enforcement points.

This separation allows security engines and applications to evolve independently.
```

             Remote Engines
                  │
        ┌─────────┼─────────┐
        │         │         │
       WAF       SIEM       ML
        │         │         │
        └─────────┼─────────┘
                  │
                  ▼
             Shield Mesh
                  │
                  ▼
          Local Enforcement
```
---
## NATS Transport

The first transport implementation uses NATS JetStream.

NATS is an implementation detail of the transport layer rather than part of the Shield Mesh programming model.

The architecture is designed around two transport implementations:
```
Shield Mesh Transport
        │
        ├── NATS
        │
        └── Shield Mesh Custom Transport
```

Applications/Engines can use either NATS or ShieldMesh Custom broker

---

## NATS State Model

The NATS implementation maintains separate state for telemetry and decisions.

**Requests**

Security telemetry is retained for a default period of: **24 hours**
This provides a bounded window for asynchronous processing, replay, and operational investigation.

**Decisions**

Security decisions are retained for a default period of: **7 days**
This provides a recovery window for enforcement nodes that temporarily disconnect.

Conceptually:

```
NATS / JetStream / Custom Fabric(Coming soon)

┌─────────────────────┐
│ Request/Event Data  │
│                     │
│ Retention: 24h      │
└─────────────────────┘

┌─────────────────────┐
│ Decision State      │
│                     │
│ Retention: 7 days   │
└─────────────────────┘
```

These are default retention policies and can be configured by the deployment.

---


## Decision Synchronization

An enforcement node maintains the version of the latest decision state it has successfully applied.

For example:

```
AppliedVersion = 1842
```

When the node reconnects after being offline, it first synchronizes decisions newer than its last applied version.

```
Node reconnects

AppliedVersion = 1842
        │
        ▼
Synchronize newer decisions
        │
        ▼
1843 → 1844 → 1845 → ... → 1857
        │
        ▼
Apply locally
        │
        ▼
AppliedVersion = 1857
        │
        ▼
Subscribe to subsequent changes
```

This prevents a node from simply reconnecting to the live stream and assuming that it received every decision while offline.

---

## Failure Policy

Shield Mesh provides two explicit request-path failure policies.

**Fail Open**

If the enforcement state cannot be trusted or determined:
```
State valid       → enforce decision
State unavailable → allow
State corrupted   → allow
State ambiguous   → allow
```

**Fail Closed**

If the enforcement state cannot be trusted or determined:

```
State valid       → enforce decision
State unavailable → block
State corrupted   → block
State ambiguous   → block
```

Applications explicitly choose their policy.

```
shieldmesh.New(shieldmesh.Config{
	Transport: tr,
	Name: "node-1",
	FailPolicy: shared.FAILCLOSED,
})
```
The failure policy is part of the security model rather than an implicit behavior.

---

## Why Local Enforcement?

A conventional architecture might put a remote security service directly into the request path:

```
Request
   │
   ▼
Application
   │
   ▼
Security Service
   │
   ▼
Decision
```

This creates a dependency between request latency/availability and the security service.

Shield Mesh instead uses:
```
             Intelligence
                  │
                  ▼
              Decision
                  │
                  ▼
          Local Enforcement
                  │
                  ▼
Request ──────► Check()
                  │
                  ▼
             Application
```

The request path can therefore continue using the latest trusted local state even when the intelligence plane is temporarily unavailable.

---

## Security Model

Security decisions are security-sensitive state.

Shield Mesh is designed around several principles:

**Authentication**

Remote engines and transport participants must be authenticated before participating in the decision system.

**Authorization**

Authentication alone does not grant permission to publish arbitrary decisions.

Transport-level authorization determines what a participant can publish or consume.

**Version validation**

Nodes must validate decision versions before applying state transitions.

**Expiration**

Decisions may contain expiration timestamps.

Expired decisions must not remain enforceable indefinitely.

**Integrity**

Decision state must be validated before it is accepted into the local enforcement state.

**Idempotent application**

Decision updates may be delivered more than once.

Applying the same valid decision multiple times must not corrupt enforcement state.

**No request-path dependency**

Normal *Check()* evaluation does not depend on a remote intelligence engine being reachable.

---

## What Shield Mesh Is Not

Shield Mesh is not intended to be:

- a WAF
- an IDS
- a SIEM
- a threat-intelligence provider
- a machine-learning platform
- a centralized authorization service
- a database
- a replacement for security engines

Instead, it provides the decision distribution and local enforcement layer between security intelligence and applications.

---

### Example

A security engine detects suspicious activity:
```
User: user-123
Event: repeated authentication failures
```

The engine generates:
```
Decision:
    Subject: user-123
    Action: BLOCK
    Version: 1842
    ExpiresAt: ...
```
Shield Mesh distributes the decision.

The application node applies it locally:

```
Local State

user-123 → BLOCK
version  → 1842
```
A subsequent request:
```
user-123 → Check()
```
is evaluated locally.

The request does not need to contact the remote security engine.

---

## Design Goals

Shield Mesh is being built around the following goals:

- Low-latency request-path enforcement
- Asynchronous security intelligence
- Remote and independently deployable engines
- Local enforcement state
- Explicit failure semantics
- Versioned decision synchronization
- Transport abstraction
- Durable decision recovery
- Simple developer-facing APIs
- Observable and reproducible performance
- Strong security boundaries

---

## Non-Goals

The initial project intentionally avoids:

- Embedded security engines
- Synchronous remote decision evaluation
- Kubernetes-specific architecture
- Built-in ML/AI
- Security-intelligence marketplace functionality
- Custom database implementation in the initial release
- Custom consensus implementation in the initial release

The goal is to establish a small, understandable core before expanding the system.

---

## Current Status

Early development.

The architecture and protocol are being developed before the initial implementation is finalized.

The project will include:

- Shield Mesh SDK
- NATS transport And Custom ShieldMesh transport
- Local decision state
- Decision synchronization
- Failure-policy enforcement
- Remote engine integration
- Security tests
- Performance benchmarks
- Recovery and failure testing

Performance claims will be published only after reproducible benchmarks are available.

---

## Roadmap

### Phase 1 — Core

- [ ] Core SDK
- [ ] "Observe()"
- [ ] "Check()"
- [ ] Decision model
- [ ] Local enforcement state
- [ ] Fail-open / fail-closed behavior
- [ ] Versioning

### Phase 2 — NATS

- [ ] NATS transport
- [ ] JetStream integration
- [ ] Request/event retention
- [ ] Decision retention
- [ ] Decision synchronization
- [ ] Reconnection and recovery

### Phase 3 — Security

- [ ] Transport authentication
- [ ] Authorization
- [ ] Decision integrity
- [ ] Expiration validation
- [ ] Replay protection
- [ ] State corruption handling

### Phase 4 — Custom Transport

- [ ] Shield Mesh native transport
- [ ] Durable state
- [ ] Replication
- [ ] Snapshot/recovery

### Phase 5 — Evaluation

- [ ] Request-path latency benchmarks
- [ ] Throughput benchmarks
- [ ] Decision propagation benchmarks
- [ ] Recovery benchmarks
- [ ] Intelligence-plane overload tests
- [ ] Memory/CPU evaluation
- [ ] Reproducible benchmark reports

---

## Performance

Shield Mesh is designed for low-latency local enforcement.

The most important performance property is:

**Remote security intelligence should not add a network round trip to every application request.**

Performance will be evaluated using reproducible benchmarks covering:

- "Check()" latency
- p50 / p95 / p99 latency
- throughput
- concurrency scaling
- memory usage
- decision synchronization latency
- recovery throughput
- intelligence-plane overload
- request-path behavior during transport failure

Benchmark results will include the hardware, operating system, Go version, Shield Mesh version, configuration, and test methodology.

No performance number is considered a project guarantee until it has been measured and reproduced.

---

## Contributing

Contributions are welcome.

Before implementing a major feature, please open an issue describing:

1. The problem being solved
2. Why it belongs in ShieldMesh
3. The proposed architectural change
4. Security implications
5. Performance implications
6. Compatibility considerations

Shield Mesh prioritizes a small and understandable core over feature accumulation.

---

## License

License information will be added before the first public release.

---

## Project Philosophy

Shield Mesh follows one central principle:

- *Security intelligence can be remote. Security enforcement should be as close to the request as possible.*

The intelligence plane can evolve, scale, fail, and recover independently.

The application request path should remain fast, predictable, and locally enforceable.
