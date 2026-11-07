# mrkvpn

**mrkvpn** is an experimental, security-first, cross-platform private VPN project for an initial group of approximately 5–10 people. It aims to combine direct-IP access, multiple independently authenticated transport options, reliable recovery and strict platform-specific leak prevention.

> [!WARNING]
> **Alpha / research software. Not a production VPN.** The current Go code can validate and render a Linux WireGuard gateway configuration, and an optional Docker lab gateway can attempt to bring up WireGuard on a compatible Linux host. There is **no working mrkvpn client, device enrollment service, kill switch, DNS/IPv6 leak protection, admin panel, QUIC adapter or Cloudflare relay** yet. Do not assume running this container protects a laptop or phone.

**[راهنمای فارسی راه‌اندازی](docs/quickstart-fa.md)** | [English Docker guide](docs/docker-getting-started.md) | [Security & limitations](docs/security.md)

## Quick start — no VPS required

You need **Docker Engine/Desktop with the Docker Compose v2 plugin** and network access to fetch the builder image and packages. A Linux kernel WireGuard module is **not required** for the first four commands.

```sh
git clone https://github.com/sysmrk/mrkvpn.git
cd mrkvpn
docker compose --profile tools build
docker compose --profile tools run --rm init
docker compose --profile tools run --rm cli gateway check -config /config/gateway.json
docker compose --profile tools run --rm cli gateway render -config /config/gateway.json -out /output/wg0.conf
```

The initializer is idempotent and creates local **gateway** keys and a zero-client JSON configuration. It never generates private keys for clients. The `check` command validates configuration without changing networking. `render` creates `.local/output/wg0.conf` with mode `0600` and **refuses to overwrite it**. All `.local/` state is ignored by Git.

The optional experimental Linux gateway is **not** started by these commands:

```sh
docker compose -f compose.yaml -f compose.lab.yaml --profile lab up -d --build gateway
```

This opt-in command publishes UDP 51820, requires Linux WireGuard kernel support and Docker `NET_ADMIN` inside the container, and is **for an isolated lab only**. Set up the firewall, inbound port permissions, test clients and leak protection separately. See the [step-by-step documentation](docs/docker-getting-started.md).

## What is in this repository?

```text
cmd/mrkvpn/                  Go CLI: gateway check/render
core/session/                In-memory, deterministic path selection
core/transport/              Authenticated packet transport interfaces
internal/wgconfig/           Validation and wg-quick file renderer
docker/                      Local bootstrap and lab gateway entrypoints
compose.yaml                 Safe opt-in tooling and diagnostics
compose.lab.yaml             Separate, opt-in Linux lab gateway
Dockerfile                   Multi-stage builder/toolbox/gateway images
docs/                        Guides, API contracts, security boundaries
.github/workflows/           Go and Docker checks on pull requests
.local/                      Local secrets/config/output; never committed
```

### Docker images and tools

| Image target | Role | Included components |
| --- | --- | --- |
| `build` | Compile/test Go (CI and local development) | Go toolchain, C compiler for race tests |
| `tooling` | No-network key bootstrap and config CLI, optional network diagnostics | mrkvpn CLI, WireGuard tools, iproute2, iptables/nftables, dig, curl, jq, ping, tcpdump, OpenSSL |
| `gateway` | Optional Linux kernel WireGuard lab endpoint | WireGuard tools, iproute2, iptables, tini |

Initialization and CLI commands run with **no network**, a **read-only container filesystem**, a drop-all capability baseline with only narrowly scoped ownership/UID-switch exceptions, and no Docker socket mount. The lab gateway alone receives `CAP_NET_ADMIN`, only when explicitly started. Docker images must still be trusted; Docker daemon access on the host is privileged.

## Design goals (not all implemented)

- IP-layer VPN with platform-native TUN and a real system-level fail-closed kill switch.
- WireGuard as the primary direct-IP transport; no domain or Cloudflare dependency.
- Independent QUIC/HTTPS transports, optional Cloudflare HTTPS/WSS relay, independent bridge endpoints and signed endpoint discovery.
- Manual, preferred and automatic path selection; design for later active-active multipath without claiming bandwidth aggregation.
- Client/gateway/control-plane separation with per-device authentication, revocation, MFA, RBAC, audit logs and a future admin UI.
- Measurable IPv4/IPv6, DNS and failure-transition leak tests on each supported OS.

## Development and quality

With Docker installed, use `make build init check render` on Linux/macOS/WSL2, or the raw Compose commands on Windows. `make test` runs Go tests in the build container. CI checks Go formatting, unit tests, race detector, static analysis, Compose syntax, image builds and an offline config-render smoke test.

Work is proposed as stacked pull requests while under review:
[architecture PR #1](https://github.com/sysmrk/mrkvpn/pull/1),
[core PR #2](https://github.com/sysmrk/mrkvpn/pull/2),
[gateway generator PR #3](https://github.com/sysmrk/mrkvpn/pull/3).
These links are informational; the project should be merged in dependency order.

## Security and governance

Never commit private keys, real gateway configuration, endpoint lists, tokens or user credentials. Local state is kept under `.local/`. Running Docker does not provide a client kill switch or eliminate leaks. See [Security & limitations](docs/security.md) before exposing any UDP port to the internet.

**License:** not selected. Until an explicit license is added, no open-source license is granted by default.
