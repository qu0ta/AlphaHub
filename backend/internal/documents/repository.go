package documents

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

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}
func (r *PostgresRepository) Save(ctx context.Context, doc Document, userID string, storageKey string) (*Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin document tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM applications
		WHERE id = $1
		  AND created_by_user_id = $2
		FOR UPDATE
	`, doc.ApplicationID, userID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock application: %w", err)
	}
	if status != "DRAFT" {
		return nil, ErrNotDraft
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
