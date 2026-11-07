# mrkvpn preliminary threat model

**Status:** draft. This is not a completed security assessment.

## Assets
User payload confidentiality and integrity; client and gateway private keys; user/device identities; control-plane credentials; endpoint manifest authenticity; routing and DNS policy; audit integrity.

## Adversaries
- Passive network observer and traffic analyst.
- Network operator performing DPI, throttling, UDP blocking, DNS interference, active probing or IP blocking.
- Malicious or compromised relay.
- Stolen client device or credential.
- Compromised gateway/control-plane operator account.
- Malicious network on Wi-Fi or captive portal.

## Trust boundaries
Client OS and privileged networking component; encrypted transport; untrusted relay/CDN; gateway; management API; persistence and backup.

## Baseline mitigations
- Vetted cryptographic primitives and authenticated endpoints; no bespoke crypto.
- Device-specific credentials with revocation and rotation plan.
- Strong administrator authentication (passkeys/WebAuthn or MFA), least-privilege RBAC, audited administrative actions.
- Enforce fail-closed routing/DNS behavior with explicit, minimal endpoint exceptions.
- Preserve end-to-end payload encryption through third-party relays.
- Signed, freshness-checked endpoint configuration and anti-rollback controls.
- Rate limits and minimal unauthenticated gateway responses.
- Secrets excluded from logs, source control and diagnostic bundles.
- Reproducible test scenarios for reconnect, path loss, client crash, sleep/resume, DNS changes, IPv6, UDP blocking and endpoint blocking.

## Residual risks
Traffic metadata remains visible to the access network and possibly the CDN; no claim of undetectability. A single reachable endpoint can be blocked. Some mobile OS behaviors limit strict fail-closed guarantees. Multiple transports over one ISP are correlated failures. A compromised endpoint or gateway may expose plaintext at termination.

## Acceptance evidence needed
Per-platform packet captures demonstrating no unintended direct egress during fault injection; key-revocation tests; admin authorization tests; transport integrity tests; relay trust tests; latency/loss/throughput/battery benchmarks.
