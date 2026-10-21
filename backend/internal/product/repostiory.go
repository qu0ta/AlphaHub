package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) GetAll(ctx context.Context) ([]Product, error) {
	const query = `SELECT * FROM products ORDER BY name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		err := rows.Scan(
			&p.Code,
			&p.Name,
			&p.Description,
			&p.MinAmountKopecks,
			&p.MaxAmountKopecks,
			&p.MinTermMonths,
			&p.MaxTermMonths,
			&p.BaseAnnualRateBps,
			&p.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("scan products: %w", err)
		}

		products = append(products, p)
	}
	return products, nil
}

func (r *PostgresRepository) GetByCode(ctx context.Context, code string) (*Product, error) {
	const query = `
		SELECT * FROM products WHERE code = $1
	`
	var p Product

	err := r.pool.QueryRow(ctx, query, code).Scan(
		&p.Code,
		&p.Name,
		&p.Description,
		&p.MinAmountKopecks,
		&p.MaxAmountKopecks,
		&p.MinTermMonths,
		&p.MaxTermMonths,
		&p.BaseAnnualRateBps,
		&p.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product by code: %w", err)
	}

	return &p, nil
}
