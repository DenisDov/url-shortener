package service_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ddovzhenko/urlshortener/internal/cache"
	"github.com/ddovzhenko/urlshortener/internal/repository"
	"github.com/ddovzhenko/urlshortener/internal/service"
	"github.com/ddovzhenko/urlshortener/internal/store"
)

// fakeRepo is an in-memory URLRepository so these tests never touch Postgres.
type fakeRepo struct {
	byCode map[string]store.Url
	byLong map[string]store.Url
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byCode: map[string]store.Url{}, byLong: map[string]store.Url{}}
}

func (f *fakeRepo) Create(_ context.Context, code, longURL string, expiresAt *time.Time) (store.Url, error) {
	u := store.Url{ShortCode: code, LongUrl: longURL, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	f.byCode[code] = u
	f.byLong[longURL] = u
	return u, nil
}

func (f *fakeRepo) FindByCode(_ context.Context, code string) (store.Url, error) {
	u, ok := f.byCode[code]
	if !ok {
		return store.Url{}, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) FindByLongURL(_ context.Context, longURL string) (store.Url, error) {
	u, ok := f.byLong[longURL]
	if !ok {
		return store.Url{}, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) RegisterClick(_ context.Context, code string) error {
	u := f.byCode[code]
	u.ClickCount++
	f.byCode[code] = u
	return nil
}

func (f *fakeRepo) PurgeExpired(_ context.Context) error { return nil }

// fakeCache is an in-memory URLCache, always missing unless primed.
type fakeCache struct{ data map[string]string }

func newFakeCache() *fakeCache { return &fakeCache{data: map[string]string{}} }

func (c *fakeCache) Get(_ context.Context, code string) (string, error) {
	v, ok := c.data[code]
	if !ok {
		return "", cache.ErrCacheMiss
	}
	return v, nil
}

func (c *fakeCache) Set(_ context.Context, code, longURL string, _ time.Duration) error {
	c.data[code] = longURL
	return nil
}

func newTestService() (*service.ShortenerService, *fakeRepo, *fakeCache) {
	repo := newFakeRepo()
	c := newFakeCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewShortenerService(repo, c, "http://localhost:8080", 7, time.Hour, logger)
	return svc, repo, c
}

func TestShorten_CreatesNewCode(t *testing.T) {
	svc, _, _ := newTestService()

	result, err := svc.Shorten(context.Background(), "https://example.com/foo", nil)
	if err != nil {
		t.Fatalf("Shorten returned error: %v", err)
	}
	if result.LongURL != "https://example.com/foo" {
		t.Errorf("unexpected long url: %q", result.LongURL)
	}
	if len(result.ShortCode) != 7 {
		t.Errorf("expected short code length 7, got %d", len(result.ShortCode))
	}
}

func TestShorten_DedupesSameURL(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	first, err := svc.Shorten(ctx, "https://example.com/dup", nil)
	if err != nil {
		t.Fatalf("first Shorten returned error: %v", err)
	}
	second, err := svc.Shorten(ctx, "https://example.com/dup", nil)
	if err != nil {
		t.Fatalf("second Shorten returned error: %v", err)
	}
	if first.ShortCode != second.ShortCode {
		t.Errorf("expected same short code for duplicate url, got %q and %q", first.ShortCode, second.ShortCode)
	}
}

func TestShorten_RejectsInvalidURL(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Shorten(context.Background(), "not-a-url", nil)
	if err != service.ErrInvalidURL {
		t.Errorf("expected ErrInvalidURL, got %v", err)
	}
}

func TestResolve_NotFound(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Resolve(context.Background(), "missing")
	if err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolve_ExpiredLink(t *testing.T) {
	svc, repo, _ := newTestService()
	past := time.Now().Add(-time.Hour)
	_, _ = repo.Create(context.Background(), "expired", "https://example.com/gone", &past)

	_, err := svc.Resolve(context.Background(), "expired")
	if err != service.ErrLinkExpired {
		t.Errorf("expected ErrLinkExpired, got %v", err)
	}
}
