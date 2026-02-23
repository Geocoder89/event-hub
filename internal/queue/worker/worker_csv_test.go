package worker

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/registration"
)

func TestBuildRegistrationsCSV(t *testing.T) {
	checkedInAt := time.Date(2026, 2, 22, 10, 30, 0, 0, time.UTC)
	createdAt1 := time.Date(2026, 2, 21, 9, 0, 0, 0, time.UTC)
	createdAt2 := createdAt1.Add(2 * time.Hour)

	regs := []registration.Registration{
		{
			ID:           "reg-1",
			EventID:      "event-1",
			UserID:       "user-1",
			Name:         "First User",
			Email:        "first@example.com",
			CheckInToken: "token-1",
			CheckedInAt:  nil,
			CreatedAt:    createdAt1,
		},
		{
			ID:           "reg-2",
			EventID:      "event-1",
			UserID:       "user-2",
			Name:         "Second User",
			Email:        "second@example.com",
			CheckInToken: "token-2",
			CheckedInAt:  &checkedInAt,
			CreatedAt:    createdAt2,
		},
	}

	out, err := buildRegistrationsCSV(regs)
	if err != nil {
		t.Fatalf("buildRegistrationsCSV returned error: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv output: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (header + 2), got %d", len(rows))
	}

	wantHeader := []string{
		"registration_id",
		"event_id",
		"user_id",
		"name",
		"email",
		"check_in_token",
		"checked_in_at",
		"created_at",
	}
	if strings.Join(rows[0], ",") != strings.Join(wantHeader, ",") {
		t.Fatalf("unexpected header: got=%v want=%v", rows[0], wantHeader)
	}

	if rows[1][0] != "reg-1" || rows[1][4] != "first@example.com" {
		t.Fatalf("unexpected first data row: %v", rows[1])
	}
	if rows[1][6] != "" {
		t.Fatalf("expected empty checked_in_at for first row, got %q", rows[1][6])
	}
	if rows[2][0] != "reg-2" || rows[2][4] != "second@example.com" {
		t.Fatalf("unexpected second data row: %v", rows[2])
	}
	if rows[2][6] != checkedInAt.Format(time.RFC3339) {
		t.Fatalf("unexpected checked_in_at for second row: got=%q want=%q", rows[2][6], checkedInAt.Format(time.RFC3339))
	}
}
