package application

import "github.com/jackc/pgx/v5/pgxpool"

type PostgresRepository struct {
	pool *pgxpool.Pool
}
