package handlers

// Context keys for storing values in request context
// Using a custom type to avoid collisions with other packages
type contextKey string

const (
	// Context keys for auth middleware values
	ContextKeyUser         contextKey = "user"
	ContextKeyClaims       contextKey = "claims"
	ContextKeyUserID       contextKey = "user_id"
	ContextKeySessionID    contextKey = "session_id"
	ContextKeyTokenVersion contextKey = "token_version"
	ContextKeyRegion       contextKey = "region"
	ContextKeySession      contextKey = "session"
	ContextKeyFreshTier    contextKey = "fresh_tier"
)
