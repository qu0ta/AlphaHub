package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
)

type PostgresRepository struct {
	pool   *pgxpool.Pool
	scorer Scorer
}

type Scorer interface {
	EvaluateScoring(context.Context, dsclient.ScoringInput) (*dsclient.ScoringResponse, error)
}

func NewPostgresRepository(pool *pgxpool.Pool, scorer Scorer) *PostgresRepository {
	return &PostgresRepository{
		pool:   pool,
		scorer: scorer,
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

	if req.CompanyID != nil {
		var allowed bool

		err := tx.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM users
            WHERE id = $1
              AND company_id = $2::uuid
        )
    `, userID, *req.CompanyID).Scan(&allowed)

		if err != nil {
			return nil, fmt.Errorf("check company access: %w", err)
		}

		if !allowed {
			return nil, ErrCompanyAccessDenied
		}
	}

	const updateQuery = `
		UPDATE applications
		SET company_id = COALESCE($4::uuid, company_id),
			requested_amount_kopecks =
		        COALESCE($5::bigint, requested_amount_kopecks),
		    requested_term_months =
		        COALESCE($6::integer, requested_term_months),
		    purpose = COALESCE($7::text, purpose),
		    data_version = data_version + 1,
		    updated_at = now()
		WHERE id = $1
		  AND created_by_user_id = $2
		  AND data_version = $3
		  AND status = 'DRAFT'
		RETURNING ` + applicationColumns
	updated, err := scanApplication(
		tx.QueryRow(
			ctx,
			updateQuery,
			id,
			userID,
			*req.DataVersion,
			req.CompanyID, // Без *
			req.RequestedAmountKopecks,
			req.RequestedTermMonths,
			req.Purpose,
		),
	)
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

	submittedAt := time.Now().UTC()
	if current.SubmittedAt != nil {
		submittedAt = *current.SubmittedAt
	}
	if err := r.scoreSubmission(ctx, tx, current, submittedAt); err != nil {
		return nil, err
	}
	var score int
	err = tx.QueryRow(ctx,
		`SELECT score FROM scoring_results WHERE application_id = $1 AND data_version = $2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		id, current.DataVersion).Scan(&score)
	if err != nil {
		return nil, fmt.Errorf("read submission score: %w", err)
	}

	route, err := SelectReviewRoute(score, 40, 70)
	if err != nil {
		return nil, err
	}
	nextStatus := "IN_REVIEW"
	if route == RouteManual {
		nextStatus = "MANUAL"
	}
	const updateQuery = `
		UPDATE applications
		SET	status = $4,
			submitted_at = $3,
			updated_at = now()
		WHERE id = $1
		  AND created_by_user_id = $2
		  AND status = 'DRAFT'
		RETURNING ` + applicationColumns
	submitted, err := scanApplication(tx.QueryRow(ctx, updateQuery, id, userID, submittedAt, nextStatus))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotDraft
	}
	if err != nil {
		return nil, fmt.Errorf("submit application: %w", err)
	}

	transitions := []struct {
		from   string
		to     string
		reason string
	}{
		{
			from:   "DRAFT",
			to:     "SUBMITTED",
			reason: "Application submitted by client",
		},
		{
			from:   "SUBMITTED",
			to:     "IN_REVIEW",
			reason: "Financial scoring completed",
		},
	}

	if nextStatus == "MANUAL" {
		transitions = append(transitions, struct {
			from   string
			to     string
			reason string
		}{
			from:   "IN_REVIEW",
			to:     "MANUAL",
			reason: "Score is within the manual review range",
		})
	}
	for _, transition := range transitions {
		_, err := tx.Exec(ctx,
			`INSERT INTO status_history (
			id,
			application_id,
			changed_by_user_id,
			from_status,
			to_status,
			reason
		) VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.NewString(), id, userID, transition.from, transition.to, transition.reason)
		if err != nil {
			return nil, fmt.Errorf("insert status history: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit application submit: %w", err)
	}

	return submitted, nil
}

func (r *PostgresRepository) scoreSubmission(ctx context.Context, tx pgx.Tx, app *Application, submittedAt time.Time) error {
	input := dsclient.ScoringInput{
		AmountKopecks:   *app.RequestedAmountKopecks,
		TermMonths:      *app.RequestedTermMonths,
		ApplicationDate: submittedAt.Format("2006-01-02"),
	}
	var debt *int64
	err := tx.QueryRow(ctx, `SELECT c.inn, p.base_annual_rate_bps, a.declared_monthly_debt_payment_kopecks
 FROM applications a JOIN companies c ON c.id=a.company_id JOIN products p ON p.code=a.product_code
 WHERE a.id=$1`, app.ID).Scan(&input.INN, &input.AnnualRateBps, &debt)
	if err != nil {
		return fmt.Errorf("load scoring parameters: %w", err)
	}
	if debt == nil {
		return ErrFinancialsUnconfirmed
	}
	input.DeclaredMonthlyDebtPaymentKopecks = *debt
	rows, err := tx.Query(ctx, `SELECT to_char(period_month,'YYYY-MM'), revenue_kopecks,
 operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks, confirmed_at IS NOT NULL
 FROM financial_records WHERE application_id=$1 AND data_version=$2 ORDER BY period_month`, app.ID, app.DataVersion)
	if err != nil {
		return fmt.Errorf("load scoring financials: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m dsclient.MonthlyFinancials
		var confirmed bool
		if err := rows.Scan(&m.PeriodMonth, &m.RevenueKopecks, &m.OperatingInflowKopecks, &m.OperatingOutflowKopecks, &m.TaxOutflowKopecks, &confirmed); err != nil {
			return err
		}
		if !confirmed {
			return ErrFinancialsUnconfirmed
		}
		input.Financials = append(input.Financials, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if len(input.Financials) != 6 {
		return ErrFinancialsUnconfirmed
	}
	if err := input.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidScoringPeriod, err)
	}
	result, err := r.scorer.EvaluateScoring(ctx, input)
	if err != nil {
		return err
	}
	snapshot, err := json.Marshal(input)
	if err != nil {
		return err
	}
	explanation, err := json.Marshal(result.Explanation)
	if err != nil {
		return err
	}
	// A financial score is not a bank decision or an assessed bank risk band.
	_, err = tx.Exec(ctx, `INSERT INTO scoring_results
 (id,application_id,data_version,score,risk_band,model_version,features_snapshot,explanation)
 VALUES ($1,$2,$3,$4,'UNASSESSED',$5,$6,$7)`, uuid.NewString(), app.ID, app.DataVersion,
		result.Score, result.ModelVersion, snapshot, explanation)
	if err != nil {
		return fmt.Errorf("save scoring result: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetScoring(ctx context.Context, id, userID string) (*ScoringResult, error) {
	var result ScoringResult
	err := r.pool.QueryRow(ctx, `SELECT s.data_version,s.score,s.model_version,s.explanation,s.features_snapshot,s.created_at
 FROM scoring_results s JOIN applications a ON a.id=s.application_id
 WHERE a.id=$1 AND a.created_by_user_id=$2 ORDER BY s.created_at DESC,s.id DESC LIMIT 1`, id, userID).Scan(
		&result.DataVersion, &result.Score, &result.ModelVersion, &result.Explanation, &result.FeaturesSnapshot, &result.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read scoring: %w", err)
	}
	return &result, nil
}
