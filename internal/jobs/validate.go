package jobs

import "strings"

// ValidatePayload performs minimal validation on decoded payloads.
// (Full validation can be extended later, but this is enough for Day 30.)
func ValidatePayload(t JobType, payload any) error {
	if !t.IsValid() {
		return ErrInvalidJobType
	}

	trim := func(s string) string { return strings.TrimSpace(s) }

	switch t {
	case JobPublishEvent:
		var p PublishEventPayload
		switch v := payload.(type) {
		case PublishEventPayload:
			p = v
		case *PublishEventPayload:
			p = *v
		default:
			return ErrPayloadTypeMismatch
		}
		if trim(p.EventID) == "" {
			return ErrInvalidJobPayload
		}
		return nil

	case JobSendRegistrationConfirmation:
		var p SendRegistrationConfirmationPayload
		switch v := payload.(type) {
		case SendRegistrationConfirmationPayload:
			p = v
		case *SendRegistrationConfirmationPayload:
			p = *v
		default:
			return ErrPayloadTypeMismatch
		}
		if trim(p.RegistrationID) == "" || trim(p.UserID) == "" || trim(p.EventID) == "" {
			return ErrInvalidJobPayload
		}
		return nil

	case JobExportRegistrationsCSV:
		var p ExportRegistrationsCSVPayload
		switch v := payload.(type) {
		case ExportRegistrationsCSVPayload:
			p = v
		case *ExportRegistrationsCSVPayload:
			p = *v
		default:
			return ErrPayloadTypeMismatch
		}
		if trim(p.EventID) == "" {
			return ErrInvalidJobPayload
		}
		return nil

	default:
		return ErrInvalidJobType
	}
}
