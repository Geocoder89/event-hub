package jobs


type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobProcessing JobStatus = "processing"
	JobSucceeded JobStatus = "succeeded"
	JobFailed JobStatus = "failed"
)

func (s JobStatus) IsValid() bool {
	switch s {
	case JobPending,JobProcessing,JobSucceeded, JobFailed:
		return true
	default:
		return false
	}
}