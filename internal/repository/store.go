package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denysdovzhenko/url-shortener/internal/db/sqlc"
)

// Store bundles the connection pool with generated queries and adds
// transaction support on top. Same execTx pattern used across the other
// portfolio projects, kept here for consistency even though this service
// doesn't need multi-statement transactions yet.
type Store struct {
	*sqlc.Queries
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		Queries: sqlc.New(pool),
		pool:    pool,
	}
}

//nolint:unused // kept for parity with the other services; no multi-statement transactions here yet
func (s *Store) execTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	if err := fn(s.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
