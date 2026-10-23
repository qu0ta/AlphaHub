package company

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

const companyColumns = `
	c.id,
	c.inn,
	c.name,
	c.business_type,
	c.registration_date,
	c.main_okved
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompany(row rowScanner) (*Company, error) {
	var company Company

	err := row.Scan(
		&company.ID,
		&company.INN,
		&company.Name,
		&company.BusinessType,
		&company.RegistrationDate,
		&company.MainOKVED,
	)
	if err != nil {
		return nil, err
	}

	return &company, nil
}
func (r *PostgresRepository) CreateForUser(ctx context.Context, userID string, company Company) (*Company, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin company creation: %w", err)

	}
	defer tx.Rollback(ctx)

	var currentCompanyID *string
	err = tx.QueryRow(ctx,
		`SELECT company_id FROM users WHERE id = $1 FOR UPDATE`,
		userID).Scan(&currentCompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock company user: %w", err)
	}
	if currentCompanyID != nil {
		return nil, ErrAlreadyLinked
	}
	const insertQuery = `INSERT INTO companies AS c (id, inn, name, business_type, registration_date, main_okved) 
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING ` + companyColumns
	created, err := scanCompany(tx.QueryRow(ctx, insertQuery,
		company.ID, company.INN, company.Name, company.BusinessType, company.RegistrationDate, company.MainOKVED))
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" && pgErr.ConstraintName == "companies_inn_key" {
			return nil, ErrINNExists
		}
		return nil, fmt.Errorf("insert company: %w", err)
	}

	result, err := tx.Exec(ctx, `
    UPDATE users
    SET company_id = $1
    WHERE id = $2
      AND company_id IS NULL
`, created.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("link company to user: %w", err)
	}
	if result.RowsAffected() != 1 {
		return nil, ErrAlreadyLinked
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit company creation: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) GetByUserID(ctx context.Context, userID string) (*Company, error) {
	const query = `
		SELECT ` + companyColumns + `
		FROM companies AS c
		JOIN users AS u ON u.company_id = c.id
		WHERE u.id = $1
	`

	company, err := scanCompany(
		r.pool.QueryRow(ctx, query, userID),
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user company: %w", err)
	}

	return company, nil
}
func (r *PostgresRepository) GetByINN(ctx context.Context, inn string) (*Company, error) {
	const query = `
		SELECT
			c.id,
			c.inn,
			c.name,
			c.business_type,
			c.registration_date,
			c.main_okved
		FROM companies AS c
		WHERE c.inn = $1
	`

	company, err := scanCompany(
		r.pool.QueryRow(ctx, query, inn),
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get company by INN: %w", err)
	}

	return company, nil
}
func (r *PostgresRepository) CreateInvitation(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error {
	result, err := r.pool.Exec(ctx, `
        INSERT INTO company_invitations (
            token_hash,
            company_id,
            created_by_user_id,
            expires_at
        )
        SELECT $1, company_id, id, $3
        FROM users
        WHERE id = $2
          AND company_id IS NOT NULL
          AND role = 'CLIENT'
    `, tokenHash, userID, expiresAt)

	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
func (r *PostgresRepository) RedeemInvitation(ctx context.Context, userID string, tokenHash string) (*Company, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invitation tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Сначала блокируем пользователя.
	var currentCompanyID *string
	err = tx.QueryRow(ctx, `
        SELECT company_id
        FROM users
        WHERE id = $1
        FOR UPDATE
    `, userID).Scan(&currentCompanyID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock invitation user: %w", err)
	}
	if currentCompanyID != nil {
		return nil, ErrAlreadyLinked
	}

	var companyID string
	err = tx.QueryRow(ctx, `
        SELECT company_id
        FROM company_invitations
        WHERE token_hash = $1
          AND used_at IS NULL
          AND expires_at > now()
        FOR UPDATE
    `, tokenHash).Scan(&companyID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidInvitation
	}
	if err != nil {
		return nil, fmt.Errorf("lock invitation: %w", err)
	}

	_, err = tx.Exec(ctx, `
        UPDATE users
        SET company_id = $1
        WHERE id = $2
    `, companyID, userID)
	if err != nil {
		return nil, fmt.Errorf("link invited user: %w", err)
	}

	_, err = tx.Exec(ctx, `
        UPDATE company_invitations
        SET used_at = now(),
            used_by_user_id = $2
        WHERE token_hash = $1
    `, tokenHash, userID)
	if err != nil {
		return nil, fmt.Errorf("consume invitation: %w", err)
	}

	const query = `
        SELECT ` + companyColumns + `
        FROM companies AS c
        WHERE c.id = $1
    `

	joinedCompany, err := scanCompany(
		tx.QueryRow(ctx, query, companyID),
	)
	if err != nil {
		return nil, fmt.Errorf("load joined company: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit invitation: %w", err)
	}

	return joinedCompany, nil
}
