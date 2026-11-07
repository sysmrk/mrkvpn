# Security policy

## Supported status

**No supported production releases exist.** All currently published source is alpha/research and should be used only in controlled development environments.

## Reporting vulnerabilities

If the repository's GitHub Security tab offers private vulnerability reporting, use that channel. Otherwise contact the repository maintainer through an existing trusted private channel before sharing sensitive details. Do not post credentials, private keys, user traffic, confidential configurations or working exploit payloads in a public issue.

## Security promises we do NOT make

- Running the Compose lab container does not prevent DNS, IPv6 or direct-route leaks on any client.
- An operational WireGuard interface does not mean authentication lifecycle, revocation, client kill switch or safe multi-transport failover is implemented.
- There is no guarantee against DPI detection, IP blocking, traffic analysis, active probing or censorship shutdowns.
- Cloudflare integration and production-ready administration are not implemented. A localhost-only alpha admin panel now supports inventory, desired policies and API-token RBAC but has **no MFA/OIDC, authenticated gateway-agent synchronization or enforced live revocation**.

See [docs/security.md](docs/security.md) for current boundaries, privileges and release requirements.
