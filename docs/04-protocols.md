# 04 - Protocol & Data Model

This document defines the wire-level representations and semantic rules of the ShieldMesh protocol. By formalizing these payloads independent of any specific programming language, we ensure that a Node written in Go, an Engine written in Python, and a Transport backed by NATS can interoperate flawlessly.

## 1. The Message Envelope

Every payload transmitted across the Shield Mesh Fabric is wrapped in a standard `Message` envelope. This allows the Transport layer to route and filter data without needing to deserialize the inner payload.

```json
{
  "id": "msg_01h62v...",
  "type": "request",
  "schema_version": "v1",
  "source": "node_web_01",
  "timestamp": "2026-09-26T06:30:00Z",
  "payload": { ... }
}
```

* **`id`**: A unique identifier (e.g., ULID or UUIDv7) for tracing and deduplication.
* **`type`**: Must be `request`, or `decision`.
* **`schema_version`**: Allows future protocol evolution without breaking backwards compatibility.
* **`source`**: The identifier of the Node or Engine that produced this message.
* **`timestamp`**: UTC time of message generation.

---

## 2. The Core Schemas
The payload field of the Message envelope contains one of the following JSON structures.

### Subject

Subjects are embedded inside Request, and Decisions. They define who or what the payload is about.
{
  "type": "ip",
  "id": "203.0.113.10"
}

### Request

Emitted by Application Nodes to describe something that happened.

```json
{
  "id": "evt_abc123",
  "subject": {
    "type": "ip",
    "id": "203.0.113.10"
  },
  "resource": "/api/v1/login",
  "timestamp": "2026-09-26T06:30:00Z",
  "metadata": {
    "username": "admin",
    "user_agent": "curl/7.68.0"
  }
}
```


### Decision

Emitted by Policy Engines to dictate enforcement state on the Application Nodes. It includes an explicit version field so enforcement nodes can determine which state they have applied.

```json
{
  "id":"dec_3884",
  "action": "BLOCK",
  "subject": {
    "type": "ip",
    "id": "203.0.113.10"
  },
  "metadata": {
    "tenant_id" : "838"
  },
  "version": 1842,
  "ttl_seconds":"60",
  "reason": "Automated block: Repeated credential stuffing",
  "source": "shieldmesh_policy_engine",
  "confidence": 0.94,
  "issued_at": "2026-09-26T06:30:06Z",
  "expires_at": "2026-09-26T07:30:06Z"
}
```

---

## 3. Protocol Semantics & Rules

To maintain consistency across a distributed fabric, all participants must adhere to the following protocol rules.

### Delivery & Idempotency

* **At-Least-Once Delivery** *: The Fabric guarantees that a message will be delivered at least once, meaning Application Nodes and Engines must expect duplicate messages.

* **Idempotent Application**: When a Node receives a Decision, it must apply it idempotently. Upserting a decision into the local sync.Map cache is naturally idempotent as long as TTLs are respected. Applying the same valid decision multiple times must not corrupt enforcement state.


### Ordering & Out-of-Order Delivery

* Messages may arrive out of order due to network latency or Transport retries.
* **State Updates**: If a Node receives a `Decision` for a Subject, it must check the version (or sequence number/issued_at timestamp). A Node must never overwrite an existing decision with an older decision that arrived late.

### State Retention Policies

To provide bounded windows for processing and node recovery, the underlying transport layer maintains explicit retention periods:

* **Security Telemetry (Requests/Events)** : Retained for a default period of 24 hours.
* **Decision State**: Retained for a default period of 7 days to ensure enforcement nodes can recover missed updates if they temporarily disconnect.


### Freshness & Expiration (TTL)
* **Explicit Expiration**: Every Decision must include an expires_at timestamp. Permanent blocks (no expiration) are heavily discouraged but can be represented by omitting the field.

* **Clock Sync**: Because decisions rely on absolute UTC timestamps for expiration, all Nodes and Engines must rely on NTP-synchronized clocks. A Node must proactively evict decisions from its local state once time.Now() > expires_at.


### Identity & Authority Binding
* **Source Tracking**: The source field tracks which engine issued a Decision.

* **Protocol Trust**: At the protocol layer, the source is just a string. However, the Node's materializer must validate that the source is authorized to issue the action before writing it to local memory.

* *Note: Transport-layer authentication validates the identity of the connection, but the Protocol layer dictates the authority of the payload.*

### Replay Prevention.

* A Node should silently discard a `Decision` if its `expires_at` time is already in the past upon arrival.
