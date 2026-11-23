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

func (store *Store) QueryLatest(ctx context.Context, query LatestQuery) ([]LatestSample, error) {
	return QueryLatest(ctx, store.db, query)
}
\nfunc (store *Store) QueryLatest(ctx context.Context, query LatestQuery) ([]LatestSample, error) {\n\treturn QueryLatest(ctx, store.db, query)\n}\n