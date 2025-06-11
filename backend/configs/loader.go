package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

/*
Configuration Loading System

Design Principles:
- Secrets come from environment variables (.env files)
- Static configuration comes from YAML files
- Environment-specific overrides (testing/development/staging/production)
*/

var (
	// GlobalConfig is the application-wide configuration instance
	GlobalConfig *Config

	// loadOnce ensures configuration is loaded exactly once
	configLoadOnce sync.Once

	// configMutex protects concurrent access to configuration
	configMutex sync.RWMutex
)

// LoadConfig loads the complete application configuration
// It combines YAML files with environment variable overrides
func LoadConfig() error {
	var err error
	configLoadOnce.Do(func() {
		err = loadConfigInternal()
	})
	return err
}

// loadConfigInternal performs the configuration loading of yaml files and env vars
func loadConfigInternal() error {
	// First, load environment variables (secrets and deployment-specific values)
	if err := LoadEnvs(); err != nil {
		return fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Determine which YAML config file to load based on environment
	environment := Envs.Server.Environment
	if environment == "" {
		environment = "development"
	}

	configPath := fmt.Sprintf("configs/yaml/%s.yaml", environment)

	// Try to load the environment-specific config file
	config, err := loadYAMLConfig(configPath)
	if err != nil {
		// Fall back
		defaultPath := "configs/yaml/default.yaml"
		config, err = loadYAMLConfig(defaultPath)
		if err != nil {
			return fmt.Errorf("failed to load default config: %w", err)
		}
	}

	// Apply environment variable overrides
	if err := applyEnvironmentOverrides(config); err != nil {
		return fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate the final configuration
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Store the global configuration
	configMutex.Lock()
	GlobalConfig = config
	configMutex.Unlock()

	return nil
}

// loadYAMLConfig loads configuration from a YAML file
func loadYAMLConfig(configPath string) (*Config, error) {
	// Get project root and construct full path
	projectRoot := getProjectRoot()
	fullPath := filepath.Join(projectRoot, configPath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", fullPath)
	}

	// Read the YAML file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", fullPath, err)
	}

	// Parse YAML into config struct
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config %s: %w", fullPath, err)
	}

	return &config, nil
}

// applyEnvironmentOverrides applies environment variable values to the config
func applyEnvironmentOverrides(config *Config) error {
	config.Envs = Envs

	// Server overrides
	if Envs.Server.Port != "" {
		config.Server.Port = Envs.Server.PortInt()
	}

	if Envs.Server.FrontendURL != "" {
		if config.Middleware.CORS.AllowedOrigins == nil {
			config.Middleware.CORS.AllowedOrigins = []string{Envs.Server.FrontendURL}
		}
	}

	// Database overrides
	if Envs.Postgres.Host != "" {
		config.Database.Postgres.Host = Envs.Postgres.Host
	}
	if Envs.Postgres.User != "" {
		config.Database.Postgres.Username = Envs.Postgres.User
	}
	if Envs.Postgres.DB != "" {
		config.Database.Postgres.Database = Envs.Postgres.DB
	}

	// Redis overrides
	if Envs.Redis.Host != "" {
		config.Redis.Host = Envs.Redis.Host
		config.Database.Redis.Host = Envs.Redis.Host
	}

	// Auth overrides
	config.Auth.JWT.AccessTokenTTL = Envs.Auth.AccessTokenTTL
	config.Auth.JWT.RefreshTokenTTL = Envs.Auth.RefreshTokenTTL

	return nil
}

// validateConfig performs comprehensive configuration validation
func validateConfig(config *Config) error {
	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Validate database configuration
	if config.Database.Postgres.Host == "" {
		return fmt.Errorf("postgres host is required")
	}
	if config.Database.Postgres.Username == "" {
		return fmt.Errorf("postgres username is required")
	}
	if config.Database.Postgres.Database == "" {
		return fmt.Errorf("postgres database name is required")
	}

	// Validate Redis configuration if enabled
	if config.Redis.Enabled && config.Redis.Host == "" {
		return fmt.Errorf("redis host is required when redis is enabled")
	}

	// Validate auth configuration
	if config.Auth.JWT.AccessTokenTTL <= 0 {
		return fmt.Errorf("access token TTL must be positive")
	}
	if config.Auth.JWT.RefreshTokenTTL <= 0 {
		return fmt.Errorf("refresh token TTL must be positive")
	}

	// Validate actor configuration
	if config.Actors.UserManager.MaxActiveUsers <= 0 {
		return fmt.Errorf("max active users must be positive")
	}

	// TODO: Validate feature flags when implemented

	return nil
}

// GetConfig returns the global configuration instance
// This is the primary way to access configuration throughout the application
func GetConfig() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return GlobalConfig
}

// GetServerConfig returns just the server configuration
func GetServerConfig() ServerConfig {
	config := GetConfig()
	if config == nil {
		return ServerConfig{}
	}
	return config.Server
}

// GetDatabaseConfig returns just the database configuration
func GetDatabaseConfig() DatabaseConfig {
	config := GetConfig()
	if config == nil {
		return DatabaseConfig{}
	}
	return config.Database
}

// GetAuthConfig returns just the auth configuration
func GetAuthConfig() AuthConfig {
	config := GetConfig()
	if config == nil {
		return AuthConfig{}
	}
	return config.Auth
}

// GetActorsConfig returns just the actors configuration
func GetActorsConfig() ActorsConfig {
	config := GetConfig()
	if config == nil {
		return ActorsConfig{}
	}
	return config.Actors
}

// IsFeatureEnabled checks if a specific feature flag is enabled
func IsFeatureEnabled(feature string) bool {
	config := GetConfig()
	if config == nil {
		return false
	}

	flags := config.FeatureFlags
	switch feature {
	case "websocket":
		return flags.EnableWebSocket
	case "file_uploads":
		return flags.EnableFileUploads
	case "image_generation":
		return flags.EnableImageGeneration
	case "web_search":
		return flags.EnableWebSearch
	case "user_profiles":
		return flags.EnableUserProfiles
	case "chat_sharing":
		return flags.EnableChatSharing
	case "analytics":
		return flags.EnableAnalytics
	case "metrics":
		return flags.EnableMetrics
	case "debug":
		return flags.EnableDebugMode
	default:
		return false
	}
}
