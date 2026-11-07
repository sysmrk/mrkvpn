# mrkvpn roadmap

## Phase 0 — design and repository foundation
- Architecture baseline and trust boundaries
- Threat model and measurable security acceptance criteria
- Transport adapter, logical session and gateway API contracts
- Platform compatibility matrix and test harness design

## Phase 1 — secure direct-IP alpha
- Linux gateway and direct-IP WireGuard
- Windows/Linux clients
- Per-device enrollment and revocation
- Full-tunnel IPv4/IPv6, DNS, OS-specific kill switch
- Failure injection and leak tests

## Phase 2 — adaptive transport research
- Health telemetry and automatic/manual/preferred path selection
- QUIC transport prototype and comparative benchmarks
- Connection standby and failover with explicit session continuity limitations

## Phase 3 — endpoint resilience
- Multiple independent entry points and signed endpoint discovery
- Bridge support, anti-probing review and blocked-endpoint recovery tests

## Phase 4 — optional Cloudflare relay
- HTTPS/WSS relay prototype, end-to-end payload encryption
- Measure reachability, throttling, latency, connection lifetime and cost
- Preserve complete direct-IP operation without Cloudflare

## Phase 5 — management
- Admin API and panel for users, devices, gateway inventory and policy
- MFA/passkeys, RBAC, audit, quotas, expiration, backups
- Authentication and key management basics must already exist in Phase 1

## Phase 6 — advanced multipath
- Flow-aware active-active scheduling
- Explore packet-level multipath, deduplication, reordering and congestion control
- Session migration only after rigorous state and security testing

## Phase 7 — production readiness
- Security review, fuzzing, performance and battery benchmarks
- Cross-platform test matrix, recovery, upgrades, HA and observability

Each phase requires demonstrated behavior; a roadmap item is not an implemented feature.
