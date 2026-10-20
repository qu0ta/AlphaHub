package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func (p *PostgresRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT
			id::text,
			name,
			email,
			password_hash,
			role,
			COALESCE(company_id::text, '')
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var user User
	err := p.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CompanyID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
func (p *PostgresRepository) FindByID(ctx context.Context, id string) (*User, error) {
	const query = `
		SELECT
			id::text,
			name,
			email,
			password_hash,
			role,
			COALESCE(company_id::text, '')
		FROM users
		WHERE id = $1
	`

	var user User

	err := p.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CompanyID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}
func (p *PostgresRepository) Create(ctx context.Context, user User) error {
	const query = `
		INSERT INTO users (
			id,
			name,
			email,
			password_hash,
			role,
			company_id
		)
		VALUES (
			$1, $2, $3, $4, $5,
			NULLIF($6, '')::uuid
		)
	`

	_, err := p.pool.Exec(
		ctx,
		query,
		user.ID,
		user.Name,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.CompanyID,
	)

	if err != nil {

		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			return ErrUserExists
		}

		return err
	}

	return nil
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}
