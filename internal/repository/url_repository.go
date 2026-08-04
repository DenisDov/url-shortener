package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ddovzhenko/urlshortener/internal/store"
)

var ErrNotFound = errors.New("url not found")

// URLRepository is the boundary the service layer depends on, so it can be
// mocked in unit tests without spinning up Postgres.
type URLRepository interface {
	Create(ctx context.Context, shortCode, longURL string, expiresAt *time.Time) (store.Url, error)
	FindByCode(ctx context.Context, shortCode string) (store.Url, error)
	FindByLongURL(ctx context.Context, longURL string) (store.Url, error)
	RegisterClick(ctx context.Context, shortCode string) error
	PurgeExpired(ctx context.Context) error
}

type urlRepository struct {
	store *Store
}

func NewURLRepository(s *Store) URLRepository {
	return &urlRepository{store: s}
}

func (r *urlRepository) Create(ctx context.Context, shortCode, longURL string, expiresAt *time.Time) (store.Url, error) {
	return r.store.CreateURL(ctx, store.CreateURLParams{
		ShortCode: shortCode,
		LongUrl:   longURL,
		ExpiresAt: expiresAt,
	})
}

func (r *urlRepository) FindByCode(ctx context.Context, shortCode string) (store.Url, error) {
	u, err := r.store.GetURLByCode(ctx, shortCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Url{}, ErrNotFound
	}
	return u, err
}

func (r *urlRepository) FindByLongURL(ctx context.Context, longURL string) (store.Url, error) {
	u, err := r.store.GetURLByLongURL(ctx, longURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Url{}, ErrNotFound
	}
	return u, err
}

func (r *urlRepository) RegisterClick(ctx context.Context, shortCode string) error {
	return r.store.IncrementClickCount(ctx, shortCode)
}

func (r *urlRepository) PurgeExpired(ctx context.Context) error {
	return r.store.DeleteExpiredURLs(ctx)
}
