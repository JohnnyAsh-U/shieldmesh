# Security Policy for AshMesh

AshMesh is a distributed security intelligence and enforcement fabric. Because it sits on the critical path of application execution, we take its security posture extremely seriously.

## Supported Versions

Only the latest minor release of the current major version receives security updates. 

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a vulnerability in AshMesh, **do not open a public issue.** 

Please email the core maintainers directly at `[Your Email/Security Alias]`. 
* Include a detailed description of the vulnerability.
* Provide a proof of concept (PoC) or steps to reproduce if applicable.
* Describe the potential impact (e.g., consensus hijacking, memory exhaustion, mTLS bypass).

We will acknowledge receipt within 48 hours and provide a timeline for triage and resolution. We will publicly disclose the vulnerability only after a patch has been merged and published.

## Threat Model Boundary
Please note that vulnerabilities arising from weak user-configured mTLS keys, compromised host Unix domain sockets (`/var/run/ashmesh.sock`), or misconfigured application SDKs fall outside the AshMesh core threat model.