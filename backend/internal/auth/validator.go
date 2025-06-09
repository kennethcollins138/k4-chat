package auth

import (
	"strings"
	"unicode"
)

// InputValidator interface for input validation
type InputValidator interface {
	ValidateEmail(email string) error
	ValidateUsername(username string) error
	ValidatePassword(password string) error
}

// DefaultValidator implements basic input validation
type DefaultValidator struct{}

func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{}
}

func (v *DefaultValidator) ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrMissingRequiredField
	}

	// Basic email validation (more comprehensive validation would use regex)
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return ErrInvalidEmail
	}

	if len(email) > 254 {
		return ErrFieldTooLong
	}

	return nil
}

func (v *DefaultValidator) ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrMissingRequiredField
	}

	if len(username) < 3 {
		return ErrFieldTooShort
	}

	if len(username) > 50 {
		return ErrFieldTooLong
	}

	// Username should be alphanumeric with optional underscores/hyphens
	for _, r := range username {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return ErrInvalidFormat
		}
	}

	return nil
}

func (v *DefaultValidator) ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	if len(password) > 128 {
		return ErrPasswordTooLong
	}

	// Check for at least one letter and one number
	hasLetter := false
	hasNumber := false

	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasNumber = true
		}
	}

	if !hasLetter || !hasNumber {
		return ErrWeakPassword
	}

	return nil
}
