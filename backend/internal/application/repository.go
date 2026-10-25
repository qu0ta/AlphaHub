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
	disbursed_at,
 annual_rate_bps
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
		&app.AnnualRateBps,
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
			data_version, annual_rate_bps
		)
		SELECT $1, $2, p.code, '', 'DRAFT', 1, p.base_annual_rate_bps
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
		minAmount  int64
		maxAmount  int64
		minTerm    int
		maxTerm    int
		annualRate int
	)

	err = tx.QueryRow(ctx, `
		SELECT
			min_amount_kopecks,
			max_amount_kopecks,
			min_term_months,
			max_term_months,
			base_annual_rate_bps
		FROM products
		WHERE code = $1
		  AND is_active = true
		FOR SHARE
	`, current.ProductCode).Scan(
		&minAmount,
		&maxAmount,
		&minTerm,
		&maxTerm,
		&annualRate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductMissing
	}
	if err != nil {
		return nil, fmt.Errorf("load product limits: %w", err)
	}

	// New applications snapshot their rate on creation. Legacy rows are initialized once.
	if current.AnnualRateBps == nil {
		if _, err = tx.Exec(ctx, "UPDATE applications SET annual_rate_bps=$2 WHERE id=$1 AND annual_rate_bps IS NULL", id, annualRate); err != nil {
			return nil, err
		}
		current.AnnualRateBps = &annualRate
	} else {
		annualRate = *current.AnnualRateBps
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
	var scoringID string
	err = tx.QueryRow(ctx,
		`SELECT id,score FROM scoring_results WHERE application_id = $1 AND data_version = $2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		id, current.DataVersion).Scan(&scoringID, &score)
	if err != nil {
		return nil, fmt.Errorf("read submission score: %w", err)
	}

	route, err := SelectReviewRoute(score, 40, 70)
	if err != nil {
		return nil, err
	}
	nextStatus := "MANUAL"
	finalReason := "Score is within the manual review range"
	if route != RouteManual {
		nextStatus, err = r.saveAutomaticDecision(ctx, tx, current, scoringID, route, annualRate)
		if err != nil {
			return nil, err
		}
		if route == RouteReject {
			finalReason = "Automatic rejection: score below 40"
		} else {
			finalReason = "Automatic offer: score at least 70 and product limits satisfied"
		}
	}
	const updateQuery = `
		UPDATE applications
		SET	status = $4,
			submitted_at = $3,
			updated_at = clock_timestamp()
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

	transitions = append(transitions, struct {
		from   string
		to     string
		reason string
	}{from: "IN_REVIEW", to: nextStatus, reason: finalReason})
	var historyAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&historyAt); err != nil {
		return nil, err
	}
	for index, transition := range transitions {
		var actor *string
		if transition.from == "DRAFT" {
			actor = &userID
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO status_history (
			id,
			application_id,
			changed_by_user_id,
			from_status,
			to_status,
			reason,created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.NewString(), id, actor, transition.from, transition.to, transition.reason, historyAt.Add(time.Duration(index)*time.Microsecond))
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
	err := tx.QueryRow(ctx, `SELECT c.inn, a.annual_rate_bps, a.declared_monthly_debt_payment_kopecks
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
 operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks, confirmed_at IS NOT NULL AND confirmed_by_user_id IS NOT NULL
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
 WHERE a.id=$1 AND a.created_by_user_id=$2 AND s.data_version=a.data_version ORDER BY s.created_at DESC,s.id DESC LIMIT 1`, id, userID).Scan(
		&result.DataVersion, &result.Score, &result.ModelVersion, &result.Explanation, &result.FeaturesSnapshot, &result.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read scoring: %w", err)
	}
	return &result, nil
}
func (r *PostgresRepository) saveAutomaticDecision(ctx context.Context, tx pgx.Tx, app *Application, scoringID string, route ReviewRoute, annualRate int) (string, error) {
	decisionID := uuid.NewString()
	status, kind, reason, comment := "REJECTED", "REJECT", "LOW_SCORE", "Score is below the automatic rejection threshold"
	var payment int64
	var err error
	if route == RouteOffer {
		payment, err = CalculateMonthlyPayment(*app.RequestedAmountKopecks, *app.RequestedTermMonths, annualRate)
		if err != nil {
			return "", ErrIncompleteApplication
		}
		status, kind, reason, comment = "OFFERED", "APPROVE", "HIGH_SCORE", "Score and active product limits allow an automatic offer"
	} else if route != RouteReject {
		return "", fmt.Errorf("unexpected automatic route %q", route)
	}
	var createdAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&createdAt); err != nil {
		return "", fmt.Errorf("automatic decision time: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO decisions(id,application_id,scoring_id,decided_by_user_id,data_version,decision_type,decision_source,reason_code,comment,policy_version,created_at)
 VALUES ($1,$2,$3,NULL,$4,$5,'AUTO',$6,$7,'score-routing-v1',$8)`, decisionID, app.ID, scoringID, app.DataVersion, kind, reason, comment, createdAt)
	if err != nil {
		return "", fmt.Errorf("save automatic decision: %w", err)
	}
	if route == RouteOffer {
		_, err = tx.Exec(ctx, `INSERT INTO offers(id,decision_id,version,amount_kopecks,term_months,annual_rate_bps,monthly_payment_kopecks,status,created_at,valid_until)
   VALUES ($1,$2,1,$3,$4,$5,$6,'PENDING',$7,$8)`, uuid.NewString(), decisionID, *app.RequestedAmountKopecks, *app.RequestedTermMonths, annualRate, payment, createdAt, createdAt.Add(AutomaticOfferValidity))
		if err != nil {
			return "", fmt.Errorf("save automatic offer: %w", err)
		}
	}
	return status, nil
}

func (r *PostgresRepository) lockOwnedApplication(ctx context.Context, tx pgx.Tx, id, user string) (*Application, error) {
	app, err := scanApplication(tx.QueryRow(ctx, `SELECT `+applicationColumns+` FROM applications WHERE id=$1 AND created_by_user_id=$2 FOR UPDATE`, id, user))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return app, err
}

func (r *PostgresRepository) GetOffers(ctx context.Context, id, user string) ([]ClientOffer, error) {
	var data []byte
	err := r.pool.QueryRow(ctx, `SELECT COALESCE((SELECT jsonb_agg(item ORDER BY item->>'created_at' DESC,item->>'id' DESC) FROM (
  SELECT to_jsonb(o)||jsonb_build_object('application_id',d.application_id,'data_version',d.data_version,'comment',d.comment,'decision_type',d.decision_type,
   'status',CASE WHEN o.status='PENDING' AND o.valid_until<=clock_timestamp() THEN 'EXPIRED' ELSE o.status END) item
  FROM offers o JOIN decisions d ON d.id=o.decision_id WHERE d.application_id=a.id
 ) entries),'[]'::jsonb) FROM applications a WHERE a.id=$1 AND a.created_by_user_id=$2`, id, user).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read client offers: %w", err)
	}
	var offers []ClientOffer
	err = json.Unmarshal(data, &offers)
	return offers, err
}

func (r *PostgresRepository) RespondOffer(ctx context.Context, id, offerID, user string, req OfferResponseRequest) (*OfferResponse, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	app, err := r.lockOwnedApplication(ctx, tx, id, user)
	if err != nil {
		return nil, err
	}
	if app.DataVersion != req.DataVersion {
		return nil, ErrVersionConflict
	}
	var data []byte
	err = tx.QueryRow(ctx, `SELECT to_jsonb(o)||jsonb_build_object('application_id',d.application_id,'data_version',d.data_version,'comment',d.comment,'decision_type',d.decision_type)
 FROM offers o JOIN decisions d ON d.id=o.decision_id WHERE o.id=$1 AND d.application_id=$2 FOR UPDATE OF o`, offerID, id).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var offer ClientOffer
	if err = json.Unmarshal(data, &offer); err != nil {
		return nil, err
	}
	if offer.DataVersion != app.DataVersion {
		return nil, ErrVersionConflict
	}
	// Identical retries after a committed response are safe; opposite responses conflict.
	accepted := req.Action == "ACCEPT" && offer.Status == "ACCEPTED" && app.Status == "ACCEPTED" && app.AcceptedOfferID != nil && *app.AcceptedOfferID == offerID
	declined := req.Action == "DECLINE" && offer.Status == "DECLINED" && app.Status == "OFFER_DECLINED"
	if accepted || declined {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &OfferResponse{id, app.DataVersion, app.Status, offer}, nil
	}
	if app.Status != "OFFERED" || offer.Status != "PENDING" || app.AcceptedOfferID != nil {
		return nil, ErrWorkflowConflict
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, err
	}
	if !offer.ValidUntil.After(now) {
		return nil, ErrOfferExpired
	}
	nextStatus, offerStatus := "OFFER_DECLINED", "DECLINED"
	var selected *string
	if req.Action == "ACCEPT" {
		nextStatus, offerStatus = "ACCEPTED", "ACCEPTED"
		selected = &offerID
	}
	_, err = tx.Exec(ctx, `UPDATE offers SET status=$2,responded_at=$3,responded_by_user_id=$4,decline_reason=NULLIF($5,''),decline_comment=NULLIF($6,'') WHERE id=$1`,
		offerID, offerStatus, now, user, req.DeclineReason, req.DeclineComment)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE offers o SET status='WITHDRAWN' FROM decisions d WHERE d.id=o.decision_id AND d.application_id=$1 AND o.id<>$2 AND o.status='PENDING'`, id, offerID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET status=$2,accepted_offer_id=$3,updated_at=$4 WHERE id=$1`, id, nextStatus, selected, now)
	if err != nil {
		return nil, err
	}
	if err = writeWorkflowHistory(ctx, tx, id, user, app.Status, nextStatus, "Client "+req.Action+" offer "+offerID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	offer.Status = offerStatus
	offer.RespondedAt = &now
	offer.RespondedByUserID = &user
	if req.DeclineReason != "" {
		offer.DeclineReason = &req.DeclineReason
	}
	if req.DeclineComment != "" {
		offer.DeclineComment = &req.DeclineComment
	}
	return &OfferResponse{id, app.DataVersion, nextStatus, offer}, nil
}

func (r *PostgresRepository) GetInfoRequests(ctx context.Context, id, user string) ([]InfoRequest, error) {
	var data []byte
	err := r.pool.QueryRow(ctx, `SELECT COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.created_at DESC,i.id DESC)
 FROM info_requests i WHERE i.application_id=a.id),'[]'::jsonb) FROM applications a WHERE a.id=$1 AND a.created_by_user_id=$2`, id, user).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var requests []InfoRequest
	err = json.Unmarshal(data, &requests)
	return requests, err
}

func (r *PostgresRepository) RespondInfo(ctx context.Context, id, requestID, user string, req InfoResponseRequest) (*Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	app, err := r.lockOwnedApplication(ctx, tx, id, user)
	if err != nil {
		return nil, err
	}
	if app.DataVersion != req.DataVersion {
		return nil, ErrVersionConflict
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM info_requests WHERE id=$1 AND application_id=$2 FOR UPDATE`, requestID, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if app.Status != "NEEDS_INFO" || status != "OPEN" || app.SubmittedAt == nil {
		return nil, ErrWorkflowConflict
	}
	if app.CompanyID == nil || app.RequestedAmountKopecks == nil || app.RequestedTermMonths == nil {
		return nil, ErrIncompleteApplication
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND company_id=$2)`, user, *app.CompanyID).Scan(&allowed)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrCompanyAccessDenied
	}
	if err = r.scoreSubmission(ctx, tx, app, *app.SubmittedAt); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE info_requests SET status='RESPONDED',response_comment=$2,responded_at=clock_timestamp() WHERE id=$1`, requestID, req.ResponseComment)
	if err != nil {
		return nil, err
	}
	result, err := scanApplication(tx.QueryRow(ctx, `UPDATE applications SET status='MANUAL',updated_at=clock_timestamp() WHERE id=$1 RETURNING `+applicationColumns, id))
	if err != nil {
		return nil, err
	}
	if err = writeWorkflowHistory(ctx, tx, id, user, "NEEDS_INFO", "MANUAL", "Client responded to information request "+requestID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func writeWorkflowHistory(ctx context.Context, tx pgx.Tx, id, user, from, to, reason string) error {
	_, err := tx.Exec(ctx, `INSERT INTO status_history(id,application_id,changed_by_user_id,from_status,to_status,reason,created_at)
 VALUES ($1,$2,$3,$4,$5,$6,clock_timestamp())`, uuid.NewString(), id, user, from, to, reason)
	return err
}
