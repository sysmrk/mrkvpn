# mrkvpn — session and transport contracts (Phase 0)

Status: design baseline and tested in-memory prototype; NOT an operational VPN.

## Responsibility boundaries

- **Platform adapter** owns TUN setup, routes, DNS, firewall/kill-switch, mobile VPN lifecycle, and binding outer transport sockets outside the TUN. It MUST enforce fail-closed policy independently of the user-interface process and session selector wherever the OS permits.
- **Transport adapter** owns endpoint validation, protocol handshake, cryptographic peer authentication, encrypted packet carriage, MTU/packet framing, and cancellation. Cloudflare/other relays are untrusted intermediaries; they do not receive payload keys.
- **Session layer** owns in-memory path bookkeeping and advisory selection policy. It neither authenticates a path nor transmits packets. Only authenticated paths may be registered by a future orchestrator.
- **Gateway** owns trusted authenticated session mapping, packet decapsulation, authorization, routing and stateful egress. It must never infer shared cryptographic identity merely because packets arrived via the same IP.
- **Control plane** owns user/device enrollment, key lifecycle and revocation, policy issuance and audit logging. It must be isolated from the packet data plane.

## Session identity

The logical session concept is associated with an immutable session ID, device identity and intended gateway identity. A transport path has its own ID and cannot change transport kind while reusing that ID. All IDs here are references, not proof of authentication. Secure binding of paths to device and gateway identity must occur in protocol adapters and the gateway, before a path becomes eligible.

## Transport API

    type Adapter interface {
        Kind() Kind
        Connect(context.Context, Endpoint, IdentityRef) (PacketConn, error)
    }

    type PacketConn interface {
        ID() string
        Kind() Kind
        ReadPacket(context.Context, []byte) (int, error)
        WritePacket(context.Context, []byte) error
        Close() error
    }

Implementations must clearly document concurrent Close behavior, context cancellation, buffer ownership, packet boundaries, maximum packet size, replay defense and full peer authentication. A WireGuard adapter may need a userspace implementation or a platform-specific tunnel integration; an ordinary kernel WireGuard interface is NOT automatically a drop-in PacketConn. QUIC alone is not permission to send unauthenticated packets. HTTPS/WSS relays must preserve application-layer end-to-end encryption when terminated by a CDN.

## Selection API and behavior

`Session.Select(now)` returns a recommended **primary** path and never installs routes. Multiple paths may remain connected and monitored. It must not be wired directly to TUN egress without a separate enforceable, authenticated policy gate.

- **Automatic**: chooses lowest relative heuristic score, with improvement threshold and minimum dwell to limit churn.
- **Preferred**: chooses a named path when healthy, else a healthy fallback (subject to dwell when current healthy).
- **Manual**: chooses **only** the named path; if absent or down, returns `ErrNoAvailablePath` without fallback.
- **No eligible path**: returns `ErrNoAvailablePath` and clears the recommendation; the platform component must keep traffic blocked outside the VPN.
- **Unexpected path status**: rejected; metrics cannot be negative, NaN or infinite.
- **Changed / lost path**: stored recommendation cleared if the selected path is removed or reported down.

The current score uses RTT, loss ratio, degraded state and deterministic tie-breaking. It is not a bandwidth estimator or censorship-detection classifier. Path reports can be stale or malicious unless authenticated and appropriately sequenced by a future orchestrator.

## Switching and continuity

This is a control-plane decision. A selected path change **does not guarantee preservation of active TCP sessions**. WireGuard peers, QUIC paths, NAT state and gateway egress identity differ by implementation. Before transparent flow migration can be claimed, specify consistent virtual IP/flow identity, gateway session state, safe rekeying, authenticated path binding, anti-replay windows, and actual migration tests.

## Active-active later

Keep the interface flexible for per-flow scheduling or explicit packet duplication/striping in a later version. Packet-level striping needs a shared authenticated frame format, packet IDs/epochs, bounded reordering and deduplication buffers, path-specific congestion/pacing, MTU management and fairness. Two transport connections over one uplink are not two independent ISPs.

## Domain and Cloudflare rules

Direct-IP WireGuard must operate without DNS or a domain. QUIC direct-IP must authenticate the server identity without relying on insecure certificate bypass. Cloudflare HTTPS/WSS mode may require a DNS hostname and is explicitly optional; no Worker is assumed to be a generic UDP port forwarder.

## Test and evidence policy

Current unit tests validate deterministic in-memory behavior and concurrent read safety. **Not covered:** OS-level DNS/IPv6 leak blocking, kill-switch crashes, WireGuard/QUIC handshakes, packet replay/forgery, DPI resistance, full-tunnel routing, client session continuity or gateway implementation. These require integration and adversarial tests before user traffic.
