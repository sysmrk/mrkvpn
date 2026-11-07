# mrkvpn client platforms and development priorities

Decision: **Windows first, Android second, Linux third**. This prioritization is for development resources, not a promise that any of these clients are already operational.

## Shared client contract

- One logical device identity and gateway identity; transport-specific keys bound to these identities by authenticated protocols.
- One system VPN interface, OS-level fail-closed packet policy, protected endpoint socket routing (avoid recursive routing through the TUN).
- Immutable, versioned, signature-verified desired policies; reject insecure or stale downgrades.
- Secure key generation/storage on the device; client **private keys never go to the gateway or admin panel**.
- Lifecycle: unconfigured -> enrolled -> connecting -> connected -> recovering -> blocked -> disconnected/revoked.
- Explicit, independently verified IPv4, IPv6 and DNS routing behavior. Block rather than silently fall back to a direct route if tunnel is required.
- A temporary Control Plane outage may not force all existing sessions offline, but policy TTL/revocation synchronization must be defined before release.
- Automatic/Preferred/Manual path preferences and eventually active-active path monitoring without false seamless-migration claims.
- Never claim undetectability under DPI/IP blocking. Maintain independent endpoint discovery and avoid single-point CDN dependency.

## Priority 1 — Windows

### Proposed layers
1. **Privileged Windows Service**: owns tunnel lifecycle, Windows Filtering Platform (WFP) enforcement, route/DNS policy and safe startup/shutdown.
2. **Transport/TUN adapter**: assess WireGuardNT versus userspace WireGuard + Wintun for the first implementation; select on measurable security/compatibility/performance criteria. QUIC multiplexing is a separate future contract, not magically supported by a WireGuardNT interface.
3. **Unprivileged desktop UI**: displays connection and gateway status; user/admin actions sent through an authenticated, ACL-restricted named pipe to the service.
4. **Windows protected key storage**: DPAPI/CNG and OS ACLs; avoid exporting plaintext keys in logs/diagnostics.
5. **Integration tests**: service crash, process kill, sleep/resume, Wi-Fi to Ethernet, IPv6, DNS/NRPT, local proxy and captive portal handling. Test WFP enforcement even when UI is closed.

### Exit criteria
Measured leak behavior and interface state during startup, reconnect, shutdown and forced crash. No automatic direct route on failure. Signed binaries and driver distribution plan for release.

## Priority 2 — Android

### Proposed layers
1. **Kotlin Android app** with platform-native lifecycle and UI.
2. **Android VpnService** for TUN and OS VPN ownership. Call `protect()` on outer transport sockets to prevent VPN loops; use network binding as applicable to Android network management.
3. Support **Always-on VPN and Block connections without VPN** where device/policy allows. The OS may still have limitations and exceptions; verify them.
4. **Android Keystore** for local device credentials, foreground service notifications and careful handling of screen off, Doze, mobile/Wi-Fi changes and battery limits.
5. Strict DNS and IPv6 policy and explicit per-app VPN bypass policy (default deny except required transport controls).

### Exit criteria
Test on actual Android versions/vendors under restricted mobile and Wi-Fi networks, with socket and DNS leak captures, app process kills and always-on/lockdown checks.

## Priority 3 — Linux

Linux client remains in scope but scheduled after Windows and Android. The earlier Linux Gateway effort remains necessary and is separate from prioritizing the desktop/mobile **clients**. Linux client implementation should use native TUN, policy routing, nftables and DNS resolver integration with fail-closed unit/system tests.

## Shared API / Gateway integration sequence

1. Versioned client enrollment and proof-of-possession design.
2. Separate device authentication from admin bearer token authentication.
3. Gateway agent authentication via mTLS, signed desired policies and applied-state receipts.
4. Windows alpha with a real WireGuard handshake and leak evidence.
5. Android alpha with native VpnService and real-device leak evidence.
6. Linux client alpha and common feature parity.
7. QUIC/relay transports, then multipath, subject to benchmark and abuse-resistance reviews.

**The admin control-plane in PR #5 currently implements inventory and desired policy only. It does not yet issue clients any live configuration or tokens for VPN access.**
