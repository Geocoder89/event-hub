package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type EventCursor struct {
	StartAt time.Time `json:"startAt"`
	ID      string    `json:"id"`
}

type RegistrationCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func EncodeEventCursor(startAt time.Time, id string) (string, error) {
	b, err := json.Marshal(EventCursor{StartAt: startAt, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeEventCursor(cursor string) (EventCursor, error) {
	if cursor == "" {
		return EventCursor{}, errors.New("empty cursor")
	}

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return EventCursor{}, err
	}

	var c EventCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return EventCursor{}, err
	}
	if c.ID == "" || c.StartAt.IsZero() {
		return EventCursor{}, errors.New("invalid cursor payload")
	}
	return c, nil
}

type JobCursor struct {
	UpdatedAt time.Time `json:"updatedAt"`
	ID        string    `json:"id"`
}

func EncodeRegistrationCursor(createdAt time.Time, id string) (string, error) {
	b, err := json.Marshal(RegistrationCursor{CreatedAt: createdAt, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeRegistrationCursor(cursor string) (RegistrationCursor, error) {
	if cursor == "" {
		return RegistrationCursor{}, errors.New("empty cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return RegistrationCursor{}, err
	}
	var c RegistrationCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return RegistrationCursor{}, err
	}
	if c.ID == "" || c.CreatedAt.IsZero() {
		return RegistrationCursor{}, errors.New("invalid cursor payload")
	}
	return c, nil
}

func EncodeJobCursor(updatedAt time.Time, id string) (string, error) {
	b, err := json.Marshal(JobCursor{UpdatedAt: updatedAt, ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeJobCursor(cursor string) (JobCursor, error) {
	if cursor == "" {
		return JobCursor{}, errors.New("empty cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return JobCursor{}, err
	}
	var c JobCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return JobCursor{}, err
	}
	if c.ID == "" || c.UpdatedAt.IsZero() {
		return JobCursor{}, errors.New("invalid cursor payload")
	}
	return c, nil
}
