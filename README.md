# mrkvpn

**mrkvpn** is an experimental, security-first, cross-platform private VPN project. It targets a small initial user group (approximately 5–10 users), with optional transports and independently managed endpoints.

> **Status: Phase 1 / gateway config-rendering alpha.** A safe WireGuard gateway configuration renderer exists, but no operational VPN client, firewall/kill switch, relay or production security guarantee has been implemented.

## Design goals

- Real IP-layer VPN with platform-native TUN/VPN interfaces.
- Direct-IP connectivity with no domain requirement; WireGuard as the initial data plane.
- Strict fail-closed behavior, DNS/IPv4/IPv6 leak protection, verified separately per OS.
- Modular transport interfaces; evaluate QUIC and HTTPS/WebSocket relays in later phases.
- **Cloudflare is optional**, never required for normal operation. Relay traffic must retain end-to-end client-to-gateway payload encryption.
- Adaptive connection health, manual/preferred/automatic path selection, signed endpoint discovery, independent bridges.
- Separate client, gateway, and control-plane responsibilities; per-device identity and revocation.
- Future admin panel for users, devices, policies, gateways, security auditing, and operations.
- Design for active-active multipath while avoiding unverified claims of seamless session migration or bandwidth aggregation.

## Current code

- `core/transport`: interfaces for authenticated IP-packet transports; no live adapters yet.
- `core/session`: thread-safe path registry and advisory path-selection logic, with unit tests.
- `docs/session-transport-contract.md`: boundaries, security invariants, known limitations, and future active-active integration concerns.
- `internal/wgconfig`: IPv4-only gateway configuration validation and wg-quick rendering; no route/firewall mutations.
- `cmd/mrkvpn`: `gateway check` and `gateway render` commands. Rendering creates a fresh 0600 file and refuses overwrites.
- `examples/gateway.example.json`: an intentionally invalid-key template; replace placeholders locally, not in Git.
- `docs/wireguard-gateway-alpha.md`: key-handling, tests, explicit limitations and non-production lab instructions.

The path selector and gateway renderer do **not** enforce network policy. Nothing in this repository currently protects real client traffic from leakage. Do not route production traffic through it.

Run checks with a supported, patched Go toolchain:

```sh
go test -race ./...
go vet ./...
```

## Gateway config alpha

See [Linux WireGuard gateway alpha](docs/wireguard-gateway-alpha.md). Example commands after creating local WireGuard keys and replacing placeholder configuration:

```sh
go run ./cmd/mrkvpn gateway check -config /path/to/gateway.json
go run ./cmd/mrkvpn gateway render -config /path/to/gateway.json -out /path/to/new-wg0.conf
```

These commands do not install or start WireGuard, or configure firewall, NAT or DNS. The output file contains a private key and is mode 0600.

## Architecture and next steps

See the [Session & Transport Contract](docs/session-transport-contract.md). The architecture baseline, preliminary threat model and roadmap are in the [Phase 0 documentation PR](https://github.com/sysmrk/mrkvpn/pull/1) until merged.

## Security principles

Use vetted cryptography and authenticated endpoints; never invent custom cryptography. Do not commit private keys, credentials, user configuration secrets or production endpoint lists. Treat startup, crash, transport switching and suspend/resume as leak-sensitive events. No claim of censorship invisibility or unconditional reachability is made.

## License

Not yet selected. No open-source license is granted by default; choose an explicit license before accepting external contributions.