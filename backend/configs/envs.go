package configs

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

/*
EnvsConfig defines the applications environment variables

This struct provides a central definition of all environment variables
organized by their respective responsibilities and by development environments.
*/
type EnvsConfig struct {
	Server ServerEnvsConfig

	Postgres PostgresEnvsConfig

	Redis RedisEnvsConfig

	Auth AuthEnvsConfig
}

type ServerEnvsConfig struct {
	Environment string
	Port        string
	FrontendURL string
}

type PostgresEnvsConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
}

type RedisEnvsConfig struct {
	Host     string
	Port     string
	Password string
}

type AuthEnvsConfig struct {
	AuthRedisURI       string
	AccessTokenSecret  string
	RefreshTokenSecret string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
}

var Envs EnvsConfig

// loadOnce ensures we only load envs a single time per process
var loadOnce sync.Once

// LoadEnvs reads .env files and environment variables exactly once.
// It is concurrency-safe; subsequent calls are no-ops.
// If validation fails the first caller receives an error.
func LoadEnvs() error {
	var err error
	loadOnce.Do(func() {
		err = loadEnvsInternal()
	})
	return err
}

// loadEnvsInternal contains the previous implementation body.
func loadEnvsInternal() error {
	// Allow an explicit env file override (useful in CI / container images)
	if customPath := os.Getenv("ENV_FILE"); customPath != "" {
		if err := godotenv.Load(customPath); err != nil {
			return fmt.Errorf("failed to load ENV_FILE=%s: %w", customPath, err)
		}
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	var envFiles string

	switch env {
	case "development":
		envFiles = ".env.development"
	case "production":
		envFiles = ".env.production"
	case "test":
		envFiles = ".env.test"
	default:
		envFiles = ".env"
	}

	envPath := filepath.Join(getProjectRoot(), envFiles)

	if err := godotenv.Load(envPath); err != nil {
		return fmt.Errorf("failed to load environment variables from %s: %w", envPath, err)
	}

	err := LoadServerEnvs()
	if err != nil {
		return fmt.Errorf("failed to load server environment variables: %w", err)
	}

	err = LoadPostgresEnvs()
	if err != nil {
		return fmt.Errorf("failed to load postgres environment variables: %w", err)
	}

	err = LoadRedisEnvs()
	if err != nil {
		return fmt.Errorf("failed to load redis environment variables: %w", err)
	}

	err = LoadAuthEnvs()
	if err != nil {
		return fmt.Errorf("failed to load auth environment variables: %w", err)
	}

	// Validate configuration and fail-fast
	if err := Envs.Validate(); err != nil {
		return err
	}

	return nil
}

func LoadServerEnvs() error {
	Envs.Server.Environment = getEnv("ENV", "development")
	Envs.Server.Port = getEnv("PORT", "8080")
	Envs.Server.FrontendURL = getEnv("FRONTEND_URL", "http://localhost:5173")

	return nil
}

func LoadPostgresEnvs() error {
	Envs.Postgres.Host = os.Getenv("POSTGRES_HOST")
	Envs.Postgres.Port = os.Getenv("POSTGRES_PORT")
	Envs.Postgres.User = os.Getenv("POSTGRES_USER")
	Envs.Postgres.Password = os.Getenv("POSTGRES_PASSWORD")
	Envs.Postgres.DB = os.Getenv("POSTGRES_DB")

	return nil
}

func LoadRedisEnvs() error {
	Envs.Redis.Host = os.Getenv("REDIS_HOST")
	Envs.Redis.Port = os.Getenv("REDIS_PORT")
	Envs.Redis.Password = os.Getenv("REDIS_PASSWORD")

	return nil
}

func LoadAuthEnvs() error {
	Envs.Auth.AuthRedisURI = os.Getenv("AUTH_REDIS_URI")

	// Secrets – in development we allow auto-generated fallbacks, in production we require explicit
	if Envs.Server.Environment == "production" {
		Envs.Auth.AccessTokenSecret = os.Getenv("ACCESS_TOKEN_SECRET")
		Envs.Auth.RefreshTokenSecret = os.Getenv("REFRESH_TOKEN_SECRET")
	} else {
		Envs.Auth.AccessTokenSecret = getEnv("ACCESS_TOKEN_SECRET", GeneratesecureToken(32))
		Envs.Auth.RefreshTokenSecret = getEnv("REFRESH_TOKEN_SECRET", GeneratesecureToken(32))
	}

	// Parse TTLs – default 15m / 168h (7d)
	accessTTLStr := getEnv("ACCESS_TOKEN_TTL", "15m")
	refreshTTLStr := getEnv("REFRESH_TOKEN_TTL", "168h")
	var err error
	Envs.Auth.AccessTokenTTL, err = time.ParseDuration(accessTTLStr)
	if err != nil {
		return fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}
	Envs.Auth.RefreshTokenTTL, err = time.ParseDuration(refreshTTLStr)
	if err != nil {
		return fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}

	return nil
}

/*
GeneratesecureToken generates a secure token of the specified length.

This function uses the crypto/rand package to generate a random byte slice
and then encodes it using base64.URLEncoding.

Parameters:
  - length: The length of the token to generate

Returns:
  - string: The generated secure token
*/
func GeneratesecureToken(length int) string {
	secureToken := make([]byte, length)
	_, err := rand.Read(secureToken)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(secureToken)
}

/*
getProjectRoot determines the root directory of the project.

This function searches for common project files (go.mod, go.sum) to identify
the root directory. It recursively traverses up the directory tree until it
*/
func getProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Look for common project files to identify the root
	rootMarkers := []string{"go.mod", "go.sum"}

	for {
		// Check if any root markers exist in this directory
		for _, marker := range rootMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the filesystem root without finding project root
			break
		}
		dir = parent
	}

	// If we couldn't determine the root, return current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return currentDir
}

/*
getEnv retrieves an environment variable or returns a default value.

This helper function simplifies accessing environment variables with fallbacks
when values are not set.

Parameters:
  - key: Name of the environment variable to retrieve
  - defaultValue: Value to return if the environment variable is not set

Returns:
  - string: The environment variable value or the default
*/
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Validate performs sanity checks and returns error if configuration is not usable.
func (e *EnvsConfig) Validate() error {
	var missing []string

	// Required in every env
	if e.Postgres.Host == "" {
		missing = append(missing, "POSTGRES_HOST")
	}
	if e.Postgres.User == "" {
		missing = append(missing, "POSTGRES_USER")
	}
	if e.Postgres.Password == "" && e.Server.Environment == "production" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if e.Postgres.DB == "" {
		missing = append(missing, "POSTGRES_DB")
	}

	if e.Redis.Host == "" {
		missing = append(missing, "REDIS_HOST")
	}

	// Secrets must be present in production
	if e.Server.Environment == "production" {
		if e.Auth.AccessTokenSecret == "" {
			missing = append(missing, "ACCESS_TOKEN_SECRET")
		}
		if e.Auth.RefreshTokenSecret == "" {
			missing = append(missing, "REFRESH_TOKEN_SECRET")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	// Warn / error on obviously invalid fields
	if _, err := strconv.Atoi(e.Server.Port); err != nil {
		return fmt.Errorf("PORT must be numeric, got %q", e.Server.Port)
	}

	return nil
}

// PortInt returns the server port as an int (defaults to 8080)
func (s ServerEnvsConfig) PortInt() int {
	p, _ := strconv.Atoi(s.Port)
	if p == 0 {
		return 8080
	}
	return p
}
