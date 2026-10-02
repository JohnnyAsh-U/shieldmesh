# Shield Mesh: Overview

**Distributed security intelligence. Local enforcement.**

Shield Mesh is a distributed security decision fabric that connects remote security intelligence engines to applications through low-latency local enforcement.

## The Problem

Modern web applications face a fundamental architectural dilemma when implementing security: the conflict between **enforcement latency** and **intelligence depth**.

To catch sophisticated attacks—such as distributed credential stuffing, slow-drip API abuse, or complex bot behavior—security systems require heavy compute, sliding-window aggregations, or machine learning.

*   If you put this intelligence **inline** on the request path (like a traditional WAF), it introduces unacceptable latency (20-100ms+) and creates massive single points of failure for the application.
*   If you move this intelligence **out-of-band** (like a SIEM or batch log processor), the analysis takes minutes or hours, meaning the attacker successfully breaches the system long before a block is ever issued.

There is a missing architectural layer: a system that allows heavy, asynchronous security intelligence to operate out-of-band, while continuously projecting its decisions into a sub-microsecond local enforcement cache on the application node.

## The Solution

Shield Mesh separates security intelligence from the application request path:

- Applications emit security telemetry asynchronously.
- Remote security engines analyze that telemetry.
- Engines publish security decisions through the Shield Mesh transport.
- Application nodes maintain local enforcement state.
- Requests are evaluated locally without requiring a network round trip to an intelligence engine.

The goal is simple: **Let security intelligence be distributed without putting the intelligence plane on the request path.**

## Architecture

```text
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
```

The two planes are intentionally separated.

### Intelligence Plane
Responsible for:
- collecting security telemetry
- analyzing events
- generating security decisions
- distributing decisions
- retaining decision history for synchronization

### Enforcement Plane
Responsible for:
- maintaining local decision state
- evaluating requests
- enforcing decisions
- continuing to operate when the intelligence plane is unavailable

The enforcement plane does not query a remote engine for every request.

## Core API

Shield Mesh exposes two primary operations.

### "Observe"
Send security telemetry to the intelligence plane.
```go
err := shield.Observe(ctx, request)
```
"Observe" is asynchronous by design. It does not wait for a security engine to analyze the event or generate a decision.

### "Check"
Evaluate a request against locally available enforcement state.
```go
decision, err := shield.Check(ctx, request)
```
The request path uses local state. No remote intelligence engine needs to be contacted during normal request evaluation.

## Decisions

Shield Mesh currently has three enforcement actions: `ALLOW`, `BLOCK`, `CHALLENGE`.

A decision contains the information required to determine whether and how it should be enforced.

```go
type Decision struct {
    Subject    string
    Action     Action
    Reason     string
    Confidence float64
    IssuedAt   time.Time
    ExpiresAt  time.Time
}
```

Decisions use Stream Sequence so that enforcement nodes can determine which state they have applied and recover from missed updates.

## Remote Intelligence Engines

Shield Mesh does not embed security intelligence engines inside the SDK. Engines are external components (WAF, SIEM, ML, etc.). The engine produces intelligence; Shield Mesh distributes the resulting decisions to enforcement points.

## Transport & State Model

The architecture is designed around two transport implementations: NATS JetStream and a future Shield Mesh Custom Transport. Applications/Engines can use either.

The NATS implementation maintains separate state:
- **Requests:** Retained for a default of **24 hours**.
- **Decisions:** Retained for a default of **7 days**.

## Decision Synchronization

An enforcement node maintains the sequence of the latest decision state it has successfully applied. When the node reconnects after being offline, it first synchronizes decisions newer than its last applied version.

## Failure Policy

Shield Mesh provides two explicit request-path failure policies.

**Fail Open:** If the enforcement state cannot be trusted or determined, the request is allowed.
**Fail Closed:** If the enforcement state cannot be trusted or determined, the request is blocked.

The failure policy is part of the security model rather than an implicit behavior.

## Why Local Enforcement?

A conventional architecture might put a remote security service directly into the request path. This creates a dependency between request latency/availability and the security service. Shield Mesh instead projects decisions to a local enforcement layer. The request path can therefore continue using the latest trusted local state even when the intelligence plane is temporarily unavailable.

## Security Model

- **Authentication:** Remote engines and transport participants must be authenticated.
- **Version validation:** Nodes must validate decision sequence before applying state transitions.
- **Expiration:** Expired decisions must not remain enforceable indefinitely.
- **Integrity:** Decision state must be validated before it is accepted.
- **Idempotent application:** Applying the same valid decision multiple times must not corrupt enforcement state.
- **No request-path dependency:** Normal evaluation does not depend on a remote engine being reachable.

## Design Goals

- Local Request-Path Enforcement: Sub-microsecond evaluations.
- Asynchronous Security Intelligence: Out-of-band analysis.
- Pluggable Engines: Easily attach custom intelligence engines.
- Explicit Freshness Semantics: Guaranteed TTL/expiration for decisions.
- Explicit Failure Behavior: Developer-defined fallback scenarios.
- Transport Independence: Interchangeable underlying infrastructure.

## Non-Goals

Shield Mesh is intentionally **not**:
- A WAF, IDS, IPS, or SIEM
- An ML Platform
- An IAM Replacement or Global Rate-Limit Coordinator
- A centralized authorization service or a database
- Embedded security engines
- Synchronous remote decision evaluation

It provides the decision distribution and local enforcement layer between security intelligence and applications.
