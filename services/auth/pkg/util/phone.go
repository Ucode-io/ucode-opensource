package util

import "regexp"

// IsValidPhone ...
func IsValidPhone(phone string) bool {
	r := regexp.MustCompile(`^\+[0-9]{7,15}$`)
	return r.MatchString(phone)
}

func IsValidTin(tin string) bool {
	r := regexp.MustCompile(`^[0-9]+$`) // only digits
	return r.MatchString(tin)
}
