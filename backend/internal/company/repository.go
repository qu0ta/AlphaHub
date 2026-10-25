package company

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
	// Компания с таким ИНН может уже существовать (демо-компании из миграции или
	// компания, которую сохранил другой пользователь). Тогда не создаём дубликат,
	// а привязываем пользователя к существующей. ON CONFLICT не обрывает транзакцию.
	const insertQuery = `INSERT INTO companies AS c (id, inn, name, business_type, registration_date, main_okved) 
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (inn) DO NOTHING
	RETURNING ` + companyColumns
	created, err := scanCompany(tx.QueryRow(ctx, insertQuery,
		company.ID, company.INN, company.Name, company.BusinessType, company.RegistrationDate, company.MainOKVED))
	if errors.Is(err, pgx.ErrNoRows) {
		const existingQuery = `SELECT ` + companyColumns + ` FROM companies AS c WHERE c.inn = $1`
		created, err = scanCompany(tx.QueryRow(ctx, existingQuery, company.INN))
	}
	if err != nil {
		return nil, fmt.Errorf("insert or load company: %w", err)
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
