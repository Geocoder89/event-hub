package postgres

import (
	"context"
	"errors"
	"time"

	notificationsdelivery "github.com/geocoder89/eventhub/internal/domain/notifications_delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationsDeliveriesRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationsDeliveriesRepo(pool *pgxpool.Pool) *NotificationsDeliveriesRepo {
	return &NotificationsDeliveriesRepo{pool: pool}
}

func (r *NotificationsDeliveriesRepo) TryStartRegistration(
	ctx context.Context,
	jobID string,
	registrationID string,
	recipient string,
) error {
	kind := "registration.confirmation"

	// 1) Insert if missing
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_deliveries (kind, registration_id, job_id, recipient, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'sending', NOW(), NOW())
	`, kind, registrationID, jobID, recipient)

	if err == nil {
		return nil
	}
	if !IsUniqueViolation(err) {
		return err
	}

	// 2) Row exists. If it was failed, "claim" it for retry by switching back to sending.
	// This is atomic: only one worker can flip failed -> sending.
	tag, uErr := r.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'sending',
		    job_id = $3,
		    recipient = $4,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE kind = $1 AND registration_id = $2 AND status = 'failed'
	`, kind, registrationID, jobID, recipient)

	if uErr != nil {
		return uErr
	}
	if tag.RowsAffected() == 1 {
		return nil // we successfully claimed the retry
	}

	// 3) Not failed. Determine whether it's already sent or currently sending.
	var status string
	var sentAt *time.Time

	qErr := r.pool.QueryRow(ctx, `
		SELECT status, sent_at
		FROM notification_deliveries
		WHERE kind = $1 AND registration_id = $2
	`, kind, registrationID).Scan(&status, &sentAt)

	if qErr != nil {
		if errors.Is(qErr, pgx.ErrNoRows) {
			// row disappeared; let caller retry
			return nil
		}
		return qErr
	}

	if sentAt != nil || status == "sent" {
		return notificationsdelivery.ErrAlreadySent
	}

	// status == "sending"
	return notificationsdelivery.ErrInProgress
}

func (r *NotificationsDeliveriesRepo) MarkRegistrationConfirmationSent(
	ctx context.Context,
	registrationID string,
	providerMessageID *string,
) error {
	kind := "registration.confirmation"

	_, err := r.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'sent',
		    sent_at = NOW(),
		    provider_message_id = $3,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE kind = $1 AND registration_id = $2
	`, kind, registrationID, providerMessageID)

	return err
}

func (r *NotificationsDeliveriesRepo) MarkRegistrationConfirmationFailed(
	ctx context.Context,
	registrationID string,
	errMsg string,
) error {
	kind := "registration.confirmation"

	_, err := r.pool.Exec(ctx, `
		UPDATE notification_deliveries
		SET status = 'failed',
		    last_error = $3,
		    updated_at = NOW()
		WHERE kind = $1 AND registration_id = $2
	`, kind, registrationID, errMsg)

	return err
}
