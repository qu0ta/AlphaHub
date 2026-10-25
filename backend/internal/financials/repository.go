package financials

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func (r *PostgresRepository) Get(ctx context.Context, applicationID, userID string) (*Financials, error) {

	var result Financials
	result.ApplicationID = applicationID
	result.Financials = make([]Month, 0)

	err := r.pool.QueryRow(ctx,
		`SELECT data_version, declared_monthly_debt_payment_kopecks
FROM applications WHERE id = $1 AND created_by_user_id = $2`, applicationID, userID).Scan(
		&result.DataVersion, &result.DeclaredMonthlyDebtPaymentKopecks)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}

	var financialVersion int
	err = r.pool.QueryRow(ctx,
		`SELECT MAX(data_version) FROM financial_records WHERE application_id = $1
			HAVING COUNT(*) > 0`, applicationID).Scan(&financialVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get financial version: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
        SELECT
            to_char(period_month, 'YYYY-MM'),
            revenue_kopecks,
            operating_inflow_kopecks,
            operating_outflow_kopecks,
            tax_outflow_kopecks,
            confirmed_at,
            confirmed_by_user_id
        FROM financial_records
        WHERE application_id = $1
          AND data_version = $2
        ORDER BY period_month
    `, applicationID, financialVersion)
	if err != nil {
		return nil, fmt.Errorf("read financial records: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var month Month
		var confirmedAt *time.Time
		var confirmedBy *string

		err := rows.Scan(
			&month.PeriodMonth,
			&month.RevenueKopecks,
			&month.OperatingInflowKopecks,
			&month.OperatingOutflowKopecks,
			&month.TaxOutflowKopecks,
			&confirmedAt,
			&confirmedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan financial record: %w", err)
		}

		result.Financials = append(result.Financials, month)
		result.ConfirmedAt = confirmedAt
		result.ConfirmedByUserID = confirmedBy
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate financial records: %w", err)
	}

	return &result, nil
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	applicationID, userID string,
	req UpdateRequest,
) (*Financials, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin financial update: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var version int

	err = tx.QueryRow(ctx, `
		SELECT status, data_version
		FROM applications
		WHERE id = $1
		  AND created_by_user_id = $2
		FOR UPDATE
	`, applicationID, userID).Scan(&status, &version)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}

	if status != "DRAFT" {
		return nil, ErrNotDraft
	}

	if version != req.DataVersion {
		return nil, ErrVersionConflict
	}

	newVersion := version + 1

	_, err = tx.Exec(ctx, `
		UPDATE applications
		SET declared_monthly_debt_payment_kopecks = $2,
		    data_version = $3,
		    updated_at = now()
		WHERE id = $1
	`,
		applicationID,
		req.DeclaredMonthlyDebtPaymentKopecks,
		newVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("update application: %w", err)
	}

	for _, month := range req.Financials {
		period, err := time.Parse(
			"2006-01-02",
			month.PeriodMonth+"-01",
		)
		if err != nil {
			return nil, ErrInvalidInput
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO financial_records (
				id,
				application_id,
				data_version,
				period_month,
				revenue_kopecks,
				operating_inflow_kopecks,
				operating_outflow_kopecks,
				tax_outflow_kopecks,
				operating_expenses_kopecks,
				existing_debt_payment_kopecks
			)
			VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8,
				NULL, NULL
			)
		`,
			uuid.NewString(),
			applicationID,
			newVersion,
			period,
			month.RevenueKopecks,
			month.OperatingInflowKopecks,
			month.OperatingOutflowKopecks,
			month.TaxOutflowKopecks,
		)
		if err != nil {
			return nil, fmt.Errorf("insert financial month: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit financial update: %w", err)
	}

	return &Financials{
		ApplicationID:                     applicationID,
		DataVersion:                       newVersion,
		DeclaredMonthlyDebtPaymentKopecks: req.DeclaredMonthlyDebtPaymentKopecks,
		Financials:                        req.Financials,
	}, nil
}

func (r *PostgresRepository) Confirm(
	ctx context.Context,
	applicationID, userID string,
	expectedVersion int,
) (*Financials, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin financial confirmation: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var appVersion int

	err = tx.QueryRow(ctx, `
        SELECT status, data_version
        FROM applications
        WHERE id = $1
          AND created_by_user_id = $2
        FOR UPDATE
    `, applicationID, userID).Scan(&status, &appVersion)

	if appVersion != expectedVersion {
		return nil, ErrVersionConflict
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}

	if status != "DRAFT" {
		return nil, ErrNotDraft
	}

	var recordCount int
	err = tx.QueryRow(ctx, `
        SELECT COUNT(*)
        FROM financial_records
        WHERE application_id = $1
          AND data_version = $2
    `, applicationID, appVersion).Scan(&recordCount)

	if err != nil {
		return nil, fmt.Errorf("count financial records: %w", err)
	}

	if recordCount != 6 {
		return nil, ErrInvalidInput
	}

	tag, err := tx.Exec(ctx, `
        UPDATE financial_records
        SET confirmed_at = now(),
            confirmed_by_user_id = $3
        WHERE application_id = $1
          AND data_version = $2
    `, applicationID, appVersion, userID)

	if err != nil {
		return nil, fmt.Errorf("confirm financial records: %w", err)
	}

	if tag.RowsAffected() != 6 {
		return nil, ErrInvalidInput
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit financial confirmation: %w", err)
	}

	return r.Get(ctx, applicationID, userID)
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}
