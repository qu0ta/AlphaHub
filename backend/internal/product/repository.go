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
	const query = `SELECT code,name,description,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,base_annual_rate_bps,is_active,requires_collateral FROM products ORDER BY name`

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
			&p.RequiresCollateral,
		)
		if err != nil {
			return nil, fmt.Errorf("scan products: %w", err)
		}

		p.AnnualRateBps = p.BaseAnnualRateBps
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return products, nil
}

func (r *PostgresRepository) GetByCode(ctx context.Context, code string) (*Product, error) {
	const query = `
		SELECT code,name,description,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,base_annual_rate_bps,is_active,requires_collateral FROM products WHERE code = $1
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
		&p.RequiresCollateral,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product by code: %w", err)
	}

	p.AnnualRateBps = p.BaseAnnualRateBps
	return &p, nil
}
