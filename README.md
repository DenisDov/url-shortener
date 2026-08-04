# url-shortener

A URL shortening service in Go: `POST` a long URL, get back a 7-character code, and `GET /{code}` redirects to the original. Links are stored in Postgres, hot codes are cached in Redis so the redirect path usually avoids the database, and clicks are counted asynchronously so counting never adds latency to a redirect.

Optional per-link TTLs let a short link expire.

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

Shorten a URL:

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

### Errors

Errors are `{"error": "<message>"}` with these statuses:

| Status | When |
| --- | --- |
| `400 Bad Request` | Malformed JSON body, or a `url` that isn't an absolute http(s) URL |
| `404 Not Found` | No link with that code |
| `410 Gone` | The link exists but its TTL has passed (redirect only) |
| `500 Internal Server Error` | Anything else; details go to the server log, not the response |

There is no authentication and no rate limiting.

## Configuration

Config comes from environment variables, loaded from `.env` if present (see `.env.example`). `DATABASE_URL` is the only required value.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ENV` | `development` | Environment name |
| `HTTP_PORT` | `8080` | Port the server binds to |
| `DATABASE_URL` | — | **Required.** Postgres connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis host:port |
| `REDIS_DB` | `0` | Redis database number |
| `BASE_URL` | `http://localhost:8080` | Public origin used to build returned short URLs |
| `CODE_LENGTH` | `7` | Characters per short code |
| `CACHE_TTL` | `1h` | How long a resolved code stays in Redis |
| `READ_TIMEOUT` | `5s` | HTTP read timeout |
| `WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `IDLE_TIMEOUT` | `120s` | How long an idle keep-alive connection is held open |

`DATABASE_URL` and `REDIS_ADDR` arrive fully formed rather than assembled from parts. Compose builds the DSN from the `POSTGRES_*` variables and points the API at the `db` and `redis` service names; outside Compose the Makefile passes `DB_DSN` through as `DATABASE_URL`. That is why `.env` carries both a `DB_DSN` (localhost) and the `POSTGRES_*` parts (compose-internal) — they describe the same database reached two different ways.

Compose publishes Postgres on `DB_PORT` (`5433` in the example, since 5432 is often already taken) and the API on `API_PORT`.

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
| `make sqlc` | Regenerate `internal/store` from the SQL |
| `make test` | `go test -v ./...` |
| `make vet` / `make lint` | `go vet` / `golangci-lint` (must be installed separately) |
| `make build-local` / `make build` | Build for the host / cross-compile to `./bin` |
| `make ci` | tidy, vet, test, build |
| `make db-backup` / `make db-restore file=...` | `pg_dump` / `pg_restore` through the db container |

goose, sqlc, and air are Go tool dependencies (`go tool` in `go.mod`) — no separate install needed.

### Changing the schema

1. `make new-migration name=whatever` and write the `-- +goose Up` / `Down` blocks in `internal/db/migrations/`.
2. Edit or add queries in `internal/db/queries/urls.sql`.
3. `make sqlc` to regenerate `internal/store/`. **Never edit `internal/store/` by hand** — it is generated.
4. `make migrate-up`.

## Architecture

```
cmd/api/          wiring: config, pgx pool, redis, server, graceful shutdown
internal/
  handler/        chi router, HTTP decoding, status-code mapping
  service/        shortening, resolution, validation, collision retry
  repository/     URLRepository interface over the generated queries
  cache/          URLCache interface + Redis implementation
  store/          sqlc-generated code (do not edit)
  db/             goose migrations + sqlc query definitions
  config/         env parsing and validation
pkg/base62/       base62 alphabet, encode/decode, crypto/rand code generation
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
