# Retainer

A private, self-hosted notes and checklists app, synced instantly across a
web app and (later) native mobile apps, behind sign-in. Single-user — one
person, one pool of notes, no sharing or collaborators.

See [CONTEXT.md](CONTEXT.md) for the domain glossary and [docs/adr/](docs/adr/)
for the architectural decisions behind the sync design, auth model, and
tech stack choices.

## Quick start (Docker Compose)

```sh
docker compose build
docker compose up -d
docker compose exec retainer /retainer set-password
```

`set-password` is the only way to set or change the login password — there's
no signup endpoint or setup UI. It prompts interactively for a new password
(twice, to confirm) and writes it straight into the SQLite file.

The `retainer` service publishes no port by default — it's meant to sit
behind a reverse proxy (see [docs/deploy/Caddyfile.example](docs/deploy/Caddyfile.example))
reachable at `retainer:8070` from other containers in the same Compose
network. For standalone use without a proxy, uncomment the `ports:` line in
`docker-compose.yml`.

Data lives in `./data` (bind-mounted to `/data` in the container) and
survives container restarts/rebuilds.

## Deploy without Docker

The Go binary is fully static (pure-Go SQLite driver, `CGO_ENABLED=0` — see
[ADR 0003](docs/adr/0003-go-backend-with-sqlite.md)) and already serves the
built frontend itself via `STATIC_DIR`, so Docker was never a requirement —
just copy a binary and a static-files directory to the server and run it.

**1. Build a release tarball** (on your dev machine — cross-compiles cleanly,
no CGO toolchain needed):

```sh
./scripts/build-release.sh          # defaults to GOOS=linux GOARCH=amd64
# GOARCH=arm64 ./scripts/build-release.sh   # for an arm64 VPS
```

This produces `retainer-linux-amd64.tar.gz` containing the `retainer`
binary and the built `dist/` frontend.

**2. On the VPS**, create a dedicated user and directories, then extract:

```sh
sudo useradd --system --home /opt/retainer --shell /usr/sbin/nologin retainer
sudo mkdir -p /opt/retainer/data
sudo tar -C /opt/retainer -xzf retainer-linux-amd64.tar.gz
sudo chown -R retainer:retainer /opt/retainer
```

**3. Set the password** (as the `retainer` user, or via `sudo -u retainer`):

```sh
sudo -u retainer DB_PATH=/opt/retainer/data/retainer.db /opt/retainer/retainer set-password
```

**4. Install and start the systemd service** — copy
[docs/deploy/retainer.service](docs/deploy/retainer.service) to
`/etc/systemd/system/retainer.service` (paths/user already match the layout
above), then:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now retainer
sudo systemctl status retainer
journalctl -u retainer -f   # logs (structured, via log/slog)
```

The service binds to `127.0.0.1:8070` only — not exposed publicly. Point
Caddy at it with [docs/deploy/Caddyfile.bare-metal.example](docs/deploy/Caddyfile.bare-metal.example).

To deploy an update: rebuild the tarball, `systemctl stop retainer`, extract
the new binary/`dist/` over the old ones (your `data/` directory is
untouched), `systemctl start retainer`.

## Development

Backend (Go) and frontend (Svelte + Vite) run as two separate processes in
dev, with Vite proxying API/WebSocket requests to the Go server.

```sh
# terminal 1 — backend on :8070
cd server
DB_PATH=retainer.db go run ./cmd/retainer set-password   # first time only
DB_PATH=retainer.db go run ./cmd/retainer

# terminal 2 — frontend on :5173, proxies /api and /ws to :8070
cd web
npm install
npm run dev
```

### Tests

```sh
cd server && go test ./...
cd web && npm run check   # svelte-check + tsc
```

## Architecture

- **Backend**: Go, stdlib `net/http`, SQLite via `modernc.org/sqlite` (pure
  Go, no CGO — see [ADR 0003](docs/adr/0003-go-backend-with-sqlite.md)).
- **Sync**: field-level merge using per-device Hybrid Logical Clocks, not
  wall-clock time — see [ADR 0001](docs/adr/0001-field-level-merge-for-sync-conflicts.md).
  `POST /api/sync` is the single endpoint that both pushes local edits and
  pulls authoritative state; a WebSocket (`/ws`) is purely a low-latency
  "something changed, go pull now" signal, not a second data format.
- **Frontend**: Svelte + TypeScript + Vite, no SvelteKit (no SSR need, Go
  already serves as the backend).
- **Ordering**: fractional indexing (LexoRank-style) for drag-and-drop —
  `server/internal/fracindex` (Go) and `web/src/lib/fracindex.ts` (client).
- **Search**: SQLite FTS5, app-maintained (reindexed inside the same
  transaction as any note/item write, not via SQL triggers).
- **Auth**: opaque session tokens in an `httpOnly; Secure` cookie, 90-day
  sliding expiration, no OAuth/passkeys, no public signup.

## Environment variables (server)

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `retainer.db` | SQLite file path |
| `ADDR` | `:8070` | Listen address |
| `STATIC_DIR` | *(unset)* | If set, serves the built frontend from this directory alongside the API |

## Deferred ideas

See [IDEAS.md](IDEAS.md) for things deliberately out of scope for v1.
