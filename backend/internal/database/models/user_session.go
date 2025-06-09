package models

import (
	"time"

	"github.com/google/uuid"
)

/*
UserSession represents a persisted user authentication session.

This model supports:
  - Cross-device login and sync
  - Session expiration and refresh logic
  - Security and audit tracking (IP, device info)
  - Support for future access management (e.g., forced logout, session revocation)

Key Design Elements:

- SessionToken:
    A secure, unguessable token (e.g., JWT, opaque UUID). It is stored in the DB for validation,
    but **never serialized** into JSON responses to avoid accidental exposure in APIs.

- DeviceInfo:
    Optional user-agent metadata string for device tracking (e.g., "Chrome on MacOS").
    Useful for session history UI or admin audit tools.

- IPAddress:
    Captures originating IP address for login, supporting geo-awareness, analytics,
    or suspicious login detection. Can be extended to include geolocation in future.

- LastActiveAt:
    Used to track session heartbeat or TTL refresh (for idle timeout logic).
    Useful for token refresh windows, UI hints (e.g., "last seen X minutes ago"),
    or eventual session cleanup.

Scalability/Future-proofing Considerations:

- Add a `RevokedAt *time.Time` field to support forced logouts (e.g., admin action or password reset).
- Consider adding `UserAgentFingerprint` or `LoginMethod` fields for extended auth metadata.
- Integrate with security tools to monitor active sessions and detect anomalies.
- Consider a token rotation mechanism for long-lived sessions.

*/

type UserSession struct {
	ID           uuid.UUID `json:"id" db:"id"`                             // Unique session ID
	UserID       uuid.UUID `json:"user_id" db:"user_id"`                   // Associated user
	SessionToken string    `json:"-" db:"session_token"`                   // Never exposed in JSON APIs
	DeviceInfo   *string   `json:"device_info,omitempty" db:"device_info"` // Optional device string
	IPAddress    *string   `json:"ip_address,omitempty" db:"ip_address"`   // Optional IP capture
	CreatedAt    time.Time `json:"created_at" db:"created_at"`             // Session creation time
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`             // Hard expiration
	LastActiveAt time.Time `json:"last_active_at" db:"last_active_at"`     // Updated on each interaction
}
