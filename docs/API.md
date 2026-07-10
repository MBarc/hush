# Hush API

Base path: `/api/v1`. All requests and responses are JSON. The web UI, the
CLI, and AI agents all use this same API.

## Authentication

Three ways to authenticate, in the order Hush checks them:

1. **Session cookie** (`hush_session`) - set by `POST /auth/login`, used by
   the web UI.
2. **Bearer token** - `Authorization: Bearer hush_...`, used by the CLI and
   agents. User tokens act as their owner; an agent token lives in a folder
   and is GET-only within that folder (cascading).
3. **Device header** - `X-Hush-Device: <hostname>`, honored only when the
   request comes from the IP the poller last saw that hostname at.

The local unix socket (`/data/hush.sock`) is always admin, no credentials.

## Permission model

| Caller | Reads | Writes |
|--------|-------|--------|
| admin user / user token | anything | anything |
| readonly user / its token | granted folder subtrees | never (403) |
| agent token | its folder and everything beneath | never (403) |
| trusted device | granted paths (folder cascades) | granted paths, if `allowWrite` |

Unauthorized reads return `404`, not `403`, so paths cannot be probed.

## Endpoints

### Auth
- `POST /auth/login` - `{username, password}` sets the session cookie
- `POST /auth/logout`
- `GET /auth/me` - current identity, role, grants

### Secrets and folders
- `GET /tree/{path}` - list folders and secrets (filtered to your grants)
- `POST /folders` - `{path}` (admin)
- `DELETE /folders/{path}?recursive=1` (admin)
- `GET /tree/{path}` also returns `tokens[]` for admins (folder items)
- `GET /secrets/{path}` - current value; `?version=N`, `?versions=1`
- `PUT /secrets/{path}` - `{value | credential}` writes a new version
- `PATCH /secrets/{path}` - `{rotation?}` metadata only
- `DELETE /secrets/{path}` (admin)
- `POST /rotate/{path}` - generate a new value per policy (admin)

### Tokens
- `GET /tokens` (admin)
- `POST /tokens` - `{name, type, path?, ttlDays?}`; an agent token requires
  `path` (the folder it reads); returns the token once
- `DELETE /tokens/{name}`

### Users and grants (admin)
- `GET /users`, `POST /users` - `{username, password?, role}`
- `DELETE /users/{name}`, `POST /users/{name}/password`
- `POST /users/{name}/grants` - `{path}`, `DELETE /users/{name}/grants/{path}`

### Devices (admin)
- `GET /devices`
- `POST /devices/{hostname}/trust` - `{scopes, allowWrite, ttlDays}`
- `POST /devices/{hostname}/block`, `DELETE /devices/{hostname}`

### Audit and health
- `GET /audit?limit=&offset=` (admin)
- `GET /healthz` (public)

## Rotation policy

Stored per secret, set via `PATCH .../rotation`:

```json
{
  "length": 32,
  "charset": "full",
  "intervalDays": 30,
  "webhookUrl": "https://automation.lan/hook",
  "webhookSecret": "shared-key",
  "includeValue": false
}
```

`charset` is one of `full`, `alnum`, `hex`, `digits`, or a literal set of
characters. When `intervalDays > 0` the server auto-rotates on schedule.

### Rotation webhooks

After each rotation, if `webhookUrl` is set, Hush POSTs:

```json
{ "event": "rotation", "path": "infra/db/root", "version": 4, "ts": 1720000000 }
```

with `value` included when `includeValue` is true. If `webhookSecret` is
set, the body is signed with HMAC-SHA256 in the `X-Hush-Signature` header.
Verify it before trusting the payload. Delivery is retried three times and
logged to the audit trail either way.
