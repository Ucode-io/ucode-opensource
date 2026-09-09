package util

import (
	"errors"
	"regexp"
)

// IfElse evaluates a condition, if true returns the first parameter otherwise the second
func IfElse(condition bool, a any, b any) any {
	if condition {
		return a
	}
	return b
}

func ValidStrongPassword(password string) error {
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters long")
	}

	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !regexp.MustCompile(`\d`).MatchString(password) {
		return errors.New("password must contain at least one digit")
	}

	return nil
}
