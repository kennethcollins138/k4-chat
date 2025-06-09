package handlers

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/kdot/k4-chat/backend/internal/auth"
)

// WriteJSONResponse writes the given data as a JSON response with the specified HTTP status code.
// It sets the "Content-Type" header to "application/json" and encodes the provided data into JSON.
// If the data is nil, no JSON is written.
//
// Parameters:
//   - w: The http.ResponseWriter used to write the response.
//   - status: The HTTP status code to set in the response.
//   - data: The data to encode as JSON and write to the response.
func WriteJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// WriteErrorResponse writes an error response as JSON with a given HTTP status code, error message, and additional details.
// It constructs a JSON object containing the error message and details, then calls WriteJSONResponse to send the response.
//
// Parameters:
//   - w: The http.ResponseWriter used to write the response.
//   - status: The HTTP status code to set in the response.
//   - message: A string describing the error.
//   - details: Additional details about the error, which can be any type.
func WriteErrorResponse(w http.ResponseWriter, status int, message string, details interface{}) {
	response := map[string]interface{}{
		"error":   message,
		"details": details,
	}
	WriteJSONResponse(w, status, response)
}

// HandleValidationErrors is a helper for writing validation error responses.
func HandleValidationErrors(w http.ResponseWriter, err error) {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		errs := make([]string, 0, len(validationErrs))
		for _, ve := range validationErrs {
			errs = append(errs, ve.Error())
		}
		WriteErrorResponse(w, http.StatusBadRequest, "Validation failed", errs)
		return
	}
	WriteErrorResponse(w, http.StatusBadRequest, "Validation failed", err.Error())
}

// StatusResponseWriter is a custom response writer that captures status code
type StatusResponseWriter struct {
	http.ResponseWriter
	StatusCode   int
	BytesWritten int
}

// NewStatusResponseWriter creates a new status response writer
func NewStatusResponseWriter(w http.ResponseWriter) *StatusResponseWriter {
	return &StatusResponseWriter{
		ResponseWriter: w,
		StatusCode:     http.StatusOK, // Default to 200 OK
	}
}

// WriteHeader captures the status code and passes it to the wrapped writer
func (srw *StatusResponseWriter) WriteHeader(code int) {
	srw.StatusCode = code
	srw.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written
func (srw *StatusResponseWriter) Write(b []byte) (int, error) {
	n, err := srw.ResponseWriter.Write(b)
	srw.BytesWritten += n
	return n, err
}

// Hijack implements the http.Hijacker interface
func (srw *StatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := srw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Flush implements the http.Flusher interface
func (srw *StatusResponseWriter) Flush() {
	f, ok := srw.ResponseWriter.(http.Flusher)
	if ok {
		f.Flush()
	}
}

// Push implements the http.Pusher interface
func (srw *StatusResponseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := srw.ResponseWriter.(http.Pusher)
	if !ok {
		return errors.New("underlying ResponseWriter does not implement http.Pusher")
	}
	return p.Push(target, opts)
}

// mapErrorToHTTPStatus maps domain errors to appropriate HTTP status codes
func (h *AuthHandler) mapErrorToHTTPStatus(err error) int {
	// Check if it's a structured auth error
	if authErr := auth.GetAuthError(err); authErr != nil {
		switch authErr.Type {
		case "validation_error":
			return http.StatusBadRequest
		case "user_error":
			if authErr.Code == "USER_EXISTS" {
				return http.StatusConflict
			}
			return http.StatusBadRequest
		case "auth_error":
			return http.StatusUnauthorized
		case "rate_limit":
			return http.StatusTooManyRequests
		case "not_implemented":
			return http.StatusNotImplemented
		default:
			return http.StatusInternalServerError
		}
	}

	// Fallback to checking standard errors
	switch {
	case errors.Is(err, auth.ErrUserAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, auth.ErrInvalidEmail):
		return http.StatusBadRequest
	case errors.Is(err, auth.ErrWeakPassword):
		return http.StatusBadRequest
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, auth.ErrUserNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
