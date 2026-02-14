# noknok

AT Protocol authentication gateway for Traefik forwardAuth.

## Project Structure

- `cmd/noknok/` — Entry point
- `internal/config/` — Environment + file-based config
- `internal/database/` — pgx pool, schema bootstrap, CRUD queries
- `internal/atproto/` — OAuth client wrapper + Postgres auth store (indigo SDK)
- `internal/session/` — Server-side session management + cookies (group support)
- `internal/server/` — Echo HTTP server, routes, handlers, admin panel, identity management

## Build & Run

```bash
go build -o noknok ./cmd/noknok
go vet ./...

# Docker
./.launch.sh
```

## Conventions

- Go module: `github.com/primal-host/noknok`
- Display name: nokNok (camelCase)
- HTTP framework: Echo v4
- Database: pgx v5 on infra-postgres, database `noknok`
- Container name: `primal-noknok`
- Schema auto-bootstraps via `CREATE TABLE IF NOT EXISTS` + `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`
- Config uses env vars with `_FILE` suffix support for Docker secrets
- All inline JS must be ES5 compatible (iPad Safari) — no async/await, fetch, const/let, arrow functions; use XMLHttpRequest, var, function expressions
- Go backtick strings injected into JS string literals must be single-line (newlines break the `<script>` block)

## Database

Postgres on `infra-postgres:5432` (host port 5433), database `noknok`, user `dba_noknok`.

Tables: `sessions`, `users`, `services`, `grants`, `oauth_requests`, `oauth_sessions`.

- `sessions` — `group_id` column links multiple identities per browser; `token` is 64-char hex; sessions expire per `SESSION_TTL`
- `users` — role column: `owner`, `admin`, `user`
- `services` — seeded from `services.json` on startup (ON CONFLICT slug DO UPDATE all fields); `admin_role` column (default 'admin') sets role for owners/admins; `enabled` (bool, default true) and `public` (bool, default false) columns for service status
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

### ForwardAuth Grant Enforcement

The `/auth` endpoint enforces per-service access:

- **Disabled service** → browser: 302 redirect to portal; non-browser: 503 Service Unavailable
- **Owner/Admin** → 200 OK for all enabled services (full access)
- **Regular user with grant** → 200 OK with `X-User-Role` header
- **Regular user without grant** → browser: 302 redirect to portal; non-browser: 403
- **No valid session + browser** → 302 redirect to login
- **No valid session + non-browser** (git, curl) → 401 so credential helpers can retry
- **Authorization header present** → 200 passthrough (lets backend validate tokens/PATs)

Service enabled status is checked before session validation — disabled services block all access.

### ForwardAuth Response Headers

| Header | Description |
|--------|-------------|
| `X-User-DID` | User's AT Protocol DID |
| `X-User-Handle` | User's Bluesky handle |
| `X-WEBAUTH-USER` | User's username (for Gitea web auth) |
| `X-User-Role` | Per-service role (from grants table or service admin_role for owners/admins) |

### OAuth Endpoints

- `GET /.well-known/oauth-client-metadata` — OAuth client metadata document
- `GET /oauth/jwks.json` — Public JWK Set for client assertion
- `GET /oauth/callback` — OAuth authorization callback

## Multi-Identity Sessions

Multiple Bluesky identities per browser via session groups (`group_id` UUID).

- First login generates a new group; subsequent logins inherit the group from the existing cookie
- OAuth callback detects duplicate DID in group and switches instead of creating a new session
- Each session has independent TTL

### Identity Routes

| Method | Path | Purpose |
|--------|------|---------|
| POST | /switch | Switch active identity (form: `id`) |
| POST | /logout/one | Log out one identity (form: `id`) |
| POST | /logout | Log out all identities (destroy group) |
| GET | /api/identities | List identities in group (JSON, never exposes tokens) |

### Portal UI

- Identity dropdown in header: active identity, switch to others, "New sign-in", admin link (owner/admin only), per-identity logout, log out all
- Service cards opened via `window.open()` for tab tracking
- Login page shows circled X close button (orange hover) when user already has a session

### Tab Management

- **BroadcastChannel `noknok_portal`**: duplicate portal tabs (from forwardAuth redirects) detect the primary and auto-close, sending a `focus` message first; primary reloads on `focus` message to pick up fresh state
- **Grant revocation**: closing tracked service tabs when grants are toggled off via admin detail panel
- **Logout**: all tracked service tabs closed on form submit
- **Auto-reload**: portal reloads on tab focus after >5s hidden to refresh grants/cards

## Admin Panel

Inline card on portal page (not overlay — overlays broken on iPad Safari). Opened via `/?admin` query param. Server-side tab switching via `?admin&tab=X` (plain `<a>` links, no JS tab switching).

### Tabs

- **Users**: sorted by role (owners first, then admins, then users); first user auto-selected; radio-select users; single Delete button enabled on selection; add-user form requires all fields (handle, username, role) before Add enables
- **Services**: add-service form requires name, slug, URL before Add enables; inline admin_role editing; single Delete button per row
- **Access**: checkbox matrix of users × services with per-grant role editing

### Service Cards (Admin Mode)

Traffic light indicators (read-only) appear on each card's top-right:

- **Admin/owner selected**: red=disabled, yellow=public, green=healthy (all three show service status)
- **Regular user selected**: green=has access, all off=no access

Clicking a service card in admin mode opens a slide-in detail panel with three large toggle buttons:

- **Admin/owner selected**: red toggles enabled/disabled, yellow toggles public/internal, green is outline spacer
- **Regular user selected**: red (no access, click to grant) or green (has access, click to revoke), yellow is outline spacer

Disabled services show a red dot on cards for all users (server-side rendered, no JS needed).

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
- **No grant** → access denied (403 or redirect to portal)

Backend services can use `X-User-Role` for authorization (e.g., Avalauncher checks for "admin" role).

### Admin API Endpoints

All under `/admin/api`, protected by `requireAdmin` middleware:

| Method | Path | Purpose |
|--------|------|---------|
| GET | /users | List all users |
| POST | /users | Create user (resolve handle → DID) |
| PUT | /users/:id/role | Change user role |
| PUT | /users/:id/username | Change username |
| DELETE | /users/:id | Delete user |
| GET | /services | List all services |
| POST | /services | Create service |
| PUT | /services/:id | Update service (name, url, admin_role) |
| PUT | /services/:id/enabled | Toggle service enabled/disabled |
| PUT | /services/:id/public | Toggle service public/internal |
| DELETE | /services/:id | Delete service |
| GET | /services/health | Parallel health check all services (HEAD requests) |
| GET | /grants | List all grants |
| POST | /grants | Create/update grant (user_id, service_id, role) |
| DELETE | /grants/:id | Delete grant |
