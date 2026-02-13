# noknok

AT Protocol authentication gateway for Traefik forwardAuth.

## Project Structure

- `cmd/noknok/` — Entry point
- `internal/config/` — Environment + file-based config
- `internal/database/` — pgx pool, schema bootstrap, CRUD queries
- `internal/atproto/` — OAuth client wrapper + Postgres auth store (indigo SDK)
- `internal/session/` — Server-side session management + cookies
- `internal/server/` — Echo HTTP server, routes, handlers, admin panel

## Build & Run

```bash
go build -o noknok ./cmd/noknok
go vet ./...

# Docker
./.launch.sh
```

## Conventions

- Go module: `github.com/primal-host/noknok`
- HTTP framework: Echo v4
- Database: pgx v5 on infra-postgres, database `noknok`
- Container name: `primal-noknok`
- Schema auto-bootstraps via `CREATE TABLE IF NOT EXISTS`
- Config uses env vars with `_FILE` suffix support for Docker secrets

## Database

Postgres on `infra-postgres:5432` (host port 5433), database `noknok`, user `dba_noknok`.

Tables: `sessions`, `users`, `services`, `grants`, `oauth_requests`, `oauth_sessions`.

- `users` — role column: `owner`, `admin`, `user`
- `services` — seeded from `services.json` on startup (ON CONFLICT slug DO UPDATE admin_role); `admin_role` column (default 'admin') sets role for owners/admins
- `grants` — user×service access matrix (CASCADE on delete); `role` column (free-text, default 'user') for per-service role granularity

## Docker

- Image/container: `primal-noknok`
- Network: `infra` (postgres/traefik)
- Port: 4321
- Traefik: `primal.host` (production), `noknok.localhost` (local)
- DNS: `192.168.147.53` (infra CoreDNS)
- Redirect: `noknok.primal.host` → `primal.host` (permanent)
- Defines `noknok-auth` forwardAuth middleware for other services

### Protected Services

All `primal.host` infrastructure services use `noknok-auth@docker` middleware:

- Traefik (`traefik.primal.host`)
- Gitea (`gitea.primal.host`)
- Athens (`athens.primal.host`)
- Avalauncher (`avalauncher.primal.host`)
- Wallet (`wallet.primal.host`)
- pgAdmin (`pgadmin.primal.host`)
- Verdaccio (`verdaccio.primal.host`)
- devpi (`devpi.primal.host`)

## Auth Flow (AT Protocol OAuth)

1. Unauthenticated request to protected service
2. Traefik calls `GET /auth` on noknok (forwardAuth)
3. No valid session cookie → 302 redirect to `/login?redirect=...`
4. User enters Bluesky handle → POST /login → indigo StartAuthFlow
5. noknok redirects user to auth server (e.g. bsky.social/oauth/authorize)
6. User authenticates + approves at auth server
7. Auth server redirects to `/oauth/callback?code=...&state=...&iss=...`
8. noknok calls indigo ProcessCallback → gets DID
9. DID verified against users table → noknok session created → cookie set
10. Redirect back to original service → forwardAuth passes with X-User-DID, X-User-Handle, X-User-Role headers

### ForwardAuth Response Headers

| Header | Description |
|--------|-------------|
| `X-User-DID` | User's AT Protocol DID |
| `X-User-Handle` | User's Bluesky handle |
| `X-User-Role` | Per-service role (from grants table or service admin_role for owners/admins) |

### Non-Browser Client Handling

The `/auth` endpoint detects client type via the `Accept` header:

- **Browser** (Accept contains `text/html`): 302 redirect to login page
- **Non-browser** (git, curl, API clients): 401 so credential helpers can retry
- **Authorization header present**: 200 passthrough (lets backend validate tokens/PATs)

### OAuth Endpoints

- `GET /.well-known/oauth-client-metadata` — OAuth client metadata document
- `GET /oauth/jwks.json` — Public JWK Set for client assertion
- `GET /oauth/callback` — OAuth authorization callback

## Admin Panel

Accessible by clicking the username in the portal header (owner/admin only).

### Role Hierarchy

| Action | Owner | Admin | User |
|--------|-------|-------|------|
| View portal | All services | All services | Granted only |
| Open admin panel | Yes | Yes | No |
| Add/remove owner | Yes | No | No |
| Add/remove admin | Yes | No | No |
| Add/remove user | Yes | Yes | No |
| Manage services | Yes | Yes | No |
| Manage grants | Yes | Yes | No |

### Per-Service Roles

Roles are resolved per-service via the `X-User-Role` header:

- **Owner/Admin** in noknok → gets the service's `admin_role` value (e.g., "admin")
- **Regular user** with a grant → gets the grant's `role` value (free-text, e.g., "user", "viewer", "editor")
- **No grant** → no `X-User-Role` header set

Backend services can use `X-User-Role` for authorization (e.g., Avalauncher checks for "admin" role).

### Admin API Endpoints

All under `/admin/api`, protected by `requireAdmin` middleware:

| Method | Path | Purpose |
|--------|------|---------|
| GET | /users | List all users |
| POST | /users | Create user (resolve handle → DID) |
| PUT | /users/:id/role | Change user role |
| DELETE | /users/:id | Delete user |
| GET | /services | List all services |
| POST | /services | Create service |
| PUT | /services/:id | Update service (name, url, admin_role) |
| DELETE | /services/:id | Delete service |
| GET | /grants | List all grants |
| POST | /grants | Create/update grant (user_id, service_id, role) |
| DELETE | /grants/:id | Delete grant |
