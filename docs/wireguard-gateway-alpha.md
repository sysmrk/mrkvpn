# mrkvpn Phase 1: Linux WireGuard gateway configuration alpha

**Status:** config generator only. This code does **not** create TUN/WireGuard interfaces, modify routes, enable forwarding, install firewall rules, manage DNS, protect client traffic from leaks, or start a service.

The current tool renders a Linux `wg-quick` config safely for inspection and manual installation. It is deliberately IPv4-only. Do **not** treat it as a complete full-tunnel VPN or censorship-resistant service yet.

## Threat boundaries

- Device private keys **stay on their devices**; the gateway receives only device public keys.
- The gateway private key stays in a separately protected file and is loaded only for rendering; neither the JSON file nor standard output contains it.
- The rendered `wg0.conf` contains the gateway private key. Protect it and avoid committing, uploading, emailing or logging it.
- Each peer is restricted to one unique `/32` tunnel address inside the server's private IPv4 subnet. No wildcard `AllowedIPs=0.0.0.0/0` is permitted on gateway peers.
- The config includes no `PostUp`, `PostDown` or shell commands.
- A WireGuard tunnel alone is not a client kill switch. IPv6 and DNS can leak if client OS policy is not yet implemented.

## Prerequisites

On the *gateway host*: install a supported, patched Go toolchain (for building) and separately install Linux WireGuard tooling (`wg`, `wg-quick`). No application dependencies are currently required beyond Go's standard library.

Generate the gateway key on **the gateway** (commands create secret files, so use on an isolated test host):

```sh
umask 077
wg genkey > /etc/wireguard/mrkvpn-gateway.key
wg pubkey < /etc/wireguard/mrkvpn-gateway.key > /etc/wireguard/mrkvpn-gateway.pub
```

Generate each device key on **that device**, using its supported WireGuard key mechanism. Transfer only the public key to the gateway. Never put private keys in source control.

Copy `examples/gateway.example.json` to a non-repository path and replace its placeholder client public key with the actual base64 public key; update the absolute gateway key path and tunnel addresses for your lab.

## Render without touching system networking

```sh
go run ./cmd/mrkvpn gateway check -config /path/to/gateway.json
go run ./cmd/mrkvpn gateway render -config /path/to/gateway.json -out /path/to/new-wg0.conf
```

`check` validates the configuration and prints only a summary. `render` creates a **new** file with mode `0600` and refuses to overwrite existing files. Both commands read the key and do not log it. There is no automatic network change.

On a disposable Linux test gateway, an operator can inspect and **manually** install the generated file under `/etc/wireguard/wg0.conf`, then separately design and test IP forwarding, ingress access restrictions, firewall, IPv4 NAT and monitoring before deliberately starting `wg-quick@wg0`. This is not an automated installation guide. Existing firewall and routing policies vary; do not paste generic network changes into a production host.

A full-tunnel client needs its own independently authenticated gateway public key, correct peer endpoint, routes (e.g. `0.0.0.0/0` *on the client* if a full IPv4 tunnel is intended), a DNS policy and OS-level fail-closed/kill-switch enforcement. **None of those client protections exist yet in mrkvpn.**

## Explicit exclusions

- No IPv6 tunnel support in this alpha. **IPv6 on a real client remains an unresolved leak risk.**
- No NAT, firewall or forwarding automation.
- No client provisioning or tunnel enrollment API.
- No automated gateway deploy, key rotation, key storage manager or revocation propagation.
- No Cloudflare, QUIC, multipath packet forwarding or traffic obfuscation.
- No claims of leak-free networking, censorship undetectability or continuous sessions.

## Tests

```sh
go test -race ./...
go vet ./...
```

Unit tests cover invalid peer addresses, duplicate identities, configuration injection attempts, private-key file permissions, symlink rejection, safe creation, refused overwrite and CLI output not exposing secrets. Integration tests with a network namespace and actual WireGuard handshake are planned separately.
