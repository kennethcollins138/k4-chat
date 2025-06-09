package apperrors

import (
	"errors"
	"fmt"
)

type AppError struct {
	Code    int
	Message string
	Cause   error
	Meta    map[string]interface{}
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// Helper to check if an error is an AppError with a specific code
func Is(err error, code int) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// Generic constructor
func New(code int, msg string, cause error) *AppError {
	return &AppError{Code: code, Message: msg, Cause: cause}
}
