package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/cors"
	"github.com/kdot/k4-chat/backend/configs"
	"go.uber.org/zap"
)

/*
SecurityMiddleware provides comprehensive HTTP security controls for the k4-chat API.

This middleware applies security headers, CORS policies, and environment-specific
configurations based on the application's YAML configuration.

Key Features:
- YAML-driven configuration from configs.MiddlewareConfig
- Environment-aware security policies (strict prod, permissive dev)
- CORS with credential safety and origin validation
- Comprehensive security header application
- API response caching controls
- Session invalidation on logout

Security Headers Applied:
- HSTS, CSP, X-Frame-Options, X-Content-Type-Options
- X-XSS-Protection, Referrer-Policy, Permissions-Policy
- Cross-Origin policies (COEP, COOP, CORP)
- Clear-Site-Data for logout endpoints
*/

// SecurityMiddleware creates a security middleware that applies both security headers
// and CORS policies based on the provided middleware configuration.
func SecurityMiddleware(cfg *configs.MiddlewareConfig, logger *zap.Logger) func(next http.Handler) http.Handler {
	// Create CORS middleware from config
	corsMiddleware := createCORSMiddleware(cfg, logger)

	return func(next http.Handler) http.Handler {
		// Chain CORS -> Security Headers -> Next Handler
		return corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			applySecurityHeaders(w, r, cfg)
			next.ServeHTTP(w, r)
		}))
	}
}

// createCORSMiddleware builds CORS middleware from the YAML configuration
func createCORSMiddleware(cfg *configs.MiddlewareConfig, logger *zap.Logger) func(http.Handler) http.Handler {
	if !cfg.CORS.Enabled {
		// Return no-op if CORS is disabled
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Start with configured origins
	allowedOrigins := cfg.CORS.AllowedOrigins

	// Add development origins if needed
	if configs.Envs.Server.Environment == "development" {
		devOrigins := []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
		}
		allowedOrigins = append(allowedOrigins, devOrigins...)

		// Add frontend URL if configured
		if configs.Envs.Server.FrontendURL != "" {
			allowedOrigins = append(allowedOrigins, configs.Envs.Server.FrontendURL)
		}
	}

	// Safety check: disable credentials if wildcard origin present
	allowCredentials := cfg.CORS.AllowCredentials
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowCredentials = false
			if logger != nil {
				logger.Warn("Disabled CORS credentials due to wildcard origin for security")
			}
			break
		}
	}

	corsConfig := cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: allowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
		Debug:            configs.Envs.Server.Environment == "development",
	}

	return cors.Handler(corsConfig)
}

// applySecurityHeaders applies security headers based on configuration and environment
func applySecurityHeaders(w http.ResponseWriter, r *http.Request, cfg *configs.MiddlewareConfig) {
	security := &cfg.Security
	isProduction := configs.Envs.Server.Environment == "production"

	// HSTS - Only in production and if enabled
	if isProduction && security.EnableHSTS {
		hstsValue := "max-age=31536000; includeSubDomains; preload"
		if security.HSTSMaxAge > 0 {
			hstsValue = strings.Replace(hstsValue, "31536000",
				strconv.Itoa(security.HSTSMaxAge), 1)
		}
		w.Header().Set("Strict-Transport-Security", hstsValue)
	}

	// Content Security Policy
	cspPolicy := security.CSPPolicy
	if cspPolicy == "" {
		// Generate default CSP based on environment
		if isProduction {
			cspPolicy = "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self' data:; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
		} else {
			cspPolicy = "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self' data:; " +
				"connect-src 'self' ws:; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
		}
	}
	w.Header().Set("Content-Security-Policy", cspPolicy)

	// X-Content-Type-Options
	if security.EnableContentTypeNoSniff {
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}

	// X-Frame-Options
	if security.EnableFrameOptions {
		frameValue := security.FrameOptionsValue
		if frameValue == "" {
			frameValue = "DENY"
		}
		w.Header().Set("X-Frame-Options", frameValue)
	}

	// Standard security headers
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy",
		"geolocation=(), microphone=(), camera=(), payment=(), usb=(), "+
			"magnetometer=(), gyroscope=(), speaker=(), ambient-light-sensor=(), "+
			"accelerometer=(), battery=()")

	// Cross-origin isolation policies
	w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")

	// Hide server information
	w.Header().Set("Server", "")

	// Certificate Transparency in production
	if isProduction {
		w.Header().Set("Expect-CT", "max-age=86400, enforce")
	}

	// Clear site data on logout
	if strings.HasSuffix(r.URL.Path, "/auth/signout") ||
		strings.HasSuffix(r.URL.Path, "/auth/signout-all") {
		w.Header().Set("Clear-Site-Data", "\"cache\", \"cookies\", \"storage\"")
	}

	// Prevent caching of API responses
	if r.Header.Get("Accept") == "application/json" ||
		r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
}
