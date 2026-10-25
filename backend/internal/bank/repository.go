package bank

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}
func (r *PostgresRepository) GetManual(ctx context.Context) ([]ManualApplication, error) {
	const query = `
		SELECT
			a.id,
			COALESCE(c.name, ''),
			COALESCE(c.inn, ''),
			a.product_code,
			a.requested_amount_kopecks,
			a.requested_term_months,
			a.status,
			scoring.score,
			a.submitted_at,
			history.manual_started_at,
			a.assigned_employee_id
		FROM applications AS a
		LEFT JOIN companies AS c
			ON c.id = a.company_id

		LEFT JOIN LATERAL (
			SELECT s.score
			FROM scoring_results AS s
			WHERE s.application_id = a.id
			  AND s.data_version = a.data_version
			ORDER BY s.created_at DESC, s.id DESC
			LIMIT 1
		) AS scoring ON true

		LEFT JOIN LATERAL (
			SELECT MAX(h.created_at) AS manual_started_at
			FROM status_history AS h
			WHERE h.application_id = a.id
			  AND h.to_status = 'MANUAL'
		) AS history ON true

		WHERE a.status = 'MANUAL'

		ORDER BY
			history.manual_started_at DESC NULLS LAST,
			a.id DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query manual application: %w", err)
	}

	defer rows.Close()
	applications := make([]ManualApplication, 0)
	for rows.Next() {
		var app ManualApplication
		err := rows.Scan(
			&app.ID,
			&app.CompanyName,
			&app.INN,
			&app.ProductCode,
			&app.RequestedAmountKopecks,
			&app.RequestedTermMonths,
			&app.Status,
			&app.Score,
			&app.SubmittedAt,
			&app.ManualReviewStartedAt,
			&app.AssignedEmployeeID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan manual application: %w", err)
		}
		applications = append(applications, app)

	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manual applications: %w", err)
	}
	return applications, nil
}
