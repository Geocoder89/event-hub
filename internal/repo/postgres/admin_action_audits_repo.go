package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminActionAuditsRepo struct {
	pool *pgxpool.Pool
}

func NewAdminActionAuditsRepo(pool *pgxpool.Pool) *AdminActionAuditsRepo {
	return &AdminActionAuditsRepo{pool: pool}
}

func (r *AdminActionAuditsRepo) Write(
	ctx context.Context,
	actorUserID, actorEmail, actorRole, action, resourceType, resourceID, requestID string,
	statusCode int,
	details map[string]any,
) error {
	detailsJSON := json.RawMessage(`{}`)
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		detailsJSON = b
	}

	now := time.Now().UTC()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO admin_action_audits (
			id,
			actor_user_id,
			actor_email,
			actor_role,
			action,
			resource_type,
			resource_id,
			request_id,
			status_code,
			details,
			created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11
		)
	`,
		uuid.NewString(),
		nullableString(actorUserID),
		nullableString(actorEmail),
		actorRole,
		action,
		resourceType,
		nullableString(resourceID),
		nullableString(requestID),
		statusCode,
		detailsJSON,
		now,
	)
	if err != nil {
		return err
	}

	return nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}

	return s
}
