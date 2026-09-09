package util

import (
	"regexp"

	"github.com/google/uuid"
)

// IsValidPhone ...
func IsValidPhone(phone string) bool {
	r := regexp.MustCompile(`^\+998[0-9]{2}[0-9]{7}$`)
	return r.MatchString(phone)
}

// IsValidEmail ...
func IsValidEmail(email string) bool {
	r := regexp.MustCompile(`^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$`)
	return r.MatchString(email)
}

// IsValidLogin ...
func IsValidLogin(login string) bool {
	r := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{5,29}$`)
	return r.MatchString(login)
}

// IsValidUUID ...
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// IsValidLogin ...
func IsValidFunctionName(functionName string) bool {
	r := regexp.MustCompile(`^[a-z0-9-]*$`)
	return r.MatchString(functionName)
}
