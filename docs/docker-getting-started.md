# mrkvpn — Docker-first getting started

This guide is for the **current alpha**. It provides reproducible build, key bootstrap, config validation and file rendering **without requiring a VPN server or WireGuard kernel on your development machine**.

## 1. Prerequisites

- Git.
- Docker Engine or Docker Desktop with Compose v2 (`docker compose version`).
- Internet access to pull the base images and Alpine packages for the first build.
- Optional `make` for convenience targets; direct Compose commands work without Make.
- **Only for the experimental lab gateway:** a Linux host with WireGuard kernel support, a rootful/compatible Docker daemon and access to UDP 51820. Docker Desktop and WSL2 may vary by kernel and networking behavior.

The repository uses multi-stage `Dockerfile` targets. No Go installation is needed on the host. The image tags used for development are specified in the Dockerfile; pin reviewed digests for an eventual production release.

## 2. Get the source and build tools

```sh
git clone https://github.com/sysmrk/mrkvpn.git
cd mrkvpn
docker compose version
docker compose --profile tools build
```

This builds the `tooling` image locally. No server is exposed and `NET_ADMIN` is not granted to the tools. A local Docker installation is needed to execute these commands; without one, use the GitHub Actions pull-request results for build checks.

## 3. Bootstrap the **gateway only**

```sh
docker compose --profile tools run --rm init
```

Creates or preserves these paths:

```text
.local/
  config/gateway.json       # gateway CIDR, port and public peer list (starts empty)
  secrets/gateway.key       # gateway PRIVATE key; never publish
  secrets/gateway.pub       # gateway public key
  output/                   # rendered configs, initially empty
```

This command does not generate client private keys or modify host routes/firewall. Re-running it preserves existing file contents, rather than silently rotating keys. On Linux/WSL2, bootstrap assigns the local config/secrets/output paths to the user who owns the checkout README, so they remain editable. The CLI runs under that UID/GID (rather than remaining root) for key access and file rendering.

**Secret storage:** on Linux and WSL2, keep this repo inside a Linux filesystem with proper mode bits; do not keep private keys on Windows-mounted NTFS under `/mnt/c`. Treat backups and Docker daemon access as privileged. A developer who can access the Docker daemon or `.local/secrets/` can access these key files.

## 4. Check and render

```sh
docker compose --profile tools run --rm cli gateway check -config /config/gateway.json
docker compose --profile tools run --rm cli gateway render -config /config/gateway.json -out /output/wg0.conf
```

The check validates JSON structure, network range, public keys, unique client /32 addresses and gateway private-key file permissions. It does **not** contact the internet. The renderer creates `.local/output/wg0.conf`, file mode `0600`; if the file exists, rendering fails intentionally. The output **contains the gateway private key**, so never display or commit it.

A zero-peer rendered gateway config is a legitimate bootstrap artifact; it cannot provide client connectivity until actual client keys are enrolled and a full client configuration is separately created.

### Add a client peer

Generate a private/public WireGuard key pair **on the client device**, using a trusted WireGuard implementation. Only copy the client's **public key** to your local gateway JSON. Edit `.local/config/gateway.json`:

```json
{
  "interface_name": "wg0",
  "address": "10.77.0.1/24",
  "listen_port": 51820,
  "private_key_file": "/secrets/gateway.key",
  "peers": [
    {
      "name": "my-laptop",
      "public_key": "BASE64_WIREGUARD_PUBLIC_KEY_FROM_CLIENT",
      "address": "10.77.0.2/32"
    }
  ]
}
```

The placeholder above is intentionally invalid. Each client requires its own unique public key and /32 tunnel IP. The gateway public key is found in `.local/secrets/gateway.pub` and is safe to share with your own clients.

Re-run `check`. Since `render` refuses to overwrite, back up or deliberately move an old `wg0.conf` **while the lab gateway is stopped**, then render a fresh version. Changing a config file alone does not apply it to a running gateway. Back up key material securely and never rotate it accidentally.

## 5. Optional Linux lab gateway — intentionally separate

On a **disposable Linux test host only**, with a rendered `.local/output/wg0.conf` and suitable host networking:

```sh
docker compose -f compose.yaml -f compose.lab.yaml --profile lab config --quiet
docker compose -f compose.yaml -f compose.lab.yaml --profile lab up -d --build gateway
docker compose -f compose.yaml -f compose.lab.yaml --profile lab ps
docker compose -f compose.yaml -f compose.lab.yaml --profile lab logs --follow gateway
```

To stop it without deleting keys:

```sh
docker compose -f compose.yaml -f compose.lab.yaml --profile lab down
```

The lab container tries to create `wg0`, enable IPv4 forwarding in **its own namespace** and add limited container-local IPv4 NAT/forwarding rules. Docker publishes UDP `51820` on the host. Host/cloud firewalls, external NAT, endpoint routing and client configuration are **your responsibility**. Running `wg show wg0` in the container only demonstrates the interface exists; the health check does **not** prove an authenticated client handshake, internet reachability or absence of leaks.

Do **not** expose this gateway while relying on it to protect real clients. No mrkvpn client application or OS-level kill switch exists yet. IPv6, DNS and WebRTC/direct-route leaks remain unmitigated by this server-side setup.

## 6. Diagnostics

For an opt-in network-connected **container-only** toolbox:

```sh
docker compose --profile diagnostics run --rm diag
```

The toolbox provides `wg`, `ip`, `iptables`, `nft`, `curl`, `dig`, `jq`, `ping`, `tcpdump` and `openssl`. It does not share the host network namespace. Captures and route tests in this container are not equivalent to packet captures on the VPN client or gateway.

## 7. Tests without a server

```sh
make test
make lint
docker compose config --quiet
docker compose -f compose.yaml -f compose.lab.yaml --profile lab config --quiet
```

`make` is optional. GitHub Actions performs Go tests, format/vet checks, Docker image builds and an offline bootstrap/config smoke test for each PR. This CI cannot establish DPI resistance, OS kill-switch behavior or successful real-world VPN connections.

## Troubleshooting

| Symptom | Likely reason / safe next step |
| --- | --- |
| `cannot connect to Docker daemon` | Start Docker Engine/Desktop and verify `docker info`. |
| `private_key_file ... permissions` | Check `0600` permission on Linux filesystem. Do not globally loosen permissions. |
| `wg0.conf already exists` | Intentional no-overwrite safety. Stop lab, preserve a backup, and explicitly move the old file before rendering again. |
| `operation not permitted` on `wg-quick up` | Linux WireGuard kernel support, container capability or Docker daemon restrictions. Check lab host and container logs. |
| No handshake | No enrolled client, UDP port/firewall/NAT issue, client endpoint mismatch, or key mismatch. This is not diagnosed by health checks alone. |
| No internet after handshake | Client routing, host/cloud firewall, Docker bridge forwarding/NAT or DNS configuration might be incomplete. |
| Docker Desktop / WSL2 | Kernel and filesystem behavior may differ. Test in WSL2 Linux filesystem and avoid assuming a production Linux deployment is equivalent. |

## Why no all-in-one VPN stack yet?

A polished install process cannot make unimplemented client protections real. The administrator UI, device enrollment, QUIC/HTTPS/Cloudflare transport and OS-level leak protection are explicitly future work. The repository makes only the **existing gateway bootstrap and inspection** easy to reproduce.
