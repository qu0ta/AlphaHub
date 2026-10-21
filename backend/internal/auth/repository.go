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

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const query = `
		SELECT
			id::text,
			fullname,
			email,
			password_hash,
			role,
			COALESCE(company_id::text, '')
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var user User
	err := scanUser(ctx, email, r, query, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func scanUser(ctx context.Context, email string, r *PostgresRepository, query string, user User) error {
	return r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CompanyID,
	)
}
func (r *PostgresRepository) FindByID(ctx context.Context, id string) (*User, error) {
	const query = `
		SELECT
			id::text,
			fullname,
			email,
			password_hash,
			role,
			COALESCE(company_id::text, '')
		FROM users
		WHERE id = $1
	`

	var user User

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CompanyID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}
func (r *PostgresRepository) Create(ctx context.Context, user User) error {
	const query = `
		INSERT INTO users (
			id,
			fullname,
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

	_, err := r.pool.Exec(
		ctx,
		query,
		user.ID,
		user.FullName,
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
