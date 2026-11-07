# Security boundaries and current limitations

## Current status: not production-ready

mrkvpn is alpha-stage research and gateway bootstrap tooling. **No application-level promise of anonymity, censorship invisibility, leak prevention, uninterrupted connection, or usable end-to-end client-to-gateway VPN is made.**

### Implemented
- Typed transport contracts and in-memory primary-path selector (not networking policy enforcement).
- Strict input validation for a narrow IPv4-only `wg-quick` gateway configuration.
- Gateway key bootstrap and file rendering with restrictive permissions and no-overwrite behavior.
- Opt-in Linux container that attempts to bring up WireGuard and container-local IPv4 egress NAT.
- Isolated Docker tooling and CI syntax/build/smoke checks.

### Not implemented
- Native mrkvpn Windows/Linux/macOS/Android/iOS clients or TUN adapters.
- Client OS routing isolation, persistent kill switch, IPv6 or DNS leak prevention.
- Device provisioning/revocation automation, central auth, admin API/console, MFA/RBAC.
- QUIC, REALITY, Cloudflare HTTPS/WebSocket relay, bridges or signed endpoint discovery.
- Validated multipath session migration, stealth, anti-probing effectiveness or censorship resistance.
- Automated backup/recovery, production observability, high availability, supply-chain release signing.

## Docker isolation

- The default Compose file only contains opt-in profiles: a local initializer, CLI, and diagnostic toolbox. `docker compose up` does not implicitly start a network-facing service.
- `init` and `cli` have `network_mode: none`, `read_only: true`, `no-new-privileges` and a drop-all baseline for Linux capabilities. The offline initializer receives only `CHOWN`, `FOWNER`, and `DAC_OVERRIDE` to maintain owner-only bind-mounted files; the CLI entrypoint receives only `SETUID` and `SETGID` to drop to the repository owner before loading secrets. No tooling service has `NET_ADMIN`. Their only writable state is explicitly mounted local configuration/output directories.
- `diag` is separately opt-in and has `NET_RAW` for container-only packet diagnostics, not `NET_ADMIN` or host networking.
- The optional **lab-only** gateway has `CAP_NET_ADMIN` to set up its own WireGuard interface, IPv4 forwarding and NAT in its container network namespace. It does **not** have `privileged: true`, `SYS_MODULE`, host network mode or Docker daemon socket access.
- The gateway publishes UDP 51820 by default. Cloud/provider/host network firewalls require deliberate configuration. The image only tests local interface existence in its health check, not tunnel security or client reachability.
- Container root and Docker daemon access are meaningful privileges. Treat all build inputs and local compose files as trusted code.

## Secrets

- Gateway private keys and the rendered `wg0.conf` are sensitive. They are never stored in the example JSON, never written to stdout by the CLI, and are excluded from Git by `.gitignore`.
- The offline initializer starts as container root to create local key material. On Linux/WSL2 it assigns ownership of known local files and directories to the owner of the checkout README. The CLI entrypoint then drops to that numeric user before accessing owner-only key files. This limits but does not eliminate the risk from compromised images, local bind mounts or daemon access.
- Ensure local `.local/` state and backups remain restricted; avoid Windows/NTFS bind mounts when permission bits cannot be enforced.
- Client keys must be generated and retained only on their respective devices. Do not pass a client private key to the gateway or control plane.

## Limitations of WireGuard lab gateway

- IPv4-only. DNS, IPv6 and WebRTC traffic on clients can escape if an external client is configured without separate OS-level controls.
- The container performs local NAT/forwarding; the host, Docker bridge, upstream NAT and cloud firewall remain separate security boundaries.
- A peer public key string passing format checks does not prove ownership; the actual WireGuard handshake must authenticate the peer.
- Rotating/moving a gateway config while the container runs may create mismatch. Stop the lab gateway and plan a controlled update.
- Never assume two transports sharing one public IP can defeat IP filtering. Cloudflare is an optional future path, not currently connected.

## Review and release expectations

Before exposing a real service, require a threat-model review, reliable revocation, meaningful per-platform packet captures around shutdown/crash/sleep/route/DNS/IPv6 transitions, replay/authentication tests, gateway firewall checks, image scanning/SBOM, load tests and external security review. GitHub Actions build and unit tests are necessary, not sufficient.

Report security issues privately to the repository owner rather than posting secrets or exploit details in public issues. A dedicated vulnerability disclosure policy can be added before public release.
