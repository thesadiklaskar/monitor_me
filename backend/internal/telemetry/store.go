package telemetry

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) QueryRange(ctx context.Context, query RangeQuery) ([]SeriesPoint, error) {
	return QueryRange(ctx, store.db, query)
}
