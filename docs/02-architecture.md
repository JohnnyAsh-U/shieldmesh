# 02 - Architecture

This document defines the high-level architecture of Shield Mesh. It outlines the core data flow, the deployment topologies, and the inviolable rules that govern the system's design.

```text
                         Security Intelligence Plane

              ┌──────────────────────────────────────┐
              │          Remote Engines              │
              │                                      │
              │  WAF · Threat Intel · SIEM · ML      │
              │  Behavioral Detection · Custom       │
              └──────────────────┬───────────────────┘
                                 │
                                 │ Decisions (Versioned)
                                 ▼
                        ┌─────────────────┐
                        │  Shield Mesh    │
                        │    Transport    │
                        │                 │
                        │ NATS / Custom   │
                        └────────┬────────┘
                                 │
                                 │ Synchronization (AppliedVersion)
                                 ▼
                    ┌────────────────────────┐
                    │   Local Enforcement    │
                    │         State          │
                    └───────────┬────────────┘
                                │
                              Check()
                                │
                                ▼
┌───────────────────────────────┼────────────────────────────────────────┐
│   Incoming Request ──► [ Go HTTP Middleware ] ──(Allowed)──► Handler   │
│                              │                                         │
│                              └─► Async Observe() ──► Intelligence Plane│
└────────────────────────────────────────────────────────────────────────┘
```
## 1. The High-Level Abstraction
At its core, Shield Mesh isolates the application from the complexities of security analysis and intelligence. The application interacts with the Shield Mesh SDK through exactly two primary mechanisms: **Observe**(reporting telemetry) and **Check** (enforcing state).

```text
       APPLICATION
            │
            ├── Observe(Event) ─────────► (Asynchronous Fire-and-Forget)
            │
            └── Check(Request)
                     │
                     ▼
           Local Enforcement State
               (Sub-microsecond)
```

The application never waits for an engine to analyze an event, nor does it query a remote database to authorize a request. It only interacts with the locally materialized enforcement state.


## 2. The Intelligence Pipeline

Behind the SDK boundary, ShieldMesh routes data through a strict intelligence pipeline. We explicitly decouple detection from enforcement to ensure engines can express suspicion without accidentally taking down production traffic. Crucially, ShieldMesh does not embed security intelligence engines inside the SDK; all engines (WAF, SIEM, ML) are strictly remote, external components.

```text
       Request  (e.g., "login")
         │
         ▼
     [ Transport ] (NATS, ShieldMesh Custom fabric)
         │
         ▼
    [ Engine ] (Evaluates request against engine rules)
         │
         ▼
      Decision (e.g., "BLOCK IP 192.168.1.50 for 15 minutes", Version: 1842)
         │
         ▼
 [ Local State Synchronization ] (Sequential pull via AppliedVersion)
         │
         ▼
    Enforcement (The next Check() drops the request)
```

## 3. Deployment Topologies

ShieldMesh is designed to adapt its deployment topology to the application's scale without changing the application's programming model. The developer's code remains identical regardless of the transport used.

 * Bus: Single Node or Distributed Transport cluster (NATS JetStream or Shield Mesh Custom Transport).
 * Engines: Dedicated, independently scaled workers running anywhere in the network.
 * State: Local sync.Map cache. Synchronization relies on the node tracking its AppliedVersion; upon reconnection, it fetches all decisions sequentially newer than its last applied version before subscribing to live changes.
 ```
                       SHIELD MESH SDK
                              │
          ┌───────────────────┴───────────────────┐
          │                                       │
      Single Node                     Distributed cluster
          │                                       │
    Local Broker                          Remote Broker
          │                                       │
    Local Engines                          Remote Engines
          │                                       │
          └───────────────────────────────────────┘
                              │
                    SAME DEVELOPER API
```

## 4. Architectural Invariants (The "Sacred Rules")

To ensure Shield Mesh remains fast, secure, and decoupled, the following rules must never be broken during implementation or extension:

 * Request-path enforcement must not require a remote engine. The Check() function must evaluate exclusively against local memory. Network calls on the hot path are strictly forbidden.

 * Application code must not depend on engine identity or implementation. The application must never address a specific engine, wait for a specific engine, or know if an engine is written in Go, Python, or Rust.

 * Local enforcement state is authoritative for the node's request-path decision. If the network partitions, the node must continue enforcing its last-known local state until TTLs expire, dictated by the configured Failure Policy.

 * Remote intelligence is asynchronously materialized into local enforcement state. The transport layer is responsible for distributing Decisions down to the nodes. Nodes must validate decision versions and sequence them accurately using AppliedVersion logic.

 * Transport is an implementation detail. Transport implementation lives entirely behind the Transport interface.


