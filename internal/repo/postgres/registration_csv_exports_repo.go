package postgres

import (
	"context"
	"errors"

	"github.com/geocoder89/eventhub/internal/domain/registrationexport"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistrationCSVExportsRepo struct {
	pool *pgxpool.Pool
}

func NewRegistrationCSVExportsRepo(pool *pgxpool.Pool) *RegistrationCSVExportsRepo {
	return &RegistrationCSVExportsRepo{pool: pool}
}

func (r *RegistrationCSVExportsRepo) Save(ctx context.Context, export registrationexport.CSVExport) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO registration_csv_exports (
			job_id, event_id, requested_by, file_name, content_type, row_count, csv_data, created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8
		)
	`,
		export.JobID,
		export.EventID,
		nullableStringPtr(export.RequestedBy),
		export.FileName,
		export.ContentType,
		export.RowCount,
		export.Data,
		export.CreatedAt,
	)
	return err
}

func (r *RegistrationCSVExportsRepo) GetByJobID(ctx context.Context, jobID string) (registrationexport.CSVExport, error) {
	var out registrationexport.CSVExport

	err := r.pool.QueryRow(ctx, `
		SELECT job_id, event_id, requested_by, file_name, content_type, row_count, csv_data, created_at
		FROM registration_csv_exports
		WHERE job_id = $1
	`, jobID).Scan(
		&out.JobID,
		&out.EventID,
		&out.RequestedBy,
		&out.FileName,
		&out.ContentType,
		&out.RowCount,
		&out.Data,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return registrationexport.CSVExport{}, registrationexport.ErrNotFound
		}
		return registrationexport.CSVExport{}, err
	}

	return out, nil
}

func nullableStringPtr(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}

	return *s
}
