package handlers

import (
	"strings"
	"unicode"
)

// isValidPassword validates the password against the policy: min 8 chars, 1 uppercase, 1 number, 1 special char.
func isValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	var hasUpper, hasNumber, hasSpecial bool
	for _, r := range password {
		if unicode.IsUpper(r) {
			hasUpper = true
		} else if unicode.IsDigit(r) {
			hasNumber = true
		} else if strings.ContainsAny(string(r), "!@#$%^&*(),.?\":{}|<>") {
			hasSpecial = true
		}
	}
	return hasUpper && hasNumber && hasSpecial
}
