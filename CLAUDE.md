# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

The Makefile `include`s and exports `.env`, so most targets depend on `DB_DSN` / `POSTGRES_*` being set there. Copy `.env.example` to `.env` first.

| Task | Command |
| --- | --- |
| Full stack (db + redis + api) | `make up` / `make down` |
| Deps only, app on host | `docker compose up -d db redis` then `make run` |
| Live reload | `make dev` (air) |
| Migrations | `make migrate-up` / `make migrate-down` (one step each) |
| New migration | `make new-migration name=add_foo` |
| Regenerate sqlc code | `make sqlc` |
| Tests | `make test` (`go test -v ./...`) |
| Vet / lint | `make vet` / `make lint` (golangci-lint must be installed separately) |
| CI pipeline | `make ci` (tidy, vet, test, build) |
| Migrate a remote DB | `make migrate-up DB_DSN="postgresql://..."` |
| Deploy | `fly deploy` (or push to `main`) |

Compose does **not** run migrations — `make migrate-up` after `make up` on a fresh database.

Single test / package:

```bash
go test -run TestShorten_DedupesSameURL ./internal/service/
```

goose, sqlc, and air are `go tool` dependencies declared in `go.mod` — no separate install.

## Architecture

Go 1.26, chi router, pgx/v5 pool, go-redis, sqlc-generated queries, goose migrations.

```
cmd/api/main.go   wiring: config → pgxpool → redis → store → repo → service → handler → server
internal/
  handler/        chi routes, JSON decode, error→status mapping
  service/        validation, dedup, code generation, collision retry, cache policy
  repository/     URLRepository interface + Store (pool + generated queries + execTx)
  cache/          URLCache interface + Redis implementation
  db/migrations/  goose SQL
  db/queries/     sqlc query definitions
  db/sqlc/        sqlc-generated — DO NOT EDIT
  config/         env parsing (caarlos0/env) + validation
pkg/base62/       alphabet, Encode/Decode, RandomCode (crypto/rand)
web/              go:embed wrapper; static/ holds the frontend
fly.toml          Fly.io app config; non-secret [env] only
.github/workflows/fly-deploy.yml   deploys on push to main
```

Dependencies point inward through interfaces. `ShortenerService` depends on `repository.URLRepository` and `cache.URLCache`, both faked in [shortener_test.go](internal/service/shortener_test.go), so service tests need neither Postgres nor Redis. Keep new business logic in `service/`; handlers should stay decode → call → map-error → encode.

### Request paths

- **Shorten** — validate absolute http(s) URL → `FindByLongURL` (dedup; returns the existing link *with its original expiry*, ignoring a new `ttl_seconds`) → `RandomCode` → insert, retrying up to `maxCollisionRetries` (5) on a Postgres unique violation.
- **Resolve** (redirect) — Redis first; on miss read Postgres, reject if `expires_at` has passed, then populate Redis for `CACHE_TTL`. Cache errors are logged and fall through to Postgres, so Redis being down costs latency, not availability. Click increment runs in a goroutine on its own 2s `context.Background()` — never on the request path.
- **Lookup** (`GET /api/v1/links/{code}`) — Postgres only, no cache, no click. Returns expired links with `"expired": true` instead of erroring, unlike Resolve.

### Conventions and gotchas

- **Never hand-edit `internal/db/sqlc/`.** Schema change flow: new goose migration → edit `internal/db/queries/urls.sql` → `make sqlc` → `make migrate-up`.
- `sqlc.yaml` overrides `timestamptz` to `time.Time` and nullable `timestamptz` to `*time.Time`; a nullable timestamp column needs both override entries or it lands as a pgtype.
- Errors cross layers as sentinels — `repository.ErrNotFound`, `service.ErrInvalidURL`, `service.ErrLinkExpired`, `cache.ErrCacheMiss` — and `handleServiceError` in [url_handler.go](internal/handler/url_handler.go) is the single place they become status codes (404/400/410, else 500). A new failure mode needs a sentinel plus a case there, not an ad-hoc `w.WriteHeader`.
- `isUniqueViolation` in [shortener.go](internal/service/shortener.go) detects Postgres `23505` via an anonymous `SQLState() string` interface, deliberately keeping pgconn out of the service layer. Preserve that if you touch it.
- Config is fully-formed `DATABASE_URL` / `REDIS_ADDR`, never assembled from parts: compose builds the DSN from `POSTGRES_*` and points at the `db`/`redis` service names, while the Makefile passes `DB_DSN` (localhost) through as `DATABASE_URL`. `.env` carrying both is intentional.
- `Store.execTx` exists but nothing uses it yet — no multi-statement transactions in this service so far.
- `PurgeExpired` / `DeleteExpiredURLs` are implemented but never called; there is no scheduled cleanup.
- Redirects are `301`, so browsers cache them: click counts undercount, and a cached redirect keeps working past expiry. Likewise a code already in Redis is served without re-checking `expires_at`, so a link can outlive its TTL by up to `CACHE_TTL`.
- The frontend is vanilla HTML/CSS/JS in `web/static/`, embedded via `go:embed` and served by `StaticHandler` on `/` and `/static/*`. No npm, no build step, same origin so no CORS. It calls `POST /api/v1/shorten` and nothing else. Keep it dependency-free — an external `<script>` would break the offline/distroless story. Note `go:embed` fails the build if `web/static/` is ever emptied.
- `POST /api/v1/shorten` is rate limited (httprate, 20/min, in-memory) keyed on `RemoteAddr` via `middleware.ClientIPFromRemoteAddr`. That keying is wrong behind Fly's proxy — every request looks like one client — and the limiter is per-machine, so it does not hold across scaled-out instances. Redirect and lookup are unlimited. No auth, no HTTP-level or database integration tests.

### Deployment

Fly.io + Neon (Postgres) + Upstash (Redis); Dockerfile builds a static binary into distroless — no shell, no on-disk assets, the frontend rides along via `go:embed`.

- Secrets (`DATABASE_URL`, `REDIS_ADDR`, `REDIS_PASSWORD`) are set with `fly secrets set`; non-secret config (`APP_ENV`, `HTTP_PORT`, `REDIS_TLS`, `BASE_URL`) lives in `fly.toml` `[env]`. Don't move a secret into `fly.toml`.
- `BASE_URL` in `fly.toml` must track the app hostname — it is what the API returns as `short_url`.
- `REDIS_TLS=true` is mandatory for Upstash: it is TLS-only and drops a plaintext handshake, surfacing as a bare `EOF` from the startup ping in `main`, which is fatal. `NewRedisCache` sets an empty `tls.Config{}` and lets go-redis fill `ServerName` from the addr.
- Upstash's REST URL/token are **not** used — the app speaks the Redis protocol over TCP, so it wants the endpoint host:port and password.
- Nothing runs migrations on deploy. Apply them by hand first: `make migrate-up DB_DSN="<neon-dsn>"` (Neon needs `sslmode=require`).
- `.github/workflows/fly-deploy.yml` deploys every push to `main` with `flyctl deploy --remote-only`; it does not run `make ci` first.
- `min_machines_running = 0` with `auto_stop_machines` — first request after idle pays a cold start. `/health` is liveness only and touches neither Postgres nor Redis, so a green Fly check says nothing about dependencies.
