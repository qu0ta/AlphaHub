package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

const applicationColumns = `
	id,
	company_id,
	created_by_user_id,
	product_code,
	assigned_employee_id,
	accepted_offer_id,
	signed_by_user_id,
	requested_amount_kopecks,
	requested_term_months,
	purpose,
	status,
	data_version,
	created_at,
	updated_at,
	submitted_at,
	signed_at,
	disbursed_at
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanApplication(row rowScanner) (*Application, error) {
	var app Application

	err := row.Scan(
		&app.ID,
		&app.CompanyID,
		&app.CreatedByUserID,
		&app.ProductCode,
		&app.AssignedEmployeeID,
		&app.AcceptedOfferID,
		&app.SignedByUserID,
		&app.RequestedAmountKopecks,
		&app.RequestedTermMonths,
		&app.Purpose,
		&app.Status,
		&app.DataVersion,
		&app.CreatedAt,
		&app.UpdatedAt,
		&app.SubmittedAt,
		&app.SignedAt,
		&app.DisbursedAt,
	)
	if err != nil {
		return nil, err
	}

	return &app, nil
}

func (r *PostgresRepository) Create(ctx context.Context, userID, productCode string) (*Application, error) {
	const query = `
		INSERT INTO applications (
			id,
			created_by_user_id,
			product_code,
			purpose,
			status,
			data_version
		)
		SELECT $1, $2, p.code, '', 'DRAFT', 1
		FROM products p
		WHERE p.code = $3 AND p.is_active = true
		RETURNING ` + applicationColumns
	app, err := scanApplication(r.pool.QueryRow(ctx, query, uuid.NewString(), userID, productCode))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductMissing
	}

	if err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}

	return app, nil
}
func (r *PostgresRepository) List(ctx context.Context, userID string) ([]Application, error) {
	const query = `SELECT ` + applicationColumns + ` FROM applications 
    WHERE created_by_user_id = $1 ORDER BY created_at DESC, id DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list application: %w", err)
	}

	defer rows.Close()

	applications := make([]Application, 0)
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		applications = append(applications, *app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applications: %w", err)
	}
	return applications, nil

}
func (r *PostgresRepository) GetByID(ctx context.Context, id, userID string) (*Application, error) {
	const query = `SELECT ` + applicationColumns + ` FROM applications WHERE id = $1 AND created_by_user_id = $2`
	app, err := scanApplication(r.pool.QueryRow(ctx, query, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)

	}
	return app, nil

}
func (r *PostgresRepository) Update(ctx context.Context, id, userID string, req UpdateRequest) (*Application, error) {
	if req.DataVersion == nil {
		return nil, ErrInvalidInput
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin application update: %w", err)
	}
	defer tx.Rollback(ctx)
	const selectQuery = `SELECT ` + applicationColumns + ` FROM applications WHERE id = $1 AND created_by_user_id = $2 FOR UPDATE`
	current, err := scanApplication(tx.QueryRow(ctx, selectQuery, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}
	if current.Status != "DRAFT" {
		return nil, ErrNotDraft
	}
	if current.DataVersion != *req.DataVersion {
		return nil, ErrVersionConflict
	}

	const updateQuery = `
		UPDATE applications
		SET requested_amount_kopecks =
		        COALESCE($4::bigint, requested_amount_kopecks),
		    requested_term_months =
		        COALESCE($5::integer, requested_term_months),
		    purpose = COALESCE($6::text, purpose),
		    data_version = data_version + 1,
		    updated_at = now()
		WHERE id = $1
		  AND created_by_user_id = $2
		  AND data_version = $3
		  AND status = 'DRAFT'
		RETURNING ` + applicationColumns
	updated, err := scanApplication(tx.QueryRow(
		ctx, updateQuery, id, userID, *req.DataVersion, req.RequestedAmountKopecks,
		req.RequestedTermMonths, req.Purpose))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("update application: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit application update: %w", err)
	}
	return updated, nil

}
func (r *PostgresRepository) Submit(ctx context.Context, id, userID string) (*Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin submit tx: %w", err)
	}
	defer tx.Rollback(ctx)

	const selectQuery = `SELECT ` + applicationColumns + ` FROM applications WHERE id = $1 AND created_by_user_id = $2 FOR UPDATE`
	current, err := scanApplication(tx.QueryRow(ctx, selectQuery, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}
	if current.Status != "DRAFT" {
		return nil, ErrNotDraft
	}
	if current.CompanyID == nil ||
		current.RequestedAmountKopecks == nil ||
		current.RequestedTermMonths == nil ||
		current.Purpose == "" {
		return nil, ErrIncompleteApplication
	}
	var companyAccess bool

	err = tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND company_id = $2)`,
		userID, *current.CompanyID).Scan(&companyAccess)
	if err != nil {
		return nil, fmt.Errorf("check company access: %w", err)
	}

	if !companyAccess {
		return nil, ErrIncompleteApplication
	}

	var (
		minAmount int64
		maxAmount int64
		minTerm   int
		maxTerm   int
	)

	err = tx.QueryRow(ctx, `
		SELECT
			min_amount_kopecks,
			max_amount_kopecks,
			min_term_months,
			max_term_months
		FROM products
		WHERE code = $1
		  AND is_active = true
	`, current.ProductCode).Scan(
		&minAmount,
		&maxAmount,
		&minTerm,
		&maxTerm,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductMissing
	}
	if err != nil {
		return nil, fmt.Errorf("load product limits: %w", err)
	}

	amount := *current.RequestedAmountKopecks
	term := *current.RequestedTermMonths

	if amount < minAmount || amount > maxAmount ||
		term < minTerm || term > maxTerm {
		return nil, ErrIncompleteApplication
	}

	const updateQuery = `
		UPDATE applications
		SET	status = 'SUBMITTED',
			submitted_at = now(),
			updated_at = now()
		WHERE id = $1
		  AND created_by_user_id = $2
		  AND status = 'DRAFT'
		RETURNING ` + applicationColumns
	submitted, err := scanApplication(tx.QueryRow(ctx, updateQuery, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotDraft
	}
	if err != nil {
		return nil, fmt.Errorf("submit application: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO status_history (
			id,
			application_id,
			changed_by_user_id,
			from_status,
			to_status,
			reason
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		uuid.NewString(),
		id,
		userID,
		"DRAFT",
		"SUBMITTED",
		"Application submitted by client",
	)

	if err != nil {
		return nil, fmt.Errorf("insert status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit application submit: %w", err)
	}

	return submitted, nil
}
