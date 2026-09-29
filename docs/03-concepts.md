# 03 - Core Concepts & Terminology

To maintain architectural clarity, ShieldMesh relies on a strict internal vocabulary.

## 1. The Data Entities

The data entities represent the flow of information through the Shield Mesh intelligence pipeline.

### Subject

**Definition:** A standardized representation of an identity or entity being evaluated, signaled, or enforced against. ShieldMesh operates on Subjects, preventing it from devolving into a simple "IP blocking" library.

*   **Examples:** `ip: 203.0.113.10`, `user: usr_123abc`, `api_key: key_xyz` etc.

### Request

**Definition:** An immutable fact about something that happened in the application at a specific point in time. It usually contains the subject.

*   **Examples:** `login, subject, resource, method, path`.


### Decision

**Definition:** An actionable security instruction, bound by a Time-To-Live (TTL), that dictates exactly how the application must handle a specific Subject. Decisions are explicitly versioned (e.g., `Version: 1842`). This versioning is required so that enforcement nodes can determine which exact state they have applied and cleanly recover from missed updates. Decisions are the final output of the intelligence pipeline.

*   **Examples:** `BLOCK IP 192.168.1.50 for 15 minutes`, `ALLOW User 456`, `CHALLENGE Session 789`.

---

## 2. The System Entities

The system entities represent the structural components that move, evaluate, and enforce the data.

### Node (Application Node)

**Definition:** An application instance running the ShieldMesh SDK runtime. The Node is responsible for emitting Events via `Observe()` and evaluating requests against the Enforcement State via `Check()`.

### Enforcement State (Local State)

**Definition:** The locally materialized, highly optimized memory cache of active Decisions residing directly within a Node. When an application calls `Check()`, it queries the Enforcement State. This state is continuously synchronized in the background.

### Engine

**Definition:** A decoupled component that consumes data from the Fabric and produces intelligence.
Engines run out-of-process as remote external components anywhere on the network.


### Fabric

**Definition:** The distributed environment through which independent Shield Mesh participants (Nodes and Engines) exchange security intelligence. When a Node connects to the Fabric, its local state becomes part of a global, eventually consistent security mesh.

### Transport

**Definition:** The underlying message broker and delivery mechanism used to move messages across the Fabric. The Transport is strictly an implementation detail (e.g., NATS JetStream or the native ShieldMesh Custom Transport) and is completely abstracted away from the application developer.

---

## 3. The Conceptual Pipeline Summary

To see how the vocabulary works together in a sentence:

> A **Node** observes a **Request** and publishes it to the **Fabric** via the **Transport**. An **Engine** consumes the Request, identifies a pattern, and issues a **Decision** to block the **Subject**. The Decision propagates via the Transport to all Nodes, which update their **Enforcement State**, immediately dropping subsequent requests.
