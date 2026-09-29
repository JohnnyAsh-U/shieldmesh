# 07 - Threat Model & Security Model

Because ShieldMesh is a distributed intelligence fabric, it introduces a unique security paradox: **the system designed to protect the application is itself a high-value target.** If an attacker can compromise the intelligence plane, they could theoretically issue a global `ALLOW` to bypass security, or a global `BLOCK` to execute a devastating Denial of Service (DoS) against legitimate users.

This document outlines the trust boundaries, the critical distinction between identity and authority, and the specific threat vectors Shield Mesh is designed to mitigate.

---

## 1. Trust Boundaries

Shield Mesh operates across distinct zones of trust. Assuming the underlying network is hostile is a fundamental requirement of the architecture.

**Zone 1: Application to Node (Trusted)**
*   Communication between the application HTTP handler and the Shield Mesh SDK (`Check()` and `Observe()`) happens entirely in-process. This is a trusted boundary.

**Zone 2: Node to Transport (Untrusted)**
*   The connection from the Application Node to the Fabric (e.g., NATS or Custom Transport) crosses the network. This boundary is untrusted and vulnerable to eavesdropping and Man-in-the-Middle (MitM) attacks.

**Zone 3: Transport to Engine (Untrusted)**
*   Engines connecting to the Fabric to consume Requests and publish Decisions operate in untrusted zones, potentially running in different clusters, VPCs, or even third-party networks.

---

## 2. Authentication (AuthN) vs. Authorization (AuthZ)

The most dangerous assumption in a distributed security system is conflating identity with authority. Shield Mesh explicitly separates the two.

### Authentication (Transport Layer)

*   **Mechanism:** Mutual TLS (mTLS) or Broker Native Credentials (e.g., NATS NKEYS).
*   **Meaning:** mTLS guarantees that the connection is encrypted and proves the identity of the client. Remote engines and transport participants must be authenticated before participating in the decision system.

### Authorization (Protocol Layer)

*   **Mechanism:** Protocol-level Capability Schemas and Payload Validation.
*   **Meaning:** Authentication alone does not grant permission to publish arbitrary decisions. Transport-level authorization determines what a participant can publish or consume. A Node's state materializer must explicitly authorize the payload.


---

## 3. Threat Scenarios & Mitigations


### Threat 1: Replay Attacks

*   **Attack:** An attacker captures a valid, highly privileged `ALLOW` Decision granting access to a specific IP. Days later, after that IP has been blocked for malicious behavior, the attacker replays the captured `ALLOW` message to the Fabric.
*   **Mitigation:** Every `Decision` contains a strict `expires_at` timestamp. By the time the attacker replays the message, the Node evaluates `time.Now() > expires_at` and rejects the payload.

### Threat 2: Decision Tampering (Man-in-the-Middle)

*   **Attack:** An attacker intercepts a `BLOCK` Decision on the wire and alters the payload to `ALLOW` before it reaches the Node.
*   **Mitigation:** Transport-layer encryption (mTLS) prevents in-transit modification. Furthermore, decision state must be validated for integrity before it is accepted into the local enforcement state.

### Threat 3: Fabric DoS via Request Flooding

*   **Attack:** An attacker spams the application's login endpoint, generating millions of `authentication.failed` Request per second, intending to exhaust the Node's memory or crash the transport broker.
*   **Mitigation:**
1.  `Observe()` is strictly non-blocking.
2.  The Node implements bounded local outbound buffers (backpressure). If the buffer fills because the Fabric cannot keep up, the Node **drops outbound Events** rather than crashing. Security telemetry is lost, but the application remains online and Local Enforcement State continues to function.

### Threat 4: State Corruption

*   **Attack:** A software bug or targeted payload manipulation results in corrupted enforcement state being written to the node's memory.
*   **Mitigation:** If the integrity of the decision state is compromised or determined to be corrupted, the node will defer to the developer's explicitly chosen Failure Policy (Fail Open or Fail Closed) to handle incoming requests securely and predictably.

---

## 4. The Security Invariants

When deploying or modifying Shield Mesh, the following security invariants must be maintained:

1.  **Zero Trust Intelligence:** Nodes must never blindly trust a Decision just because it arrived via the Fabric. Decisions must be validated for Freshness (issued_at) and Version.
2.  **Version Validation:** Nodes must validate decision versions before applying state transitions to ensure chronological accuracy and prevent state regression.
3.  **Integrity:** Decision state must be strictly validated before it is accepted into the local enforcement state.
4.  **Idempotent Application:** Decision updates may be delivered more than once. Applying the same valid decision multiple times must not corrupt the local enforcement state.
5.  **Ephemerality:** Permanent blocks (Decisions without an `expires_at`) are an anti-pattern. Expired decisions must not remain enforceable indefinitely.
6.  **Availability > Telemetry:** If the choice is between crashing the application or dropping security telemetry, Shield Mesh will always drop telemetry.
