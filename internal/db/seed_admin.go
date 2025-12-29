package db

import (
	"context"
	"errors"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/user"
	"github.com/geocoder89/eventhub/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureAdminUser(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return nil
	}

	// check if the user exists

	var dummy string

	err := pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, cfg.AdminEmail).Scan(&dummy)

	if err == nil {
		return nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	hash, err := security.HashPassword(cfg.AdminPassword)

	if err != nil {
		return err
	}

	now := time.Now().UTC()

	u := user.User{
		ID:           uuid.NewString(),
		Email:        cfg.AdminEmail,
		PasswordHash: hash,
		Name:         cfg.AdminName,
		Role:         cfg.AdminRole,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO USERS (id,email,password_hash, name,role,created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		`,
		u.ID, u.Email, u.PasswordHash, u.Name, u.Role, u.CreatedAt, u.UpdatedAt,
	)

	return err
}
