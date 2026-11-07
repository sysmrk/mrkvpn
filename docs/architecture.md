# mrkvpn architecture baseline (Phase 0)

## Scope and non-goals
Private alpha (5–10 users). Cross-platform design, initially Linux gateway and Windows/Linux clients. No production-readiness claim. Multipath packet striping, transparent session migration, and distributed control plane are **not** v0.1 commitments.

## Logical components
1. **Client**: OS-specific VPN/TUN adapter, route/DNS/firewall enforcement, local credential storage, connection manager, health monitor, transport adapters.
2. **Gateway**: authenticated tunnel termination, peer/session mapping, policy enforcement, packet forwarding, IPv4/IPv6 egress and metrics.
3. **Control plane**: user/device enrollment, device key lifecycle, gateway configuration, revocation, policies, admin API and audit log.
4. **Optional relays/bridges**: independent entry points to the gateway. Relays must not need plaintext user payloads or client VPN private keys.

Control and data planes are separate trust boundaries. Existing authorized sessions may continue through a bounded control-plane outage, subject to explicit revocation/credential lifetime policy.

## Transport strategy
- **v0.1**: direct-IP WireGuard, authenticated device peers.
- **Research**: independent QUIC-based IP packet tunnel with its own security design; QUIC is not a drop-in WireGuard transport.
- **Research**: HTTPS/WSS relay, optionally using Cloudflare; does not imply native UDP WireGuard forwarding via Workers.
- **Later**: transport health evaluation and standby/failover, then flow-aware active-active scheduling.
- **Advanced research**: packet-level multipath requires common authenticated session framing, packet ordering/deduplication, replay protections, gateway coordination and congestion management.

A single physical uplink limits total throughput even with several transports. Multiple transport connections on the same IP share an IP-blocking failure domain.

## Fail-closed requirements
All traffic policy must be enforced outside the application process when the OS allows it. No silent direct routing on tunnel failure, startup, crash, transport switch or sleep/resume. Treat IPv4, IPv6, DNS and local network exceptions separately. A kill switch may not cover every platform scenario; each must be tested.

## Endpoint resilience
Maintain a small set of independent endpoints. Signed endpoint manifests, embedded trust roots, expiry/rollback protection and safe bootstrap behavior are design requirements. Domainless operation is mandatory for direct-IP mode, not for Cloudflare HTTPS hostnames.

## Preliminary repository structure
```text
apps/             # platform clients (future)
core/             # session, transport, policy, health, config (future)
gateway/          # tunnel termination and egress (future)
control/          # admin API and identity (future)
docs/             # architecture, threat model, ADRs, roadmap
tests/            # integration, security, performance (future)
```

## Open architecture questions
- How should WireGuard identity relate to a future transport-independent logical session?
- What exact platform-specific firewall/DNS lifecycle guarantees can be achieved?
- How do signed endpoint manifests bootstrap during prolonged censorship?
- Which transport fallback preserves acceptable latency, battery use and end-to-end encryption?
- Which data must be retained for operational metrics while minimizing user privacy exposure?
