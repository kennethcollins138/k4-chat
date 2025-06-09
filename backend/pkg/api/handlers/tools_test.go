package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteJSONResponse_WithData verifies that WriteJSONResponse sets the correct headers,
// status code, and writes valid JSON when provided with non-nil data.
func TestWriteJSONResponse_WithData(t *testing.T) {
	rr := httptest.NewRecorder()
	data := map[string]interface{}{"foo": "bar"}
	statusCode := http.StatusOK

	WriteJSONResponse(rr, statusCode, data)

	// Check that the Content-Type header is correctly set.
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type to be application/json, got %q", ct)
	}

	// Check that the status code is set.
	if rr.Code != statusCode {
		t.Errorf("expected status code %d, got %d", statusCode, rr.Code)
	}

	// Verify that the body contains valid JSON matching our data.
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("error unmarshaling response: %v", err)
	}
	if response["foo"] != "bar" {
		t.Errorf("expected response foo to be %q, got %v", "bar", response["foo"])
	}
}

// TestWriteJSONResponse_WithNilData verifies that when nil is passed for data,
// WriteJSONResponse still sets headers and status code but leaves the body empty.
func TestWriteJSONResponse_WithNilData(t *testing.T) {
	rr := httptest.NewRecorder()
	statusCode := http.StatusNoContent

	WriteJSONResponse(rr, statusCode, nil)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type to be application/json, got %q", ct)
	}

	if rr.Code != statusCode {
		t.Errorf("expected status code %d, got %d", statusCode, rr.Code)
	}

	if rr.Body.Len() != 0 {
		t.Errorf("expected empty body when data is nil, got %q", rr.Body.String())
	}
}

// TestWriteErrorResponse verifies that WriteErrorResponse writes a JSON object
// containing both the error message and details.
func TestWriteErrorResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	statusCode := http.StatusBadRequest
	message := "something went wrong"
	details := "error details"

	WriteErrorResponse(rr, statusCode, message, details)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type to be application/json, got %q", ct)
	}

	if rr.Code != statusCode {
		t.Errorf("expected status code %d, got %d", statusCode, rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("error unmarshaling response: %v", err)
	}

	if response["error"] != message {
		t.Errorf("expected error message %q, got %q", message, response["error"])
	}

	if response["details"] != details {
		t.Errorf("expected details %q, got %q", details, response["details"])
	}
}
