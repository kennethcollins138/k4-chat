package tokens

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateDeviceID(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		ipAddress string
		expected  string
	}{
		{
			name:      "valid inputs",
			userAgent: "Mozilla/5.0",
			ipAddress: "127.0.0.1",
			expected:  GenerateDeviceID("Mozilla/5.0", "127.0.0.1"),
		},
		{
			name:      "empty inputs",
			userAgent: "",
			ipAddress: "",
			expected:  GenerateDeviceID("", ""),
		},
		{
			name:      "special characters",
			userAgent: "Mozilla/5.0 (#1!)",
			ipAddress: "192.168.1.1",
			expected:  GenerateDeviceID("Mozilla/5.0 (#1!)", "192.168.1.1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDeviceID(tt.userAgent, tt.ipAddress)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		request  *http.Request
		expected string
	}{
		{
			name: "valid request",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("Accept-Language", "en-US")
				r.Header.Set("Accept-Encoding", "gzip")
				r.Header.Set("Accept", "text/html")
				r.RemoteAddr = "127.0.0.1"
				r.Header.Set("User-Agent", "Mozilla/5.0")
				return r
			}(),
			expected: GenerateFingerprint(func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("Accept-Language", "en-US")
				r.Header.Set("Accept-Encoding", "gzip")
				r.Header.Set("Accept", "text/html")
				r.RemoteAddr = "127.0.0.1"
				r.Header.Set("User-Agent", "Mozilla/5.0")
				return r
			}()),
		},
		{
			name: "missing headers",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.RemoteAddr = ""
				return r
			}(),
			expected: GenerateFingerprint(func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.RemoteAddr = ""
				return r
			}()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateFingerprint(tt.request)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractBindingData(t *testing.T) {
	tests := []struct {
		name       string
		request    *http.Request
		expectedID string
	}{
		{
			name: "complete data",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("User-Agent", "Mozilla/5.0")
				r.RemoteAddr = "127.0.0.1"
				return r
			}(),
			expectedID: GenerateDeviceID("Mozilla/5.0", "127.0.0.1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindingData := ExtractBindingData(tt.request)
			if bindingData.DeviceID != tt.expectedID {
				t.Errorf("expected %v, got %v", tt.expectedID, bindingData.DeviceID)
			}
		})
	}
}

func TestValidateBinding(t *testing.T) {
	tests := []struct {
		name        string
		binding     *TokenBinding
		request     *http.Request
		expectValid bool
	}{
		{
			name: "valid binding and request",
			binding: &TokenBinding{
				DeviceID:  GenerateDeviceID("Mozilla/5.0", "127.0.0.1"),
				UserAgent: "Mozilla/5.0",
				IPAddress: "127.0.0.1",
				Fingerprint: GenerateFingerprint(func() *http.Request {
					r := httptest.NewRequest(http.MethodGet, "/", nil)
					r.Header.Set("Accept-Language", "en-US")
					r.Header.Set("Accept-Encoding", "gzip")
					r.Header.Set("Accept", "text/html")
					r.RemoteAddr = "127.0.0.1"
					r.Header.Set("User-Agent", "Mozilla/5.0")
					return r
				}()),
			},
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("Accept-Language", "en-US")
				r.Header.Set("Accept-Encoding", "gzip")
				r.Header.Set("Accept", "text/html")
				r.RemoteAddr = "127.0.0.1"
				r.Header.Set("User-Agent", "Mozilla/5.0")
				return r
			}(),
			expectValid: true,
		},
		{
			name:        "nil binding",
			binding:     nil,
			request:     httptest.NewRequest(http.MethodGet, "/", nil),
			expectValid: false,
		},
		{
			name: "mismatched data",
			binding: &TokenBinding{
				DeviceID:    GenerateDeviceID("OtherAgent", "255.255.255.255"),
				UserAgent:   "OtherAgent",
				IPAddress:   "255.255.255.255",
				Fingerprint: GenerateFingerprint(httptest.NewRequest(http.MethodGet, "/", nil)),
			},
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set("User-Agent", "Mozilla/5.0")
				r.RemoteAddr = "127.0.0.1"
				return r
			}(),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBinding(tt.binding, tt.request)
			if result != tt.expectValid {
				t.Errorf("expected %v, got %v", tt.expectValid, result)
			}
		})
	}
}
