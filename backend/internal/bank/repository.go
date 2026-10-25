package bank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qu0ta/AlphaHub/internal/application"
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

const applicationDetailQuery = `
SELECT jsonb_build_object(
 'application', to_jsonb(a),
 'info_requests',COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.created_at DESC,i.id DESC) FROM info_requests i WHERE i.application_id=a.id),'[]'::jsonb),
 'decisions', COALESCE((SELECT jsonb_agg(to_jsonb(d) ORDER BY d.created_at DESC,d.id DESC)
   FROM decisions d WHERE d.application_id=a.id),'[]'::jsonb),
 'offers', COALESCE((SELECT jsonb_agg(to_jsonb(o)||jsonb_build_object('status',CASE WHEN o.status='PENDING' AND o.valid_until<=statement_timestamp() THEN 'EXPIRED' ELSE o.status END) ORDER BY o.created_at DESC,o.id DESC)
   FROM offers o JOIN decisions d ON d.id=o.decision_id WHERE d.application_id=a.id),'[]'::jsonb),
 'company', (SELECT to_jsonb(c) FROM companies c WHERE c.id=a.company_id),
 'product', jsonb_build_object('code',p.code,'name',p.name,'base_annual_rate_bps',COALESCE(a.annual_rate_bps,p.base_annual_rate_bps)),
 'declared_monthly_debt_payment_kopecks', a.declared_monthly_debt_payment_kopecks,
 'financials', COALESCE((
   SELECT jsonb_agg(jsonb_build_object(
     'period_month',to_char(f.period_month,'YYYY-MM'),
     'data_version',f.data_version,
     'revenue_kopecks',f.revenue_kopecks,
     'operating_inflow_kopecks',f.operating_inflow_kopecks,
     'operating_outflow_kopecks',f.operating_outflow_kopecks,
     'tax_outflow_kopecks',f.tax_outflow_kopecks,
     'confirmed_at',f.confirmed_at,'confirmed_by_user_id',f.confirmed_by_user_id
   ) ORDER BY f.period_month)
   FROM financial_records f WHERE f.application_id=a.id AND f.data_version=a.data_version
 ),'[]'::jsonb),
 'scoring', (
   SELECT jsonb_build_object('id',s.id,'data_version',s.data_version,'score',s.score,
     'model_version',s.model_version,'explanation',s.explanation,'created_at',s.created_at)
   FROM scoring_results s WHERE s.application_id=a.id AND s.data_version=a.data_version
   ORDER BY s.created_at DESC,s.id DESC LIMIT 1
 ),
 'documents', COALESCE((
   SELECT jsonb_agg(jsonb_build_object('id',d.id,'application_id',d.application_id,
     'document_type',d.document_type,'original_filename',d.original_filename,
     'processing_status',d.processing_status,'uploaded_at',d.uploaded_at)
     ORDER BY d.uploaded_at DESC,d.id DESC)
   FROM documents d WHERE d.application_id=a.id
 ),'[]'::jsonb),
 'checks', COALESCE((
   SELECT jsonb_agg(to_jsonb(c) ORDER BY c.started_at DESC,c.id DESC)
   FROM check_results c WHERE c.application_id=a.id AND c.data_version=a.data_version
 ),'[]'::jsonb),
 'history', COALESCE((
   SELECT jsonb_agg(to_jsonb(h) ORDER BY h.created_at,h.id)
   FROM status_history h WHERE h.application_id=a.id
 ),'[]'::jsonb)
)
FROM applications a JOIN products p ON p.code=a.product_code
WHERE a.id=$1 AND a.status <> 'DRAFT' AND a.submitted_at IS NOT NULL
`

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*ApplicationDetail, error) {
	var payload []byte
	if err := r.pool.QueryRow(ctx, applicationDetailQuery, id).Scan(&payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read bank application: %w", err)
	}
	var detail ApplicationDetail
	if err := json.Unmarshal(payload, &detail); err != nil {
		return nil, fmt.Errorf("decode bank application: %w", err)
	}
	return &detail, nil
}

func (r *PostgresRepository) Claim(ctx context.Context, id, employeeID string, expectedVersion int) (*Assignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin bank assignment: %w", err)
	}
	defer tx.Rollback(ctx)
	var role string
	err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 AND role='BANK_EMPLOYEE' FOR SHARE`, employeeID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("check employee: %w", err)
	}
	var assigned *string
	result := &Assignment{ApplicationID: id, AssignedEmployeeID: employeeID}
	err = tx.QueryRow(ctx, `SELECT status,data_version,assigned_employee_id,updated_at
 FROM applications WHERE id=$1 AND status <> 'DRAFT' AND submitted_at IS NOT NULL FOR UPDATE`, id).Scan(
		&result.Status, &result.DataVersion, &assigned, &result.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock bank application: %w", err)
	}
	if result.Status != "MANUAL" {
		return nil, ErrNotManual
	}
	if result.DataVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	if assigned != nil && *assigned != employeeID {
		return nil, ErrAlreadyAssigned
	}
	if assigned == nil {
		err = tx.QueryRow(ctx, `UPDATE applications SET assigned_employee_id=$2,updated_at=clock_timestamp()
   WHERE id=$1 RETURNING updated_at`, id, employeeID).Scan(&result.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("assign bank application: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bank assignment: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) Decide(ctx context.Context, id, employeeID string, req DecisionRequest) (*DecisionResult, error) {
	req, err := validateDecisionRequest(req)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin bank decision: %w", err)
	}
	defer tx.Rollback(ctx)
	var role string
	err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 AND role='BANK_EMPLOYEE' FOR SHARE`, employeeID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("check decision employee: %w", err)
	}
	var status, productCode string
	var version int
	var assigned *string
	var amount *int64
	var term *int
	var savedRate *int
	err = tx.QueryRow(ctx, `SELECT status,data_version,assigned_employee_id,product_code,requested_amount_kopecks,requested_term_months,annual_rate_bps
 FROM applications WHERE id=$1 AND status <> 'DRAFT' AND submitted_at IS NOT NULL FOR UPDATE`, id).Scan(&status, &version, &assigned, &productCode, &amount, &term, &savedRate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock decision application: %w", err)
	}
	if status != "MANUAL" {
		return nil, ErrNotManual
	}
	if version != req.DataVersion {
		return nil, ErrVersionConflict
	}
	if assigned == nil {
		return nil, ErrNotAssigned
	}
	if *assigned != employeeID {
		return nil, ErrAlreadyAssigned
	}
	var scoringID *string
	err = tx.QueryRow(ctx, `SELECT (SELECT id FROM scoring_results WHERE application_id=$1 AND data_version=$2 ORDER BY created_at DESC,id DESC LIMIT 1)`, id, version).Scan(&scoringID)
	if err != nil {
		return nil, fmt.Errorf("read decision scoring: %w", err)
	}
	var createdAt time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&createdAt); err != nil {
		return nil, err
	}
	result := &DecisionResult{ApplicationID: id, DataVersion: version, Status: "REJECTED"}
	result.Decision = DecisionRecord{
		ID: uuid.NewString(), ApplicationID: id, ScoringID: scoringID, DecidedByUserID: &employeeID,
		DataVersion: version, DecisionType: req.Action, DecisionSource: "MANUAL", ReasonCode: req.ReasonCode,
		Comment: req.Comment, PolicyVersion: "manual-v1", CreatedAt: createdAt,
	}
	if req.Action != ActionReject {
		if scoringID == nil || amount == nil || term == nil {
			return nil, ErrDecisionNotReady
		}
		var confirmed bool
		err = tx.QueryRow(ctx, `SELECT COUNT(*)=6 AND COALESCE(bool_and(confirmed_at IS NOT NULL AND confirmed_by_user_id IS NOT NULL),false)
   FROM financial_records WHERE application_id=$1 AND data_version=$2`, id, version).Scan(&confirmed)
		if err != nil {
			return nil, fmt.Errorf("check decision financials: %w", err)
		}
		if !confirmed {
			return nil, ErrDecisionNotReady
		}
		var minAmount, maxAmount int64
		var minTerm, maxTerm, rate int
		var active bool
		err = tx.QueryRow(ctx, `SELECT min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,base_annual_rate_bps,is_active
   FROM products WHERE code=$1 FOR SHARE`, productCode).Scan(&minAmount, &maxAmount, &minTerm, &maxTerm, &rate, &active)
		if err != nil {
			return nil, fmt.Errorf("read offer product: %w", err)
		}
		if savedRate != nil {
			rate = *savedRate
		}
		offerAmount, offerTerm := *amount, *term
		if req.Action == ActionCounterOffer {
			offerAmount, offerTerm, rate = *req.Offer.AmountKopecks, *req.Offer.TermMonths, *req.Offer.AnnualRateBps
		}
		if !active || offerAmount < minAmount || offerAmount > maxAmount || offerTerm < minTerm || offerTerm > maxTerm {
			return nil, ErrProductLimits
		}
		payment, err := monthlyPayment(offerAmount, offerTerm, rate)
		if err != nil {
			return nil, err
		}
		if !req.ValidUntil.After(createdAt) {
			return nil, ErrInvalidInput
		}
		result.Status = "OFFERED"
		result.Offer = &Offer{ID: uuid.NewString(), DecisionID: result.Decision.ID, Version: 1, AmountKopecks: offerAmount,
			TermMonths: offerTerm, AnnualRateBps: rate, MonthlyPaymentKopecks: payment, Status: "PENDING", CreatedAt: createdAt, ValidUntil: req.ValidUntil.UTC()}
	}
	d := result.Decision
	_, err = tx.Exec(ctx, `INSERT INTO decisions(id,application_id,scoring_id,decided_by_user_id,data_version,decision_type,decision_source,reason_code,comment,policy_version,created_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, d.ID, d.ApplicationID, d.ScoringID, d.DecidedByUserID, d.DataVersion, d.DecisionType, d.DecisionSource, d.ReasonCode, d.Comment, d.PolicyVersion, d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("save bank decision: %w", err)
	}
	if o := result.Offer; o != nil {
		_, err = tx.Exec(ctx, `INSERT INTO offers(id,decision_id,version,amount_kopecks,term_months,annual_rate_bps,monthly_payment_kopecks,status,created_at,valid_until)
   VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, o.ID, o.DecisionID, o.Version, o.AmountKopecks, o.TermMonths, o.AnnualRateBps, o.MonthlyPaymentKopecks, o.Status, o.CreatedAt, o.ValidUntil)
		if err != nil {
			return nil, fmt.Errorf("save bank offer: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET status=$2,updated_at=$3 WHERE id=$1`, id, result.Status, createdAt)
	if err != nil {
		return nil, fmt.Errorf("update decision status: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO status_history(id,application_id,changed_by_user_id,from_status,to_status,reason,created_at)
 VALUES ($1,$2,$3,'MANUAL',$4,$5,$6)`, uuid.NewString(), id, employeeID, result.Status, req.ReasonCode+": "+req.Comment, createdAt)
	if err != nil {
		return nil, fmt.Errorf("save decision history: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE info_requests SET status='RESOLVED',resolved_at=clock_timestamp() WHERE application_id=$1 AND status='RESPONDED'`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit bank decision: %w", err)
	}
	return result, nil
}

type lockedBankApplication struct {
	status   string
	version  int
	assigned *string
}

func lockBankApplication(ctx context.Context, tx pgx.Tx, id, employee string) (*lockedBankApplication, error) {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 AND role='BANK_EMPLOYEE' FOR SHARE`, employee).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, err
	}
	var result lockedBankApplication
	err = tx.QueryRow(ctx, `SELECT status,data_version,assigned_employee_id FROM applications WHERE id=$1 AND status<>'DRAFT' AND submitted_at IS NOT NULL FOR UPDATE`, id).Scan(&result.status, &result.version, &result.assigned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}
func requireAssignee(app *lockedBankApplication, employee string, version int) error {
	if app.version != version {
		return ErrVersionConflict
	}
	if app.assigned == nil {
		return ErrNotAssigned
	}
	if *app.assigned != employee {
		return ErrAlreadyAssigned
	}
	return nil
}
func (r *PostgresRepository) CreateInfoRequest(ctx context.Context, id, employee string, req InfoRequestInput) (*application.InfoRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	app, err := lockBankApplication(ctx, tx, id, employee)
	if err != nil {
		return nil, err
	}
	if app.status != "MANUAL" {
		return nil, ErrNotManual
	}
	if err = requireAssignee(app, employee, req.DataVersion); err != nil {
		return nil, err
	}
	var open bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM info_requests WHERE application_id=$1 AND status='OPEN')`, id).Scan(&open); err != nil {
		return nil, err
	}
	if open {
		return nil, ErrInfoRequestOpen
	}
	items, err := json.Marshal(req.RequiredItems)
	if err != nil {
		return nil, err
	}
	var data []byte
	err = tx.QueryRow(ctx, `INSERT INTO info_requests(id,application_id,created_by_user_id,description,required_items,status,created_at)
 VALUES ($1,$2,$3,$4,$5,'OPEN',clock_timestamp()) RETURNING to_jsonb(info_requests)`, uuid.NewString(), id, employee, req.Description, items).Scan(&data)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET status='NEEDS_INFO',updated_at=clock_timestamp() WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO status_history(id,application_id,changed_by_user_id,from_status,to_status,reason,created_at)
 VALUES ($1,$2,$3,'MANUAL','NEEDS_INFO',$4,clock_timestamp())`, uuid.NewString(), id, employee, req.Description)
	if err != nil {
		return nil, err
	}
	var result application.InfoRequest
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &result, nil
}
func (r *PostgresRepository) ChangeAssignment(ctx context.Context, id, employee string, version int, target *string) (*AssignmentChange, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	app, err := lockBankApplication(ctx, tx, id, employee)
	if err != nil {
		return nil, err
	}
	if app.status != "MANUAL" && app.status != "NEEDS_INFO" {
		return nil, ErrNotManual
	}
	if err = requireAssignee(app, employee, version); err != nil {
		return nil, err
	}
	if target != nil {
		var role string
		err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1 AND role='BANK_EMPLOYEE' FOR SHARE`, *target).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		if err != nil {
			return nil, err
		}
	}
	var updated time.Time
	if target != nil && *target == employee {
		err = tx.QueryRow(ctx, `SELECT updated_at FROM applications WHERE id=$1`, id).Scan(&updated)
	} else {
		err = tx.QueryRow(ctx, `UPDATE applications SET assigned_employee_id=$2,updated_at=clock_timestamp() WHERE id=$1 RETURNING updated_at`, id, target).Scan(&updated)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &AssignmentChange{ApplicationID: id, AssignedEmployeeID: target, DataVersion: version, Status: app.status, UpdatedAt: updated}, nil
}

func (r *PostgresRepository) checkReadEmployee(ctx context.Context, employee string) error {
	var valid bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='BANK_EMPLOYEE')`, employee).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return ErrForbidden
	}
	return nil
}
func (r *PostgresRepository) ListApplications(ctx context.Context, employee, view string, f ListFilter) (*ApplicationPage, error) {
	if err := r.checkReadEmployee(ctx, employee); err != nil {
		return nil, err
	}
	const query = `WITH details AS (
 SELECT a.id,COALESCE(c.name,'') company_name,COALESCE(c.inn,'') inn,a.product_code,a.requested_amount_kopecks,a.requested_term_months,a.data_version,
 CASE WHEN a.status='OFFERED' AND NOT EXISTS(SELECT 1 FROM offers o JOIN decisions d ON d.id=o.decision_id
  WHERE d.application_id=a.id AND d.data_version=a.data_version AND o.status='PENDING' AND o.valid_until>statement_timestamp()) THEN 'OFFER_EXPIRED' ELSE a.status END status,
 (SELECT score FROM scoring_results s WHERE s.application_id=a.id AND s.data_version=a.data_version ORDER BY s.created_at DESC,s.id DESC LIMIT 1) score,
 a.submitted_at,a.assigned_employee_id,
 (SELECT MAX(created_at) FROM status_history h WHERE h.application_id=a.id AND h.to_status='MANUAL') manual_review_started_at
 FROM applications a LEFT JOIN companies c ON c.id=a.company_id
 WHERE a.submitted_at IS NOT NULL AND a.status<>'DRAFT' AND EXISTS(SELECT 1 FROM users WHERE id=$5 AND role='BANK_EMPLOYEE')
 ), filtered AS (
 SELECT * FROM details WHERE
 (($1='manual' AND status='MANUAL') OR ($1='archive' AND status IN ('REJECTED','ACCEPTED','OFFER_DECLINED','OFFER_EXPIRED','SIGNED','DISBURSED','CLOSED','CANCELLED')))
 AND ($2='' OR strpos(lower(company_name),lower($2))>0 OR strpos(inn,$2)>0 OR strpos(id::text,lower($2))>0)
 AND ($3='' OR status=$3)
 AND ($4='' OR ($4='unassigned' AND assigned_employee_id IS NULL) OR assigned_employee_id::text=$4)
 ), page AS (SELECT * FROM filtered ORDER BY COALESCE(manual_review_started_at,submitted_at) DESC,id DESC LIMIT $6 OFFSET $7)
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY COALESCE(manual_review_started_at,submitted_at) DESC,id DESC) FROM page),'[]'::jsonb),
 'total',(SELECT count(*) FROM filtered),'limit',$6::int,'offset',$7::int)`
	var data []byte
	err := r.pool.QueryRow(ctx, query, view, f.Query, f.Status, f.Assigned, employee, f.Limit, f.Offset).Scan(&data)
	if err != nil {
		return nil, err
	}
	var result ApplicationPage
	err = json.Unmarshal(data, &result)
	return &result, err
}
func (r *PostgresRepository) ListDecisions(ctx context.Context, employee string, f ListFilter) (*DecisionPage, error) {
	if err := r.checkReadEmployee(ctx, employee); err != nil {
		return nil, err
	}
	const query = `WITH filtered AS (
 SELECT d.*,COALESCE(c.name,'') company_name,COALESCE(c.inn,'') inn,a.status application_status
 FROM decisions d JOIN applications a ON a.id=d.application_id LEFT JOIN companies c ON c.id=a.company_id
 WHERE d.decided_by_user_id=$1 AND EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='BANK_EMPLOYEE')
 AND ($2='' OR strpos(lower(COALESCE(c.name,'')),lower($2))>0 OR strpos(COALESCE(c.inn,''),$2)>0 OR strpos(a.id::text,lower($2))>0)
 AND ($3='' OR a.status=$3)
 ), page AS (SELECT * FROM filtered ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5)
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY created_at DESC,id DESC) FROM page),'[]'::jsonb),
 'total',(SELECT count(*) FROM filtered),'limit',$4::int,'offset',$5::int)`
	var data []byte
	err := r.pool.QueryRow(ctx, query, employee, f.Query, f.Status, f.Limit, f.Offset).Scan(&data)
	if err != nil {
		return nil, err
	}
	var result DecisionPage
	err = json.Unmarshal(data, &result)
	return &result, err
}
