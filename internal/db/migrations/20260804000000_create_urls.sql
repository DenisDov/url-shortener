-- +goose Up
CREATE TABLE urls (
    id          BIGSERIAL   PRIMARY KEY,
    short_code  TEXT        NOT NULL UNIQUE,
    long_url    TEXT        NOT NULL,
    click_count BIGINT      NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Shorten() dedups by long_url before generating a new code, so that lookup
-- needs an index too -- it runs on every create.
CREATE INDEX idx_urls_long_url ON urls (long_url);

-- Supports DeleteExpiredURLs; partial because rows without a TTL never match.
CREATE INDEX idx_urls_expires_at ON urls (expires_at) WHERE expires_at IS NOT NULL;

-- +goose Down
DROP TABLE urls;
