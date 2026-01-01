package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRefreshTokenNotFound = errors.New("refresh not found")

type RefreshTokenRow struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
	CreatedAt  time.Time
}

type RefreshTokensRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshTokensRepo(pool *pgxpool.Pool) *RefreshTokensRepo {
	return &RefreshTokensRepo{pool: pool}
}

func (r *RefreshTokensRepo) Create(ctx context.Context, tx pgx.Tx, row RefreshTokenRow) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id,token_hash, expires_at, revoked_at, replaced_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		`,
		row.ID, row.UserID, row.TokenHash, row.ExpiresAt, row.RevokedAt, row.ReplacedBy, row.CreatedAt,
	)
	return err
}

// Locks the row to prevent concurrent refresh races

func (r *RefreshTokensRepo) GetForUpdate(ctx context.Context, tx pgx.Tx, id string) (RefreshTokenRow, error) {
	var row RefreshTokenRow

	err := tx.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by, created_at
		FROM refresh_tokens
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&row.ID,
		&row.UserID,
		&row.TokenHash,
		&row.ExpiresAt,
		&row.RevokedAt,
		&row.ReplacedBy,
		&row.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshTokenRow{}, ErrRefreshTokenNotFound
		}

		return RefreshTokenRow{}, err
	}

	return row, nil
}

func (r *RefreshTokensRepo) Revoke(ctx context.Context, tx pgx.Tx, id string, replacedBy *string) error {
	_, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW(), replaced_by = $2
		WHERE id = $1
	`, id, replacedBy)

	return err
}

func (r *RefreshTokensRepo) RevokeAllForUser(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)

	return err
}

func (r *RefreshTokensRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, pgx.TxOptions{})
}
