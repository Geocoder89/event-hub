package jobs

type JobType string


const (
	JobPublishEvent JobType = "publish_event"


	// Future use cases 

	JobSendRegistrationConfirmation JobType = "send_registration_confirmation"
	JobExportRegistrationsCSV       JobType = "export_registrations_csv"
)

// check to see if the job type is a known constant 


func (t JobType) IsValid() bool {
	switch t {
	case JobPublishEvent, JobSendRegistrationConfirmation, JobExportRegistrationsCSV:
		return true
	default:
		return false
	}
}