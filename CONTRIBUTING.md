# Contributing to mrkvpn

This repository is in **alpha / research**. Before opening a substantial pull request, discuss the design and security implications with the maintainer. No explicit open-source license has been selected yet; do not assume unrestricted redistribution rights.

## Reproducible setup

See [English Docker guide](docs/docker-getting-started.md) or [راهنمای فارسی](docs/quickstart-fa.md). You do not need a VPS for tooling/build checks.

```sh
docker compose --profile tools build
docker compose --profile tools run --rm init
docker compose --profile tools run --rm cli gateway check -config /config/gateway.json
make test
make lint
```

Do not commit `.local/`, real key material or actual user/device endpoint lists. Please use synthetic test values.

## Design and review standards

- Keep the trust boundary between client, gateway, control plane and untrusted relays explicit.
- Do not implement custom cryptography or make unverifiable claims about censorship resistance.
- Every added privilege, mount, published port, firewall change and routing exception needs justification and tests.
- Document input validation, failure behavior, concurrency guarantees and secret handling.
- Add tests for both success and failure cases; include repeatable leak/failover evidence when those subsystems exist.
- Mark features as experimental until the corresponding integration and security tests exist.
- Use narrow, reviewable PRs and require CI green before merging; current bootstrap PRs are stacked and may need orderly rebasing.

## CI

The workflow runs Go format checks, race-enabled unit tests, `go vet`, Compose parsing, image builds, shell syntax checks and offline bootstrap/render smoke tests. Passing CI is not equivalent to having a working client, secured gateway or successful real-network testing.

## Documentation

Update `README.md`, relevant guides in `docs/`, and the threat model when behavior changes. Prefer exact, copyable commands that state whether they read or modify the network/system.

## Security reports

Never post secrets, client identities, private endpoint lists, or exploit payloads in public issues. See [SECURITY.md](SECURITY.md).
