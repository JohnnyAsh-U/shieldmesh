# 06 - Consistency and Failure Semantics

Because ShieldMesh decouples intelligence from enforcement using an asynchronous fabric, it is subject to the realities of distributed systems: network partitions, broker outages, and clock drift. This document explicitly defines Shield Mesh’s consistency model and how Application Nodes must behave when the intelligence plane degrades.

## 1. The Consistency Spectrum

ShieldMesh does not offer Strong Consistency (where every read is guaranteed to see the most recently written value across the entire cluster). Attempting to do so would violate our sub-microsecond local enforcement mandate. Instead, we map security states to different consistency guarantees.

| State | Semantics | Definition |
| --- | --- | --- |
| **Local Enforcement State** | *Strongly Consistent (Local)* | Immediately available to the Node. A `Check()` always reflects exactly what is in the Node's memory at that exact nanosecond. |
| **Global Enforcement** | *Eventual Consistency* | A Decision made by an Engine will eventually propagate to all Nodes, provided the Fabric is healthy. |
| **Security Freshness** | *Bounded Staleness* | An authorization or block decision is permitted to be slightly out of date, but *only* within a defined temporal limit (TTL). If the limit is crossed, the data is evicted. |
| **Cross-Node Quotas** | *Best-Effort* | ShieldMesh is not a distributed lock manager. Global rate-limiting relies on eventually consistent aggregation and may slightly overshoot during high concurrency. |

---

## 2. Failure Scenarios & Resolution

A Node must remain resilient when the Fabric fails. Here is exactly how the runtime resolves distributed system edge cases.

### Engine Unavailable

*   **Scenario:** A remote Engine crashes and stops producing Decisions.
*   **Resolution:** The Node continues enforcing its current state. Existing Decisions remain active until their `expires_at` TTL is reached. New threats that rely solely on that Engine will go undetected, but the application remains fully available.

### Broker / Transport Unavailable

*   **Scenario:** The Transport cluster goes down or the Node loses network connectivity to the Fabric.
*   **Resolution:**
1.  `Check()` continues using the Local Enforcement State without interruption.
2.  `Observe()` must not block the application. Depending on configuration, the Transport adapter will either buffer Events in memory (up to a limit) or drop them entirely to prevent OOM errors.
3.  No new Decisions arrive. Existing Decisions eventually expire via their TTLs.

### Node Restart & Reconnection

*   **Scenario:** The Application Node scales down, restarts, or reconnects after a prolonged network partition.
*   **Resolution:** Shield Mesh utilizes Decision Sequence Synchronization.
1.  An enforcement node maintains the version of the latest decision state it has successfully applied (e.g., `AppliedVersion = 1842`).
2.  When the node reconnects, it does not simply subscribe to the live stream. Instead, it first synchronizes decisions sequentially from the transport that are newer than its last applied version.
3.  Missed updates can be successfully recovered because the NATS JetStream transport or ShieldMesh fabric retains Decision state for a default period of **7 days**.
4.  Once the node applies these historical decisions locally and updates its `AppliedVersion`, it subscribes to subsequent live changes.

### Duplicate Decisions

*   **Scenario:** At-least-once delivery semantics cause the Node to receive the exact same Decision twice.
*   **Resolution:** Decisions are inherently idempotent. The Node overwrites the existing map entry. Since the `expires_at` and `action` are identical, the enforcement outcome remains unchanged.

### Out-of-Order Decisions

*   **Scenario:** An Engine issues an `ALLOW` at T1, and a `BLOCK` at T2. Due to a network retry, the Node receives the `BLOCK` first, and the `ALLOW` second.
*   **Resolution:** The Node's state materializer must evaluate the decision's explicit `sequence` or `issued_at` timestamp. If a newly arrived message has a version older than the currently cached state for that Subject, it is silently discarded.

### Expired Decisions (Replay Attacks)

*   **Scenario:** An attacker captures an old `ALLOW` Decision on the wire and replays it to the Fabric days later.
*   **Resolution:** The Node evaluates the `expires_at` timestamp against its local clock. If `time.Now() > expires_at`, the message is instantly rejected.

### Conflicting Decisions

*   **Scenario:** Engine A issues a `BLOCK` for an IP. Engine B simultaneously issues an `ALLOW` for the same IP.
*   **Resolution:** ShieldMesh resolves conflicts strictly by **Action Weight**. By default: `BLOCK` > `CHALLENGE` > `ALLOW`. The most restrictive action always wins in Local Enforcement State.

---

## 3. Explicit Failure Policies

When a Node cannot definitively determine the state of a Subject (e.g., if a Decision is corrupted, or if the user explicitly configures a strict remote-check requirement that fails), the application developer must be in control of the fallback behavior. The failure policy is part of the security model rather than an implicit behavior.

ShieldMesh exposes strict `FailurePolicy` configurations at the SDK level.

*   **`FailOpen`:** If the enforcement state is unavailable, corrupted, or ambiguous, the request is allowed. This prioritizes business continuity and availability over absolute security.
*   **`FailClosed`:** If the enforcement state is unavailable, corrupted, or ambiguous, the request is immediately blocked. This prioritizes absolute security but risks application downtime during internal outages.

---

## 4. Freshness and Clock Synchronization

Because Shield Mesh relies on Bounded Staleness rather than synchronous locks, **time is a critical security primitive.**

1.  **Mandatory TTLs:** Every `Decision` injected into the Fabric must contain an `expires_at` timestamp. Expired decisions must not remain enforceable indefinitely.
2.  **NTP Requirement:** To run ShieldMesh in a distributed architecture, all participating host machines must have NTP (Network Time Protocol) synchronization enabled. The ShieldMesh runtime will warn on startup if significant local clock skew is detected during transport handshakes.
