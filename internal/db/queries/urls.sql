-- name: CreateURL :one
INSERT INTO urls (short_code, long_url, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetURLByCode :one
SELECT * FROM urls
WHERE short_code = $1;

-- name: GetURLByLongURL :one
-- used to dedup: if the same long_url was shortened before, reuse the code
SELECT * FROM urls
WHERE long_url = $1
LIMIT 1;

-- name: IncrementClickCount :exec
UPDATE urls
SET click_count = click_count + 1,
    updated_at = now()
WHERE short_code = $1;

-- name: DeleteExpiredURLs :exec
DELETE FROM urls
WHERE expires_at IS NOT NULL AND expires_at < now();
