package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
	"github.com/kdot/k4-chat/backend/configs"
	"go.uber.org/zap"
)

/*
SecurityMiddleware provides comprehensive HTTP security controls for the k4-chat API.

Key Responsibilities:
- Content Security Policy (CSP) enforcement with environment-specific policies
- Cross-Origin Resource Sharing (CORS) configuration and enforcement
- Security header injection (HSTS, XFO, XCTO, etc.)
- API response caching controls for sensitive data
- Session invalidation headers for logout endpoints
- Production vs development security posture management

Security Headers Applied:
- HSTS: Forces HTTPS in production environments
- CSP: Prevents XSS with strict policies in production, relaxed in development
- X-Frame-Options: Prevents clickjacking attacks
- X-Content-Type-Options: Prevents MIME type sniffing
- X-XSS-Protection: Legacy XSS protection (for older browsers)
- Referrer-Policy: Controls referrer information leakage
- Permissions-Policy: Restricts access to browser APIs
- Cross-Origin policies: COEP, COOP, CORP for isolation
- Expect-CT: Certificate Transparency enforcement in production
- Clear-Site-Data: Clears browser data on logout

CORS Behavior:
- Development: Allows additional localhost origins, maintains credentials
- Production: Strict origin allowlist from configuration
- Credential Safety: Automatically disables credentials if wildcard origins present

Design Notes:
- Environment-aware security policies (strict prod, permissive dev)
- Spec-compliant CORS implementation preventing credential leakage
- Graceful header chaining with proper middleware composition
- Performance-conscious header application (no unnecessary recalculation)
- Clear separation between CORS logic and general security headers

Integration Points:
Router -> SecurityMiddleware -> Application Handlers
Configuration from: configs.Envs.Server.Environment, configs.Envs.Server.FrontendURL
*/

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	Logger           *zap.Logger
}

// DefaultSecurityConfig returns a secure default configuration suitable for most applications.
// It includes the configured frontend URL and localhost for development convenience.
// All modern security headers are enabled with safe defaults.
func DefaultSecurityConfig(logger *zap.Logger) *SecurityConfig {
	return &SecurityConfig{
		AllowedOrigins:   []string{configs.Envs.Server.FrontendURL, "http://localhost"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes - balance between performance and security
		Logger:           logger,
	}
}

// SecurityHeaders applies a comprehensive set of security headers to all HTTP responses.
// Headers are environment-aware: production uses strict policies while development
// allows necessary flexibility for debugging and hot-reloading.
//
// Applied Headers:
// - HSTS: Production only, prevents protocol downgrade attacks
// - CSP: Strict in production (no eval/inline), permissive in development
// - Anti-clickjacking, MIME sniffing prevention, XSS protection
// - Cross-origin isolation policies for enhanced security
// - Cache control for sensitive API responses
// - Site data clearing for logout endpoints
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HSTS - Force HTTPS (only in production)
		// Development typically uses HTTP, so HSTS would break local testing
		if configs.Envs.Server.Environment == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Content Security Policy – environment-specific policies
		// Production: No unsafe-eval or unsafe-inline for maximum XSS protection
		// Development: Allows inline scripts/styles and eval for hot-reloading and debugging
		var cspPolicy string
		if configs.Envs.Server.Environment == "production" {
			cspPolicy = "default-src 'self'; " +
				"script-src 'self'; " + // No inline scripts in production
				"style-src 'self' 'unsafe-inline'; " + // Allow inline styles for CSS frameworks
				"img-src 'self' data: https:; " + // Images from self, data URLs, HTTPS
				"font-src 'self' data:; " + // Fonts from self and data URLs
				"connect-src 'self'; " + // XHR/fetch to same origin only
				"frame-ancestors 'none'; " + // Prevent framing (defense in depth with X-Frame-Options)
				"base-uri 'self'; " + // Restrict base tag to prevent injection
				"form-action 'self'" // Forms can only submit to same origin
		} else { // development/test environments
			cspPolicy = "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " + // Allow inline scripts and eval for dev tools
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self' data:; " +
				"connect-src 'self' ws:; " + // Allow WebSocket connections for hot-reloading
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
		}
		w.Header().Set("Content-Security-Policy", cspPolicy)

		// Prevent MIME type sniffing attacks
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking by disallowing framing
		w.Header().Set("X-Frame-Options", "DENY")

		// Legacy XSS protection for older browsers (modern browsers rely on CSP)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Control referrer information to prevent information leakage
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict access to powerful browser APIs
		w.Header().Set("Permissions-Policy",
			"geolocation=(), "+
				"microphone=(), "+
				"camera=(), "+
				"payment=(), "+
				"usb=(), "+
				"magnetometer=(), "+
				"gyroscope=(), "+
				"speaker=(), "+
				"ambient-light-sensor=(), "+
				"accelerometer=(), "+
				"battery=()")

		// Cross-origin isolation policies for enhanced security
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

		// Hide server information to reduce attack surface
		w.Header().Set("Server", "")

		// Certificate Transparency enforcement in production
		if configs.Envs.Server.Environment == "production" {
			w.Header().Set("Expect-CT", "max-age=86400, enforce")
		}

		// Clear browser data on logout endpoints for security
		if strings.HasSuffix(r.URL.Path, "/auth/signout") || strings.HasSuffix(r.URL.Path, "/auth/signout-all") {
			w.Header().Set("Clear-Site-Data", "\"cache\", \"cookies\", \"storage\"")
		}

		// Prevent caching of sensitive API responses
		if r.Header.Get("Accept") == "application/json" ||
			r.Header.Get("Content-Type") == "application/json" {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware creates a CORS middleware with environment-aware origin handling.
// It automatically prevents credential leakage by disabling credentials when wildcard
// origins are present, maintaining CORS specification compliance.
//
// Development Behavior:
// - Adds localhost variants to allowed origins for local development
// - Enables debug mode for detailed CORS logging
//
// Production Behavior:
// - Uses strict origin allowlist from configuration
// - No debug logging for performance
//
// Security Features:
// - Automatic credential safety: disables credentials if "*" origin present
// - Configurable timeouts and headers
// - Comprehensive method and header allowlists
func CORSMiddleware(config *SecurityConfig) func(next http.Handler) http.Handler {
	// In development, add localhost variants for convenience
	allowedOrigins := config.AllowedOrigins
	if configs.Envs.Server.Environment == "development" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://127.0.0.1:5173")
	}

	// CORS specification compliance: wildcard origins cannot be used with credentials
	// This prevents accidental credential leakage in misconfigured environments
	allowCreds := config.AllowCredentials
	for _, o := range allowedOrigins {
		if o == "*" {
			allowCreds = false
			break
		}
	}

	cors := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   config.AllowedMethods,
		AllowedHeaders:   config.AllowedHeaders,
		ExposedHeaders:   config.ExposedHeaders,
		AllowCredentials: allowCreds,
		MaxAge:           config.MaxAge,
		Debug:            configs.Envs.Server.Environment == "development" || configs.Envs.Server.Environment == "test",
	})

	return cors.Handler
}

// SecurityMiddleware combines CORS and security headers into a single middleware chain.
// The order is important: CORS handler wraps security headers to ensure proper
// preflight request handling while maintaining security header application.
//
// Middleware Order:
// Request -> CORS Handler -> Security Headers -> Application Handler
// Response <- CORS Handler <- Security Headers <- Application Handler
//
// This ensures:
// - CORS preflight requests are handled before security headers
// - Security headers are applied to all responses including CORS responses
// - Proper error handling and response modification throughout the chain
func SecurityMiddleware(config *SecurityConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Chain the middleware: CORS wraps SecurityHeaders which wraps the next handler
		corsHandler := CORSMiddleware(config)
		securityHeadersHandler := SecurityHeaders(next)

		return corsHandler(securityHeadersHandler)
	}
}
