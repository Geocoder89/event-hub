package jobs

import "errors"

var (
	ErrInvalidJobType     = errors.New("invalid job type")
	ErrInvalidJobStatus   = errors.New("invalid job status")
	ErrInvalidJobPayload  = errors.New("invalid job payload")
	ErrPayloadTypeMismatch = errors.New("payload type mismatch for job type")
)
