package jobs

import (
	"encoding/json"
	"fmt"
)


func EncodePayload (t JobType, payload any)([]byte, error) {
	if !t.IsValid() {
		return nil, ErrInvalidJobType
	}

	switch t {
	case JobPublishEvent:
		_, ok := payload.(PublishEventPayload)

		if !ok {
			_, ok2 := payload.(*PublishEventPayload) 

			if !ok2 {
				return nil, ErrPayloadTypeMismatch
			}
		}

	case JobSendRegistrationConfirmation:
		_,ok := payload.(SendRegistrationConfirmationPayload) 

		if !ok {
			_, ok2 := payload.(*SendRegistrationConfirmationPayload)

			if !ok2 {
				return nil, ErrPayloadTypeMismatch
			}
		}

	case JobExportRegistrationsCSV:
			_,ok := payload.(ExportRegistrationsCSVPayload) 

		if !ok {
			_, ok2 := payload.(*ExportRegistrationsCSVPayload)

			if !ok2 {
				return nil, ErrPayloadTypeMismatch
			}
		}
		
	}


	b, err := json.Marshal(payload)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJobPayload, err)
	}
	
	return b, nil
}


// DecodePayload unmarshals job.Payload into the correct typed payload struct.
func DecodePayload(j Job) (any, error) {
	if !j.Type.IsValid() {
		return nil, ErrInvalidJobType
	}
	if len(j.Payload) == 0 {
		return nil, ErrInvalidJobPayload
	}

	switch j.Type {
	case JobPublishEvent:
		var p PublishEventPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidJobPayload, err)
		}
		return p, nil

	case JobSendRegistrationConfirmation:
		var p SendRegistrationConfirmationPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidJobPayload, err)
		}
		return p, nil

	case JobExportRegistrationsCSV:
		var p ExportRegistrationsCSVPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidJobPayload, err)
		}
		return p, nil

	default:
		return nil, ErrInvalidJobType
	}
}
