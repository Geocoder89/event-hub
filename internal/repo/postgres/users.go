package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailAlreadyUsed = errors.New("email is already in use")

type UsersRepo struct {
	pool *pgxpool.Pool
}

func NewUsersRepo(pool *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{pool: pool}
}

func (r *UsersRepo) Create(ctx context.Context, email, passwordHash, name, role string) (user.User, error) {
	now := time.Now().UTC()
	u := user.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.CreatedAt, u.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return user.User{}, ErrEmailAlreadyUsed
		}
		return user.User{}, err
	}

	return u, nil

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

		return user.User{}, err
	}
	return u, nil
}
