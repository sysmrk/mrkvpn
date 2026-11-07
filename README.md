# mrkvpn

**mrkvpn** is an experimental, security-first, cross-platform private VPN project. It targets a small initial user group and aims to provide reliable connections on restricted networks, with optional transports and independently managed endpoints.

> **Status: architecture / research (Phase 0).** No functional VPN client, gateway, relay, or security guarantee is implemented yet.

## Design goals

- Real IP-layer VPN with a platform-native TUN/VPN interface.
- Direct-IP connectivity without requiring a domain; WireGuard is the initial data plane.
- Strict fail-closed behavior, DNS/IPv4/IPv6 leak protection, and platform-specific validation.
- Modular transport interfaces; evaluate QUIC and HTTPS/WebSocket relays in later phases.
- **Cloudflare is optional**, never required for normal operation. Its HTTPS/WSS relay must preserve client-to-gateway payload confidentiality.
- Adaptive connection health, manual/preferred/automatic path selection, and independently authenticated endpoint discovery.
- Separate client, gateway, and control-plane responsibilities; per-device identity and revocation.
- Future admin panel for users, devices, policies, gateways, security auditing, and operations.
- Design for active-active multipath while avoiding premature claims of seamless session migration or bandwidth aggregation.

## Initial scope

Private alpha for approximately 5–10 users. Start with Linux gateway and Windows/Linux clients, followed by Android, macOS, and iOS adapters. Favor a modular monolith for management rather than unnecessary distributed infrastructure.

## Architecture

See [architecture baseline](docs/architecture.md), [threat model](docs/threat-model.md), and [roadmap](docs/roadmap.md).

## Security principles

Use vetted cryptographic protocols and libraries. Do not invent custom cryptography. Never commit private keys, credentials, user configuration secrets, or production endpoint lists. Treat transport switching, startup, crash, network changes, and suspend/resume as leak-sensitive events. Test claims on each supported operating system.

## Development

Phase 0 is documentation and interface design. Implementations and benchmarks will be added incrementally with tests and code review.

## License

Not yet selected. No open-source license is granted by default; choose an explicit license before accepting external contributions.
