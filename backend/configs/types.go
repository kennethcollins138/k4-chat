package configs

import (
	"time"
)

/*
Configuration Types for k4-chat Application

This file defines all configuration structures used throughout the application.
It's organized by domain and provides type safety for all configuration values.
*/

// Config represents the complete application configuration
type Config struct {
	// Core application configs
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logger   LoggerConfig   `yaml:"logger"`

	// Feature configs
	FeatureFlags FeatureFlagsConfig `yaml:"feature_flags"`
	OAuth        OAuthConfig        `yaml:"oauth"`
	RateLimiting RateLimitingConfig `yaml:"rate_limiting"`
	Middleware   MiddlewareConfig   `yaml:"middleware"`

	// Actor system configs
	Actors ActorsConfig `yaml:"actors"`

	// Environment-specific overrides (from .env files)
	Envs EnvsConfig `yaml:"-"`
}

// ServerConfig defines HTTP server configuration
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes  int           `yaml:"max_header_bytes"`
	EnableProfiling bool          `yaml:"enable_profiling"`
	TrustedProxies  []string      `yaml:"trusted_proxies"`
	HealthCheckPath string        `yaml:"health_check_path"`
	MetricsPath     string        `yaml:"metrics_path"`
}

// DatabaseConfig aggregates all database configurations
type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
	Redis    RedisConfig    `yaml:"redis"`
}

// PostgresConfig defines PostgreSQL connection configuration
type PostgresConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	Database          string        `yaml:"database"`
	Username          string        `yaml:"username"`
	SSLMode           string        `yaml:"ssl_mode"`
	MaxOpenConns      int           `yaml:"max_open_conns"`
	MaxIdleConns      int           `yaml:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime   time.Duration `yaml:"conn_max_idle_time"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period"`
	QueryTimeout      time.Duration `yaml:"query_timeout"`
	MigrationPath     string        `yaml:"migration_path"`
	EnableLogging     bool          `yaml:"enable_logging"`
}

// RedisConfig defines unified Redis configuration (for both caching and database operations)
type RedisConfig struct {
	Enabled        bool                 `yaml:"enabled"`
	Host           string               `yaml:"host"`
	Port           int                  `yaml:"port"`
	Username       string               `yaml:"username"` // Redis 6+ ACL username
	Password       string               `yaml:"password"` // Redis password
	Database       int                  `yaml:"database"`
	KeyPrefix      string               `yaml:"key_prefix"`  // Only used for caching Redis
	DefaultTTL     time.Duration        `yaml:"default_ttl"` // Only used for caching Redis
	PoolSize       int                  `yaml:"pool_size"`
	MinIdleConns   int                  `yaml:"min_idle_conns"`
	MaxRetries     int                  `yaml:"max_retries"`
	DialTimeout    time.Duration        `yaml:"dial_timeout"`
	ReadTimeout    time.Duration        `yaml:"read_timeout"`
	WriteTimeout   time.Duration        `yaml:"write_timeout"`
	PoolTimeout    time.Duration        `yaml:"pool_timeout"`
	IdleTimeout    time.Duration        `yaml:"idle_timeout"`
	MaxConnAge     time.Duration        `yaml:"max_conn_age"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// CircuitBreakerConfig defines CB pattern configuration
type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
}

// AuthConfig defines authentication and authorization configuration
type AuthConfig struct {
	JWT            JWTConfig            `yaml:"jwt"`
	Sessions       SessionConfig        `yaml:"sessions"`
	PasswordPolicy PasswordPolicyConfig `yaml:"password_policy"`
	OAuth          OAuthProvidersConfig `yaml:"oauth_providers"`
	RateLimiting   AuthRateLimitConfig  `yaml:"rate_limiting"`
	Security       AuthSecurityConfig   `yaml:"security"`
}

// JWTConfig defines JWT token configuration
type JWTConfig struct {
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
	Issuer          string        `yaml:"issuer"`
	Audience        string        `yaml:"audience"`
	Algorithm       string        `yaml:"algorithm"`
}

// SessionConfig defines session management configuration
type SessionConfig struct {
	MaxSessions           int           `yaml:"max_sessions_per_user"`
	SessionTimeout        time.Duration `yaml:"session_timeout"`
	RefreshThreshold      time.Duration `yaml:"refresh_threshold"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval"`
	SecureCookies         bool          `yaml:"secure_cookies"`
	SameSitePolicy        string        `yaml:"same_site_policy"`
	DeviceBinding         bool          `yaml:"device_binding"`
	FingerprintValidation bool          `yaml:"fingerprint_validation"`
}

// PasswordPolicyConfig defines password requirements
type PasswordPolicyConfig struct {
	MinLength        int           `yaml:"min_length"`
	RequireUppercase bool          `yaml:"require_uppercase"`
	RequireLowercase bool          `yaml:"require_lowercase"`
	RequireNumbers   bool          `yaml:"require_numbers"`
	RequireSymbols   bool          `yaml:"require_symbols"`
	MaxAttempts      int           `yaml:"max_attempts"`
	LockoutDuration  time.Duration `yaml:"lockout_duration"`
}

// OAuthConfig defines OAuth provider configuration
type OAuthConfig struct {
	OAuthEnabled bool                     `yaml:"oauth_enabled"`
	Providers    map[string]OAuthProvider `yaml:"providers"`
}

// OAuthProvidersConfig defines OAuth providers configuration
type OAuthProvidersConfig struct {
	Google  OAuthProvider `yaml:"google"`
	GitHub  OAuthProvider `yaml:"github"`
	Discord OAuthProvider `yaml:"discord"`
}

// OAuthProvider defines individual OAuth provider settings
type OAuthProvider struct {
	ProviderEnabled bool     `yaml:"provider_enabled"`
	ClientID        string   `yaml:"client_id"`
	RedirectURL     string   `yaml:"redirect_url"`
	Scopes          []string `yaml:"scopes"`
	AuthURL         string   `yaml:"auth_url"`
	TokenURL        string   `yaml:"token_url"`
	UserInfoURL     string   `yaml:"user_info_url"`
}

// AuthRateLimitConfig defines auth-specific rate limiting
type AuthRateLimitConfig struct {
	LoginAttempts   RateLimitRule `yaml:"login_attempts"`
	TokenRefresh    RateLimitRule `yaml:"token_refresh"`
	PasswordReset   RateLimitRule `yaml:"password_reset"`
	AccountCreation RateLimitRule `yaml:"account_creation"`
}

// AuthSecurityConfig defines auth security settings
type AuthSecurityConfig struct {
	RequireTwoFactor        bool          `yaml:"require_two_factor"`
	AllowedOrigins          []string      `yaml:"allowed_origins"`
	BlockSuspiciousLogins   bool          `yaml:"block_suspicious_logins"`
	SessionRotationInterval time.Duration `yaml:"session_rotation_interval"`
}

// LoggerConfig defines logging configuration
type LoggerConfig struct {
	Level              string `yaml:"level"`
	Format             string `yaml:"format"` // json, console
	Output             string `yaml:"output"` // stdout, stderr, file
	FilePath           string `yaml:"file_path"`
	MaxSize            int    `yaml:"max_size_mb"`
	MaxBackups         int    `yaml:"max_backups"`
	MaxAge             int    `yaml:"max_age_days"`
	Compress           bool   `yaml:"compress"`
	EnableSampling     bool   `yaml:"enable_sampling"`
	SamplingInitial    int    `yaml:"sampling_initial"`
	SamplingThereafter int    `yaml:"sampling_thereafter"`
}

// FeatureFlagsConfig defines feature flag configuration
type FeatureFlagsConfig struct {
	EnableWebSocket       bool `yaml:"enable_websocket"`
	EnableFileUploads     bool `yaml:"enable_file_uploads"`
	EnableImageGeneration bool `yaml:"enable_image_generation"`
	EnableWebSearch       bool `yaml:"enable_web_search"`
	EnableUserProfiles    bool `yaml:"enable_user_profiles"`
	EnableChatSharing     bool `yaml:"enable_chat_sharing"`
	EnableAnalytics       bool `yaml:"enable_analytics"`
	EnableMetrics         bool `yaml:"enable_metrics"`
	EnableDebugMode       bool `yaml:"enable_debug_mode"`
}

// RateLimitingConfig defines global rate limiting configuration
type RateLimitingConfig struct {
	RLEnabled      bool                     `yaml:"rl_enabled"`
	Storage        string                   `yaml:"storage"` // memory, redis
	DefaultLimits  map[string]RateLimitRule `yaml:"default_limits"`
	PerUserLimits  map[string]RateLimitRule `yaml:"per_user_limits"`
	WhitelistedIPs []string                 `yaml:"whitelisted_ips"`
	KeyGenerator   string                   `yaml:"key_generator"` // ip, user, custom
}

// RateLimitRule defines a rate limiting rule
type RateLimitRule struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
	Burst    int           `yaml:"burst"`
}

// MiddlewareConfig defines middleware configuration
type MiddlewareConfig struct {
	Security    SecurityMiddlewareConfig `yaml:"security"`
	Compression CompressionConfig        `yaml:"compression"`
	CORS        CORSConfig               `yaml:"cors"`
	Timeout     TimeoutConfig            `yaml:"timeout"`
}

// SecurityMiddlewareConfig defines security middleware settings
type SecurityMiddlewareConfig struct {
	EnableHSTS               bool     `yaml:"enable_hsts"`
	HSTSMaxAge               int      `yaml:"hsts_max_age"`
	EnableFrameOptions       bool     `yaml:"enable_frame_options"`
	FrameOptionsValue        string   `yaml:"frame_options_value"`
	EnableContentTypeNoSniff bool     `yaml:"enable_content_type_no_sniff"`
	CSPPolicy                string   `yaml:"csp_policy"`
	AllowedHosts             []string `yaml:"allowed_hosts"`
}

// CompressionConfig defines compression settings
type CompressionConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Level     int      `yaml:"level"`
	MinLength int      `yaml:"min_length"`
	Types     []string `yaml:"types"`
}

// CORSConfig defines CORS settings
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// TimeoutConfig defines timeout settings
type TimeoutConfig struct {
	Read    time.Duration `yaml:"read"`
	Write   time.Duration `yaml:"write"`
	Request time.Duration `yaml:"request"`
}

// ActorsConfig defines actor system configuration
type ActorsConfig struct {
	Supervisor   SupervisorConfig   `yaml:"supervisor"`
	UserManager  UserManagerConfig  `yaml:"user_manager"`
	Connection   ConnectionConfig   `yaml:"connection"`
	ChatSession  ChatSessionConfig  `yaml:"chat_session"`
	LLMManager   LLMManagerConfig   `yaml:"llm_manager"`
	ToolsManager ToolsManagerConfig `yaml:"tools_manager"`
}

// SupervisorConfig defines supervisor actor configuration
type SupervisorConfig struct {
	MaxRestarts         int           `yaml:"max_restarts"`
	RestartWindow       time.Duration `yaml:"restart_window"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

// UserManagerConfig defines user manager actor configuration
type UserManagerConfig struct {
	MaxActiveUsers        int           `yaml:"max_active_users"`
	IdleTimeout           time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout       time.Duration `yaml:"shutdown_timeout"`
	HealthCheckInterval   time.Duration `yaml:"health_check_interval"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval"`
	MaxConnectionsPerUser int           `yaml:"max_connections_per_user"`
}

// ConnectionConfig defines connection actor configuration
type ConnectionConfig struct {
	PingInterval     time.Duration `yaml:"ping_interval"`
	PingTimeout      time.Duration `yaml:"ping_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout"`
	ReadTimeout      time.Duration `yaml:"read_timeout"`
	MaxMessageSize   int           `yaml:"max_message_size"`
	BufferSize       int           `yaml:"buffer_size"`
	CompressionLevel int           `yaml:"compression_level"`
}

// ChatSessionConfig defines chat session actor configuration
type ChatSessionConfig struct {
	MaxSessions      int           `yaml:"max_sessions_per_user"`
	IdleTimeout      time.Duration `yaml:"idle_timeout"`
	MessageBatchSize int           `yaml:"message_batch_size"`
	HistoryLimit     int           `yaml:"history_limit"`
	AutoSaveInterval time.Duration `yaml:"auto_save_interval"`
}

// LLMManagerConfig defines LLM manager configuration
type LLMManagerConfig struct {
	DefaultModel  string                       `yaml:"default_model"`
	Timeout       time.Duration                `yaml:"timeout"`
	MaxConcurrent int                          `yaml:"max_concurrent_requests"`
	RetryAttempts int                          `yaml:"retry_attempts"`
	RetryDelay    time.Duration                `yaml:"retry_delay"`
	Providers     map[string]LLMProviderConfig `yaml:"providers"`
}

// LLMProviderConfig defines individual LLM provider configuration
type LLMProviderConfig struct {
	Enabled     bool          `yaml:"enabled"`
	BaseURL     string        `yaml:"base_url"`
	Model       string        `yaml:"model"`
	MaxTokens   int           `yaml:"max_tokens"`
	Temperature float64       `yaml:"temperature"`
	TopP        float64       `yaml:"top_p"`
	Timeout     time.Duration `yaml:"timeout"`
}

// ToolsManagerConfig defines tools manager configuration
type ToolsManagerConfig struct {
	Enabled       bool                  `yaml:"enabled"`
	Timeout       time.Duration         `yaml:"timeout"`
	MaxConcurrent int                   `yaml:"max_concurrent"`
	Tools         map[string]ToolConfig `yaml:"tools"`
}

// ToolConfig defines individual tool configuration
type ToolConfig struct {
	Enabled     bool                   `yaml:"enabled"`
	Timeout     time.Duration          `yaml:"timeout"`
	MaxRequests int                    `yaml:"max_requests_per_hour"`
	Config      map[string]interface{} `yaml:"config"`
}
