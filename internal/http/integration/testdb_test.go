package integration__test

import (
	"os"
	"strings"
	"testing"
)

func requiredTestDBDSN(t *testing.T) string {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DB_DSN"))
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set; skipping DB-backed integration test")
	}

	return dsn
}
