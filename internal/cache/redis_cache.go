package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

// URLCache is deliberately narrow -- the redirect path only ever needs
// get/set by short_code. Keeping it as an interface lets the service layer
// run with an in-memory fake in tests instead of a real Redis instance.
type URLCache interface {
	Get(ctx context.Context, shortCode string) (string, error)
	Set(ctx context.Context, shortCode, longURL string, ttl time.Duration) error
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string, db int, password string, useTLS bool) *RedisCache {
	opts := &redis.Options{
		Addr:     addr,
		DB:       db,
		Password: password,
	}
	if useTLS {
		// ServerName is left unset: go-redis dials with tls.DialWithDialer,
		// which fills it in from addr's host when the config doesn't set one.
		opts.TLSConfig = &tls.Config{}
	}

	return &RedisCache{client: redis.NewClient(opts)}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Get(ctx context.Context, shortCode string) (string, error) {
	val, err := c.client.Get(ctx, key(shortCode)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, shortCode, longURL string, ttl time.Duration) error {
	return c.client.Set(ctx, key(shortCode), longURL, ttl).Err()
}

func key(shortCode string) string {
	return "url:" + shortCode
}
