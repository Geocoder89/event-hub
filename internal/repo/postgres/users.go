package postgres

import (
	"context"
	"errors"

	"github.com/geocoder89/eventhub/internal/domain/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type UsersRepo struct {
	pool *pgxpool.Pool
}

func NewUsersRepo(pool *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{pool: pool}
}

func (r *UsersRepo) GetByEmail(ctx context.Context, email string) (user.User, error) {
	var u user.User

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, email, password_hash, name, role, created_at, updated_at
         FROM users
         WHERE email = $1`,
		email,
	).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Name,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			
			return user.User{}, ErrUserNotFound
		}

		return user.User{},err
	}
	return u,nil
}
