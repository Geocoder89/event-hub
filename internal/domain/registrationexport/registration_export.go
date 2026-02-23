package registrationexport

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("registration export not found")

type CSVExport struct {
	JobID       string
	EventID     string
	RequestedBy *string
	FileName    string
	ContentType string
	RowCount    int
	Data        []byte
	CreatedAt   time.Time
}
