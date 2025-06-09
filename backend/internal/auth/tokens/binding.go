package tokens

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// TokenBinding represents the binding data for a token
type TokenBinding struct {
	DeviceID    string `json:"device_id"`
	UserAgent   string `json:"user_agent"`
	IPAddress   string `json:"ip_address"`
	Fingerprint string `json:"fingerprint"`
}

// GenerateDeviceID creates a unique device ID based on user agent and IP
func GenerateDeviceID(userAgent, ipAddress string) string {
	data := fmt.Sprintf("%s:%s", userAgent, ipAddress)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GenerateFingerprint creates a unique fingerprint for the client
func GenerateFingerprint(r *http.Request) string {
	// Collect various client attributes
	attributes := []string{
		r.UserAgent(),
		r.Header.Get("Accept-Language"),
		r.Header.Get("Accept-Encoding"),
		r.Header.Get("Accept"),
		r.RemoteAddr,
	}

	// Create a hash of all attributes
	data := strings.Join(attributes, "|")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ExtractBindingData extracts binding data from the request
func ExtractBindingData(r *http.Request) *TokenBinding {
	deviceID := GenerateDeviceID(r.UserAgent(), r.RemoteAddr)
	fingerprint := GenerateFingerprint(r)

	return &TokenBinding{
		DeviceID:    deviceID,
		UserAgent:   r.UserAgent(),
		IPAddress:   r.RemoteAddr,
		Fingerprint: fingerprint,
	}
}

// ValidateBinding validates if the token binding matches the current request
func ValidateBinding(binding *TokenBinding, r *http.Request) bool {
	if binding == nil {
		return false
	}

	currentDeviceID := GenerateDeviceID(r.UserAgent(), r.RemoteAddr)
	currentFingerprint := GenerateFingerprint(r)

	return binding.DeviceID == currentDeviceID && binding.Fingerprint == currentFingerprint
}
