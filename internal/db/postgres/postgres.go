package postgres

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Pool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return pool, nil
}
