# url-shortener

A URL shortening service in Go: `POST` a long URL, get back a 7-character code, and `GET /{code}` redirects to the original. Links are stored in Postgres, hot codes are cached in Redis so the redirect path usually avoids the database, and clicks are counted asynchronously so counting never adds latency to a redirect.

Optional per-link TTLs let a short link expire.

Runs locally on Docker Compose, and in production on Fly.io with Neon (Postgres) and Upstash (Redis) — see [Deployment](#deployment).

## Contents

- [Quick start](#quick-start) — get the stack running in three commands
- [API](#api) — endpoints, request and response shapes, error statuses
- [Web UI](#web-ui) — the embedded single-page frontend
- [Configuration](#configuration) — environment variables and defaults
- [Deployment](#deployment) — Fly.io + Neon + Upstash, and the gotchas
- [Development](#development) — make targets and the schema-change flow
- [Architecture](#architecture) — layout, request paths, behaviour worth knowing
- [Testing](#testing) — what is covered and what isn't
- [Reference](#reference) — Postman collection

## Quick start

Requires Docker and Go 1.26+.

```bash
cp .env.example .env
```

Edit `POSTGRES_PASSWORD` (and `DB_DSN` to match) in `.env`, then bring up Postgres, Redis, and the API:

```bash
make up
```

Compose does not run migrations, so create the schema once the database is healthy:

```bash
make migrate-up
```

Open <http://localhost:8080/> for the web UI, or shorten from the terminal:

```bash
curl -s -X POST localhost:8080/api/v1/shorten -H 'Content-Type: application/json' -d '{"url":"https://go.dev/doc/effective_go"}'
```

```json
{
  "short_code": "aK3fQ9x",
  "short_url": "http://localhost:8080/aK3fQ9x",
  "long_url": "https://go.dev/doc/effective_go"
}
```

Follow it:

```bash
curl -i localhost:8080/aK3fQ9x
```

Tear everything down with `make down`.

## API

Base URL defaults to `http://localhost:8080`. All responses are JSON except the redirect.

### `POST /api/v1/shorten`

Creates a short link, or returns the existing one if this exact long URL was shortened before.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `url` | string | yes | Absolute `http` or `https` URL |
| `ttl_seconds` | integer | no | Link expires this many seconds from now |

```bash
curl -s -X POST localhost:8080/api/v1/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/a/very/long/path","ttl_seconds":3600}'
```

`201 Created`:

```json
{
  "short_code": "aK3fQ9x",
  "short_url": "http://localhost:8080/aK3fQ9x",
  "long_url": "https://example.com/a/very/long/path",
  "expires_at": "2026-08-04T11:30:00Z"
}
```

`expires_at` is omitted when the link has no TTL.

### `GET /{code}`

`301 Moved Permanently` to the long URL, with `Location` set. Registers a click.

### `GET /api/v1/links/{code}`

Returns what a code points at *without* redirecting or counting a click — useful for inspecting or debugging a link.

```bash
curl -s localhost:8080/api/v1/links/aK3fQ9x
```

`200 OK`:

```json
{
  "short_code": "aK3fQ9x",
  "short_url": "http://localhost:8080/aK3fQ9x",
  "long_url": "https://example.com/a/very/long/path",
  "click_count": 12,
  "expired": false,
  "expires_at": "2026-08-04T11:30:00Z",
  "created_at": "2026-08-04T10:30:00Z"
}
```

Unlike the redirect, this endpoint still serves expired links — they come back with `"expired": true` rather than an error.

### `GET /health`

`200 OK` with `{"status":"ok"}`. Liveness only; it does not check Postgres or Redis.

### `GET /` and `GET /static/*`

The web UI and its assets. See [Web UI](#web-ui).

### Errors

Errors are `{"error": "<message>"}` with these statuses:

| Status | When |
| --- | --- |
| `400 Bad Request` | Malformed JSON body, or a `url` that isn't an absolute http(s) URL |
| `404 Not Found` | No link with that code |
| `410 Gone` | The link exists but its TTL has passed (redirect only) |
| `429 Too Many Requests` | More than 20 `POST /api/v1/shorten` calls from one IP in a minute |
| `500 Internal Server Error` | Anything else; details go to the server log, not the response |

There is no authentication. `POST /api/v1/shorten` is rate limited to 20 requests per minute per client IP (`httprate`, in-memory); the redirect and lookup endpoints are not limited.

## Web UI

`GET /` serves a single page for shortening a URL and copying the result — a form, an expiry dropdown, and the short link. It covers `POST /api/v1/shorten` only; there is no UI for lookup or stats.

<p align="center">
  <img src="docs/web-ui.png" alt="The web UI: a long URL field, an expiry dropdown, a Shorten button, and the resulting short link with a Copy button" width="560">
</p>

It is plain HTML, CSS, and JavaScript in `web/static/`, embedded into the binary with `go:embed` and served on the same origin as the API, so there is no build step, no asset directory to deploy, and no CORS configuration. Editing the files requires a rebuild — `make dev` picks that up automatically.

Routing is unaffected: chi matches the literal `/static` segment ahead of `/{code}`, and `/` never matches `/{code}`. The only cost is that the short code `static` is now unreachable, out of 62⁷ possibilities.

## Configuration

Config comes from environment variables, loaded from `.env` if present (see `.env.example`). `DATABASE_URL` is the only required value.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ENV` | `development` | Environment name |
| `HTTP_PORT` | `8080` | Port the server binds to |
| `DATABASE_URL` | — | **Required.** Postgres connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis host:port |
| `REDIS_DB` | `0` | Redis database number |
| `REDIS_PASSWORD` | — | Redis password; empty for local/compose Redis, required by hosted providers |
| `REDIS_TLS` | `false` | Dial Redis over TLS. Upstash and most hosted providers need `true` |
| `BASE_URL` | `http://localhost:8080` | Public origin used to build returned short URLs |
| `CODE_LENGTH` | `7` | Characters per short code |
| `CACHE_TTL` | `1h` | How long a resolved code stays in Redis |
| `READ_TIMEOUT` | `5s` | HTTP read timeout |
| `WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `IDLE_TIMEOUT` | `120s` | How long an idle keep-alive connection is held open |

`DATABASE_URL` and `REDIS_ADDR` arrive fully formed rather than assembled from parts. Compose builds the DSN from the `POSTGRES_*` variables and points the API at the `db` and `redis` service names; outside Compose the Makefile passes `DB_DSN` through as `DATABASE_URL`. That is why `.env` carries both a `DB_DSN` (localhost) and the `POSTGRES_*` parts (compose-internal) — they describe the same database reached two different ways.

Compose publishes Postgres on `DB_PORT` (`5433` in the example, since 5432 is often already taken) and the API on `API_PORT`.

## Deployment

The service runs on [Fly.io](https://fly.io) with Postgres on [Neon](https://neon.tech) and Redis on [Upstash](https://upstash.com) — three managed pieces, no servers to keep. `fly.toml` holds the non-secret config; connection strings and passwords are Fly secrets.

The Dockerfile builds a static `CGO_ENABLED=0` binary into `gcr.io/distroless/static-debian12:nonroot`, so the deployed image has no shell, no package manager, and no assets on disk — the frontend is embedded in the binary via `go:embed`.

### 1. Postgres on Neon

Create a project, copy the connection string, and run the migrations against it from your machine. `make migrate-up` takes `DB_DSN` as an override, so no `.env` editing is needed:

```bash
make migrate-up DB_DSN="postgresql://user:password@host.neon.tech/dbname?sslmode=require"
```

Repeat that for each new migration — nothing in the deploy pipeline runs goose, so the schema is applied by hand before the code that needs it ships.

### 2. Redis on Upstash

Create a database and copy its **Redis endpoint and password** from the connection details — the host:port pair and the password, *not* the REST URL and token.

### 3. Fly.io

Install `flyctl`, then from the repository root:

```bash
fly launch
```

`fly launch` detects the Dockerfile and writes `fly.toml`. Then set the secrets — these stay out of the repo and are injected as environment variables at boot:

```bash
fly secrets set DATABASE_URL="postgresql://user:password@host.neon.tech/dbname?sslmode=require" REDIS_ADDR="your-db.upstash.io:6379" REDIS_PASSWORD="your-upstash-password"
```

```bash
fly deploy
```

`REDIS_TLS=true` and `APP_ENV=production` live in `fly.toml` under `[env]` because they are not secret. **`BASE_URL` also lives there and must match the app's hostname** — it is what the API echoes back as `short_url`, so a stale value hands out short links pointing at the wrong host.

Deployment gotchas worth knowing:

- **Upstash is TLS-only.** Without `REDIS_TLS=true` the handshake is dropped and the startup ping fails with a bare `EOF` rather than anything mentioning TLS. `main` treats that ping as fatal, so the machine never becomes healthy.
- **Neon needs `sslmode=require`** in the DSN; its pooled connection string is the one to use for a service that keeps a pgx pool open.
- **Machines scale to zero.** `min_machines_running = 0` with `auto_stop_machines`, so the first request after an idle period pays a cold start.
- **The rate limiter keys off `RemoteAddr`.** Behind Fly's proxy that is not the end user's address, so on Fly the 20/min budget is shared rather than per-client. Fixing it means switching `ClientIPFromRemoteAddr` to the XFF-based variant in [url_handler.go](internal/handler/url_handler.go) — see the comment there.
- **`/health` is liveness only.** It answers `200` without touching Postgres or Redis, so a passing Fly health check does not prove the dependencies are reachable.

### Continuous integration and deployment

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) lints, vets, tests (with `-race`) and builds. It runs on every pull request, and [`.github/workflows/fly-deploy.yml`](.github/workflows/fly-deploy.yml) calls it as a reusable workflow so a push to `main` cannot deploy unless it passes.

The deploy job then runs `flyctl deploy --remote-only`, using a `FLY_API_TOKEN` repository secret. It builds on Fly's remote builders, so nothing is built in the Actions runner. Migrations are still your job — nothing applies them on deploy.

The lint job uses [`golangci/golangci-lint-action`](https://github.com/golangci/golangci-lint-action) rather than `make lint`, with the version pinned to match local installs. golangci-lint is deliberately *not* a `go tool` dependency: [upstream recommends the binary install](https://golangci-lint.run/docs/welcome/install/local/) and advises against `go install` and `go tool`.

## Development

Run the app on the host against the containerised dependencies:

```bash
docker compose up -d db redis
make migrate-up
make run
```

`make dev` does the same with live reload via [air](https://github.com/air-verse/air).

`make help` lists the documented targets. The full set:

| Target | Description |
| --- | --- |
| `make up` / `make down` | Start / stop the full compose stack |
| `make run` / `make dev` | Run locally, with or without live reload |
| `make migrate-up` / `make migrate-down` | Apply / roll back one migration |
| `make new-migration name=add_foo` | Create a new goose migration |
| `make sqlc` | Regenerate `internal/db/sqlc` from the SQL |
| `make test` | `go test -v ./...` |
| `make vet` / `make lint` | `go vet` / `golangci-lint` (install separately: `brew install golangci-lint`) |
| `make build-local` / `make build` | Build for the host / cross-compile to `./bin` |
| `make ci` | tidy, vet, lint, test, build |
| `make db-backup` / `make db-restore file=...` | `pg_dump` / `pg_restore` through the db container |

goose, sqlc, and air are Go tool dependencies (`go tool` in `go.mod`) — no separate install needed.

### Changing the schema

1. `make new-migration name=whatever` and write the `-- +goose Up` / `Down` blocks in `internal/db/migrations/`.
2. Edit or add queries in `internal/db/queries/urls.sql`.
3. `make sqlc` to regenerate `internal/db/sqlc/`. **Never edit `internal/db/sqlc/` by hand** — it is generated.
4. `make migrate-up`.

## Architecture

```
cmd/api/          wiring: config, pgx pool, redis, server, graceful shutdown
internal/
  handler/        chi router, HTTP decoding, status-code mapping
  service/        shortening, resolution, validation, collision retry
  repository/     URLRepository interface over the generated queries
  cache/          URLCache interface + Redis implementation
  db/             goose migrations, sqlc query definitions, and sqlc/ generated code (do not edit)
  config/         env parsing and validation
pkg/base62/       base62 alphabet, encode/decode, crypto/rand code generation
web/              go:embed wrapper + static/ frontend assets
docs/             images and other assets referenced by the README
```

Layers depend inward through interfaces: the service depends on `repository.URLRepository` and `cache.URLCache`, both of which are faked in `internal/service/shortener_test.go`, so the service tests need neither Postgres nor Redis.

**Shorten** validates the URL, looks for an existing row with the same `long_url` (deduplication), then generates a random base62 code and inserts it. A unique-violation on `short_code` is retried with a fresh code up to 5 times — at length 7 that is 62⁷ ≈ 3.5 × 10¹² possibilities, so collisions are a safety net rather than an expected path.

Codes are random (`crypto/rand`) rather than `base62.Encode(id)` so they aren't sequentially guessable. `Encode`/`Decode` remain in `pkg/base62` as the alternative strategy.

**Resolve** checks Redis first and returns immediately on a hit. On a miss it reads Postgres, rejects the link if `expires_at` has passed, writes the code into Redis for `CACHE_TTL`, and returns. Cache failures are logged and fall through to Postgres — Redis being down degrades latency, not availability. The click increment runs in a goroutine on its own 2-second context, off the request path.

### Behaviour worth knowing

- **Redirects are `301`.** Browsers and proxies cache them, so repeat visits may never reach the service — click counts undercount, and a client that cached the redirect keeps following it after the link expires.
- **Expired links can still redirect for up to `CACHE_TTL`.** The expiry check happens on the Postgres path; a code already in Redis is served without re-checking. Shorten TTLs below `CACHE_TTL` if that matters.
- **Deduplication ignores `ttl_seconds`.** Re-shortening a URL that already exists returns the original link with its original expiry; it does not extend or shorten it.
- **Expired rows are never deleted automatically.** `DeleteExpiredURLs` and `URLRepository.PurgeExpired` exist but nothing calls them — there is no scheduled cleanup yet.

## Testing

```bash
make test
```

Covers base62 encoding round-trips, config loading and validation, and the service layer against in-memory fakes. There are no HTTP-level or database integration tests.

## Reference

- [Postman collection](https://personal-0016.postman.co/workspace/golang-apps~e606583f-1b2f-4ff6-bb45-b44ab26cad1b/collection/5714115-576f0423-16d2-4003-81db-1680ec56d811?action=share&creator=5714115&active-environment=5714115-60d4b751-73a6-4482-bf26-42afb5107ba4)
