# Shield Mesh

> [!WARNING]
> **Experimental Stage / MVP Phase**
> Shield Mesh is currently in an experimental phase. APIs, transport protocols, and internal architecture may change or break as we continue to test and harden the system.
**Distributed security intelligence. Local enforcement.**

[![Go Report Card](https://goreportcard.com/badge/github.com/JohnnyAsh-U/shieldmesh)](https://goreportcard.com/report/github.com/JohnnyAsh-U/shieldmesh)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Shield Mesh is a distributed security decision fabric that connects remote security intelligence engines to applications through low-latency local enforcement.

It allows heavy, asynchronous security intelligence (WAFs, Threat Intel, SIEM, ML) to operate out-of-band, while continuously projecting decisions into a sub-microsecond local enforcement cache on the application node.

## v0.1.0 Release

Shield Mesh v0.1.0 provides a sub-microsecond local enforcement cache and supports distributed security intelligence through NATS JetStream.

### Key Features
- **Core SDK & Middleware:** Seamless HTTP handler wrapping.
- **NATS Transport:** Built-in JetStream support for telemetry and decision sync.
- **Local Enforcement Cache:** Sub-microsecond evaluations without network round trips.
- **Fail Policies:** Explicit `FailOpen` or `FailClosed` semantics.
- **Standalone Proxy (`shieldd`):** Run as a reverse proxy or NGINX auth server without changing application code.

## Installation

**SDK:**
```bash
go get github.com/JohnnyAsh-U/shieldmesh
```

**Standalone Proxy:**
```bash
go install github.com/JohnnyAsh-U/shieldmesh/cmd/shieldd@latest
```

## Quick Start

### 1. Using the SDK Middleware

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/JohnnyAsh-U/shieldmesh"
	"github.com/JohnnyAsh-U/shieldmesh/shared"
	"github.com/JohnnyAsh-U/shieldmesh/transport"
)

func main() {
	// Connect to NATS JetStream
	tr, err := transport.NewNatsTransport("nats://localhost:4222")
	if err != nil {
		log.Fatal(err)
	}

	// Initialize Shield Mesh
	shield := shieldmesh.NewShieldMesh(shieldmesh.Config{
		Transport:  tr,
		Name:       "api-node-1",
		FailPolicy: shared.FAILOPEN,
	})

	// Sync background intelligence
	shield.Start(context.Background())
	defer shield.Stop()

	// Define your app
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Protected by Shield Mesh!"))
	})

	// Wrap app with Shield Mesh
	http.ListenAndServe(":8080", shield.Middleware(app))
}
```

### 2. Running the Standalone Proxy

Acts as a full sidecar reverse proxy protecting your application:

```bash
shieldd --listen :8080 --target http://localhost:8081 --nats nats://localhost:4222 --mode proxy
```

**NGINX Auth Request Mode:**
Integrates seamlessly with NGINX's `auth_request` module. In this mode, `shieldd` evaluates incoming headers (`X-Forwarded-For` or `X-Real-IP`) and responds with `200 OK` (allow) or `403 Forbidden` (block), without actually proxying the HTTP body.

```bash
shieldd --listen :8080 --nats nats://localhost:4222 --mode auth
```

*Nginx configuration example:*
```nginx
location / {
    auth_request /shieldmesh_auth;
    proxy_pass http://your_upstream;
}

location = /shieldmesh_auth {
    proxy_pass http://localhost:8080;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header X-Real-IP $remote_addr;
}
```

## Documentation

Comprehensive documentation is available in the [`docs/`](./docs) directory:

- [00 - Overview & Architecture](./docs/00-overview.md)
- [04 - Protocols](./docs/04-protocols.md)
- [06 - Consistency and Failure](./docs/06-consistency-and-failure.md)

## Current Status

**Phase 1 & 2 Completed:** Core SDK, Observe/Check API, Local Enforcement State, NATS JetStream transport, and `shieldd` proxy are production-ready for v0.1.0.

## Contributing

Contributions are welcome. Please open an issue describing the problem and proposed architectural changes before implementing major features.
