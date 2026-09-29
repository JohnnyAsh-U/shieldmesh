# 01 - Problem and Goals

## The Problem

Modern web applications face a fundamental architectural dilemma when implementing security: the conflict between **enforcement latency** and **intelligence depth**

To catch sophisticated attacks—such as distributed credential stuffing, slow-drip API abuse, or complex bot behavior—security systems require heavy compute, sliding-window aggregations, or machine learning 

*   If you put this intelligence **inline** on the request path (like a traditional WAF), it introduces unacceptable latency (20-100ms+) and creates massive single points of failure for the application.

*   If you move this intelligence **out-of-band** (like a SIEM or batch log processor), the analysis takes minutes or hours, meaning the attacker successfully breaches the system long before a block is ever issued.

There is a missing architectural layer: a system that allows heavy, asynchronous security intelligence to operate out-of-band, while continuously projecting its decisions into a sub-microsecond local enforcement cache on the application node.

## The Question

> Can a generic security-intelligence protocol decouple security analysis from request-path enforcement, allowing independently developed engines to contribute intelligence while heterogeneous enforcement nodes/middlewares maintain low-latency local decisions?

## Goals

Shield Mesh is designed to solve this dichotomy by adhering to the following design goals:

*   **Local Request-Path Enforcement:** Application nodes must evaluate security decisions (Check) in sub-microsecond time using entirely local memory, never blocking the HTTP request to wait on a network call.
*   **Asynchronous Security Intelligence:** Application nodes fire-and-forget telemetry (Observe). Heavy security analysis happens entirely out of the application's critical path.
*   **Pluggable Engines:** Developers can attach custom detectors (written in any language) to the intelligence plane without changing a single line of the application's code.
*   **Explicit Freshness Semantics:** Every security decision must have an explicit TTL/expiration to ensure nodes fail gracefully and don't permanently enforce stale intelligence.
*   **Explicit Failure Behavior:** The developer must be able to define exactly how the application behaves (Fail Open vs. Fail Closed) if the intelligence plane disconnects.
*   **Transport Independence:** The core application SDK must not leak infrastructure details. Transport is strictly an implementation detail utilizing either NATS (JetStream) or the native Shield Mesh Custom Transport.

## Non-Goals

To prevent scope creep and ensure Shield Mesh remains a pure security runtime, it is explicitly **NOT**:

*   **Not a WAF:** It does not ship with built-in regex rules for SQLi/XSS. (Though a WAF can be built *as an external Engine* on top of Shield Mesh).
*   **Not an IDS/IPS:** It operates at Layer 7 (Application logic), not Layer 3/4 (Packet inspection).
*   **Not a SIEM:** It routes events for real-time intelligence, but it is not a long-term data lake or log storage query engine.
*   **Not an ML Platform:** It does not host, train, or execute ML models—it provides the fabric for external ML models to subscribe to events and publish decisions. Built-in ML/AI is intentionally avoided.
*   **Not an IAM Replacement:** It does not issue JWTs, handle OAuth, or replace your identity provider.
*   **Not a Global Rate-Limit Coordinator:** It is designed for security intelligence, not guaranteeing precise global API quota limits across a distributed cluster.
*   **Not a Consensus System:** It relies on underlying transports for delivery guarantees; it does not implement a custom consensus algorithm in its initial release.
*   **Not a Distributed Database:** It materializes temporary enforcement state; it is not the source of truth for durable application data and avoids custom database implementations in the initial release.
*   **No Embedded Engines:** It intentionally avoids embedding security engines directly within the SDK or the application process.
*   **No Synchronous Remote Evaluation:** Normal request evaluation will never depend on synchronous remote decision evaluation.

Are you ready for 02-architecture.md?
