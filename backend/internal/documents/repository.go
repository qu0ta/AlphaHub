package documents

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qu0ta/AlphaHub/internal/dsclient"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}
func (r *PostgresRepository) Save(ctx context.Context, doc Document, userID string, storageKey string, expectedVersion int, extraction *dsclient.ExtractFinancialsResponse) (*Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin document tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var version int
	var inn *string

	err = tx.QueryRow(ctx, `
		SELECT a.status, a.data_version, c.inn
 FROM applications a LEFT JOIN companies c ON c.id = a.company_id
		WHERE a.id = $1
 AND a.created_by_user_id = $2
 FOR UPDATE OF a
	`, doc.ApplicationID, userID).Scan(&status, &version, &inn)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}
	if status != "DRAFT" && status != "NEEDS_INFO" {
		return nil, ErrNotDraft
	}

	if version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if inn == nil {
		return nil, ErrCompanyRequired
	}
	if extraction == nil && doc.DocumentType != "SUPPORTING_DOCUMENT" {
		return nil, ErrInvalidInput
	}
	if extraction != nil && *inn != extraction.INN {
		return nil, ErrINNMismatch
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO documents (
			id,
			application_id,
			uploaded_by_user_id,
			document_type,
			original_filename,
			storage_key,
			processing_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING uploaded_at
	`,
		doc.ID,
		doc.ApplicationID,
		userID,
		doc.DocumentType,
		doc.OriginalFilename,
		storageKey,
		doc.ProcessingStatus,
	).Scan(&doc.UploadedAt)

	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}

	if doc.DocumentType == "SUPPORTING_DOCUMENT" {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &doc, nil
	}

	newVersion := version + 1
	for _, month := range extraction.Financials {
		_, err = tx.Exec(ctx, `INSERT INTO financial_records
   (id, application_id, source_document_id, data_version, period_month,
    revenue_kopecks, operating_inflow_kopecks, operating_outflow_kopecks, tax_outflow_kopecks,
    operating_expenses_kopecks, existing_debt_payment_kopecks)
   VALUES ($1,$2,$3,$4,$5::date,$6,$7,$8,$9,NULL,NULL)`,
			uuid.NewString(), doc.ApplicationID, doc.ID, newVersion, month.PeriodMonth+"-01",
			month.RevenueKopecks, month.OperatingInflowKopecks, month.OperatingOutflowKopecks, month.TaxOutflowKopecks)
		if err != nil {
			return nil, fmt.Errorf("save extracted month: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET data_version=$2,
 declared_monthly_debt_payment_kopecks=$3, updated_at=now() WHERE id=$1`,
		doc.ApplicationID, newVersion, extraction.DeclaredMonthlyDebtPaymentKopecks)
	if err != nil {
		return nil, fmt.Errorf("save extracted debt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit document: %w", err)
	}

	return &doc, nil
}
func (r *PostgresRepository) List(ctx context.Context, applicationID string, userID string) ([]Document, error) {
	// Не раскрываем документы чужой заявки.
	var exists bool

	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM applications
			WHERE id = $1
			  AND created_by_user_id = $2
		)
	`, applicationID, userID).Scan(&exists)

	if err != nil {
		return nil, fmt.Errorf("check application ownership: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			d.id,
			d.application_id,
			d.document_type,
			d.original_filename,
			d.processing_status,
			d.uploaded_at
		FROM documents AS d
		WHERE d.application_id = $1
		ORDER BY d.uploaded_at DESC, d.id DESC
	`, applicationID)

	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	result := make([]Document, 0)

	for rows.Next() {
		var doc Document

		if err := rows.Scan(
			&doc.ID,
			&doc.ApplicationID,
			&doc.DocumentType,
			&doc.OriginalFilename,
			&doc.ProcessingStatus,
			&doc.UploadedAt,
		); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}

		result = append(result, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return result, nil
}

// Check ownership before sending any document to DS. Save rechecks under a lock.
func (r *PostgresRepository) PrepareUpload(ctx context.Context, applicationID, userID string) (*UploadContext, error) {
	var result UploadContext
	var status string
	var inn *string
	err := r.pool.QueryRow(ctx, `SELECT a.status, a.data_version, c.inn
 FROM applications a LEFT JOIN companies c ON c.id=a.company_id
 WHERE a.id=$1 AND a.created_by_user_id=$2`, applicationID, userID).Scan(&status, &result.Version, &inn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("prepare PDF upload: %w", err)
	}
	if status != "DRAFT" && status != "NEEDS_INFO" {
		return nil, ErrNotDraft
	}
	if inn == nil {
		return nil, ErrCompanyRequired
	}
	result.INN = *inn
	return &result, nil
}

// Delete удаляет документ черновика вместе с финансами, извлечёнными из него.
// Версия заявки увеличивается: прежнее подтверждение финансов перестаёт действовать.
// Возвращает storage_key, чтобы сервис удалил сам файл после фиксации транзакции.
func (r *PostgresRepository) Delete(ctx context.Context, appID, docID, userID string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin document delete: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1 AND created_by_user_id=$2 FOR UPDATE`, appID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock application for document delete: %w", err)
	}
	if status != "DRAFT" {
		return "", ErrNotDraft
	}

	var storageKey string
	err = tx.QueryRow(ctx, `SELECT storage_key FROM documents WHERE id=$1 AND application_id=$2`, docID, appID).Scan(&storageKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrDocumentNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read document for delete: %w", err)
	}

	if _, err = tx.Exec(ctx, `DELETE FROM financial_records WHERE application_id=$1 AND source_document_id=$2`, appID, docID); err != nil {
		return "", fmt.Errorf("delete document financials: %w", err)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM documents WHERE id=$1 AND application_id=$2`, docID, appID); err != nil {
		return "", fmt.Errorf("delete document: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE applications SET data_version=data_version+1, updated_at=now() WHERE id=$1`, appID); err != nil {
		return "", fmt.Errorf("bump application version: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit document delete: %w", err)
	}
	return storageKey, nil
}

func (r *PostgresRepository) GetForDownload(ctx context.Context, appID, docID, user, role string) (*DownloadMetadata, error) {
	var result DownloadMetadata
	err := r.pool.QueryRow(ctx, `SELECT d.id,d.application_id,d.document_type,d.original_filename,d.processing_status,d.uploaded_at,d.storage_key
 FROM documents d JOIN applications a ON a.id=d.application_id JOIN users u ON u.id=$3
 WHERE a.id=$1 AND d.id=$2 AND u.role=$4 AND ((u.role='CLIENT' AND a.created_by_user_id=u.id) OR (u.role='BANK_EMPLOYEE' AND a.status<>'DRAFT' AND a.submitted_at IS NOT NULL))`, appID, docID, user, role).Scan(
		&result.ID, &result.ApplicationID, &result.DocumentType, &result.OriginalFilename, &result.ProcessingStatus, &result.UploadedAt, &result.StorageKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read document metadata: %w", err)
	}
	return &result, nil
}
