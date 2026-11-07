# mrkvpn Control Plane v0.1 API and architecture

**Scope:** one private-alpha instance. Provides persisted **desired inventory state only**. Does not enroll WireGuard peers automatically, apply firewall changes, issue configuration to clients, authenticate gateway agents or disconnect live sessions.

## Trust boundaries

```text
Browser (token in memory)
       |
       | HTTP to 127.0.0.1 only (or SSH tunnel)
       v
mrkvpn-control (non-root, no NET_ADMIN, no Docker socket)
       |--> Typed HTTP API / role authorization
       |--> Atomic single-writer State store (private Docker volume)
       '--> Management audit event stream
       
[NOT CONNECTED] Gateway agent, policy distribution, client enrollment
```

The panel's RBAC affects **management API rights**, not live VPN traffic. Revoking a device in the database is *not* sufficient for WireGuard peer removal. Every eventual Gateway apply must provide authenticated command authorization, versioning, replay protection, ACK/timeout and rollback.

## Resources

- `User`: ID, name, email, status, max_devices, RFC3339 expiration.
- `Device`: device ID, user ID, human label, platform (windows/android/linux/macos/ios), base64 WireGuard public key, status. **No client private keys**.
- `Gateway`: ID, name, direct IP:port, transport and desired enable/disable state.
- `Policy`: per-user desired path mode, optional preferred transport, allowlist and DNS mode (tunnel only).
- `Token`: ID, role, SHA-256 digest, expiration, revocation flag, name. Plaintext returned once; bootstrap stored separately in private file until rotated.
- `AuditEvent`: timestamp, actor token ID, action and target ID. No traffic payloads or browsing destinations.

## HTTP endpoints

| Method | Route | Minimum permission |
|---|---|---|
| GET | `/healthz` | Public, localhost only |
| GET | `/api/v1/me` | Read |
| GET | `/api/v1/overview` | Read |
| GET, POST | `/api/v1/users` | Read / Write |
| PATCH | `/api/v1/users/{id}` | Write |
| GET, POST | `/api/v1/devices` | Read / Write |
| PATCH | `/api/v1/devices/{id}` | Write |
| GET, POST | `/api/v1/gateways` | Read / Write |
| PATCH | `/api/v1/gateways/{id}` | Write |
| GET | `/api/v1/policies` | Read |
| PUT | `/api/v1/policies/{user_id}` | Write |
| GET | `/api/v1/audit` | Read (newest 200) |
| GET, POST | `/api/v1/tokens` | Owner |
| POST | `/api/v1/tokens/{id}/revoke` | Owner |

All resource APIs require `Authorization: Bearer <secret>`. Mutation requests require `Content-Type: application/json`, accept only declared JSON fields and have a request size cap. Responses are JSON; errors contain `error.code` and `error.message`.

### Example: Desired user and device

```json
{
  "name": "Example User",
  "email": "example@invalid.test",
  "max_devices": 2,
  "expires_at": ""
}
```

```json
{
  "user_id": "usr_...",
  "name": "Android phone",
  "platform": "android",
  "wireguard_public_key": "BASE64_32_BYTE_WIREGUARD_PUBLIC_KEY"
}
```

Examples are illustrative and intentionally do not contain valid production identities. Generate WireGuard private keys on the client device and transfer only its public key.

## Authentication and authorization

- Owner: all management routes and token management.
- Admin: read and inventory/policy mutations; no token management.
- Operator and Auditor: read-only in this alpha.
- Cryptographically random 256-bit bearer tokens, hashed at rest, role bound, expirable and revocable.
- One bootstrap Owner token stored as `/data/bootstrap.token` (0600) **must be rotated** after first login; last active Owner protection exists.
- Not yet implemented: human login with WebAuthn/OIDC/MFA; sessions, phishing-resistant auth, granular per-resource RBAC, rate limiting or IP access policies.

## Data store

Small single-node file-backed state with a schema version, atomic rename, fsync and single-writer file lock. Local private-volume permissions required. Not appropriate for multiple replicas or enterprise workloads. JSON-based audit is **not append-only or tamper-evident** and can be changed by a privileged host/volume owner.

## Security constraints

- Docker maps port 8787 to **host loopback only**; no public ingress and no HTTPS proxy at this stage.
- Security headers include CSP, X-Frame-Options DENY, nosniff and no-store.
- Browser does not store bearer token in `localStorage`, sessionStorage or cookies; reload requires reentry.
- No unsafe HTML interpolation of user-supplied fields in the UI (textContent).
- Container runs as numeric unprivileged user, drops capabilities and has read-only root filesystem.
- API has no cross-origin allowance; it is designed for same-origin UI.
- Do not confuse a management inventory record with effective gateway state.

## Production gate

Add OIDC/MFA/Passkey, audited enrollment, gateway mTLS and signed policy apply, safe peer revoke, PostgreSQL migration, backups, integration and security tests before public administration or live deployments. Document and test how policy changes propagate and how stale gateway state is reconciled.
