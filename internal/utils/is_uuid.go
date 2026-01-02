package utils

import "github.com/google/uuid"

func IsUUID(s string) bool {
	_, err := uuid.Parse(s)

	return err == nil
}
