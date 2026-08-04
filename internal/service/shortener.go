package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/ddovzhenko/urlshortener/internal/cache"
	"github.com/ddovzhenko/urlshortener/internal/repository"
	"github.com/ddovzhenko/urlshortener/pkg/base62"
)

var (
	ErrInvalidURL  = errors.New("invalid url")
	ErrLinkExpired = errors.New("link expired")
)

const maxCollisionRetries = 5

type ShortenerService struct {
	repo       repository.URLRepository
	cache      cache.URLCache
	baseURL    string
	codeLength int
	cacheTTL   time.Duration
	logger     *slog.Logger
}

func NewShortenerService(
	repo repository.URLRepository,
	c cache.URLCache,
	baseURL string,
	codeLength int,
	cacheTTL time.Duration,
	logger *slog.Logger,
) *ShortenerService {
	return &ShortenerService{
		repo:       repo,
		cache:      c,
		baseURL:    baseURL,
		codeLength: codeLength,
		cacheTTL:   cacheTTL,
		logger:     logger,
	}
}

type ShortenResult struct {
	ShortCode string
	ShortURL  string
	LongURL   string
	ExpiresAt *time.Time
}

// Shorten creates a short code for longURL, or returns the existing one if
// this exact URL was already shortened before.
func (s *ShortenerService) Shorten(ctx context.Context, longURL string, ttl *time.Duration) (ShortenResult, error) {
	if err := validateURL(longURL); err != nil {
		return ShortenResult{}, err
	}

	if existing, err := s.repo.FindByLongURL(ctx, longURL); err == nil {
		return s.toResult(existing.ShortCode, existing.LongUrl, existing.ExpiresAt), nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return ShortenResult{}, fmt.Errorf("check existing url: %w", err)
	}

	var expiresAt *time.Time
	if ttl != nil {
		t := time.Now().Add(*ttl)
		expiresAt = &t
	}

	for attempt := 0; attempt < maxCollisionRetries; attempt++ {
		code, err := base62.RandomCode(s.codeLength)
		if err != nil {
			return ShortenResult{}, fmt.Errorf("generate code: %w", err)
		}

		created, err := s.repo.Create(ctx, code, longURL, expiresAt)
		if err == nil {
			return s.toResult(created.ShortCode, created.LongUrl, created.ExpiresAt), nil
		}

		// unique_violation on short_code -- extremely unlikely at length 7,
		// but retry with a fresh code rather than failing the request
		if isUniqueViolation(err) {
			s.logger.Warn("short code collision, retrying", "attempt", attempt, "code", code)
			continue
		}
		return ShortenResult{}, fmt.Errorf("create url: %w", err)
	}

	return ShortenResult{}, fmt.Errorf("exhausted %d attempts generating a unique code", maxCollisionRetries)
}

// Resolve returns the long URL for a short code, checking the cache first.
// Click count is incremented in the background so it never adds latency to
// the redirect itself.
func (s *ShortenerService) Resolve(ctx context.Context, shortCode string) (string, error) {
	if longURL, err := s.cache.Get(ctx, shortCode); err == nil {
		s.registerClickAsync(shortCode)
		return longURL, nil
	} else if !errors.Is(err, cache.ErrCacheMiss) {
		s.logger.Warn("cache lookup failed, falling back to db", "error", err)
	}

	u, err := s.repo.FindByCode(ctx, shortCode)
	if err != nil {
		return "", err // repository.ErrNotFound propagates as-is
	}

	if u.ExpiresAt != nil && u.ExpiresAt.Before(time.Now()) {
		return "", ErrLinkExpired
	}

	if err := s.cache.Set(ctx, shortCode, u.LongUrl, s.cacheTTL); err != nil {
		s.logger.Warn("cache write failed", "error", err)
	}

	s.registerClickAsync(shortCode)
	return u.LongUrl, nil
}

// registerClickAsync fires the click-count update on its own context so a
// slow or failing DB write never blocks the redirect response.
func (s *ShortenerService) registerClickAsync(shortCode string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.repo.RegisterClick(ctx, shortCode); err != nil {
			s.logger.Error("failed to register click", "short_code", shortCode, "error", err)
		}
	}()
}

func (s *ShortenerService) toResult(code, longURL string, expiresAt *time.Time) ShortenResult {
	return ShortenResult{
		ShortCode: code,
		ShortURL:  s.baseURL + "/" + code,
		LongURL:   longURL,
		ExpiresAt: expiresAt,
	}
}

func validateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrInvalidURL
	}
	return nil
}

// isUniqueViolation checks for Postgres error code 23505 without importing
// pgconn here, keeping this file free of the driver-specific dependency.
func isUniqueViolation(err error) bool {
	type pgErrCoder interface{ SQLState() string }
	var pgErr pgErrCoder
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
