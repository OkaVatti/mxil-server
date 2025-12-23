package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Security SecurityConfig
	Email    EmailConfig
	Network  NetworkConfig
	Storage  StorageConfig
	Logging  LoggingConfig
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	TLSEnabled   bool
	TLSCertPath  string
	TLSKeyPath   string
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	JWTSecret             string
	JWTExpiration         time.Duration
	EncryptionKey         []byte
	PasswordMinLength     int
	PasswordRequireSymbol bool
	PasswordRequireNumber bool
	PasswordRequireUpper  bool
	MaxLoginAttempts      int
	LockoutDuration       time.Duration
}

// EmailConfig represents email configuration
type EmailConfig struct {
	Domain         string
	SMTPPort       int
	SMTPSPort      int
	IMAPPort       int
	POP3Port       int
	MaxMessageSize int64
	EnableDKIM     bool
	EnableSPF      bool
	EnableDMARC    bool
	DKIMSelector   string
	DKIMPrivateKey string
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	EnableI2P     bool
	I2PRouterHost string
	I2PRouterPort int
	EnableTor     bool
	TorProxyHost  string
	TorProxyPort  int
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	Backend     string
	LocalPath   string
	MaxFileSize int64
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string
	Format     string
	Output     string
	FilePath   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// Load loads configuration from environment and YAML file
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load YAML config
	configPath := getEnv("CONFIG_PATH", "./config.yml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var yamlConfig Config
	if err := yaml.Unmarshal(configData, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Override with environment variables
	overrideWithEnv(&yamlConfig)

	// Validate configuration
	if err := validateConfig(&yamlConfig); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &yamlConfig, nil
}

// overrideWithEnv overrides configuration with environment variables
func overrideWithEnv(cfg *Config) {
	// Server
	if v := getEnv("SERVER_HOST", ""); v != "" {
		cfg.Server.Host = v
	}
	if v := getEnv("SERVER_PORT", ""); v != "" {
		cfg.Server.Port = parseInt(v, cfg.Server.Port)
	}
	if v := getEnv("SERVER_TLS_ENABLED", ""); v != "" {
		cfg.Server.TLSEnabled = parseBool(v)
	}

	// Database
	if v := getEnv("DATABASE_HOST", ""); v != "" {
		cfg.Database.Host = v
	}
	if v := getEnv("DATABASE_PORT", ""); v != "" {
		cfg.Database.Port = parseInt(v, cfg.Database.Port)
	}
	if v := getEnv("DATABASE_USER", ""); v != "" {
		cfg.Database.User = v
	}
	if v := getEnv("DATABASE_PASSWORD", ""); v != "" {
		cfg.Database.Password = v
	}
	if v := getEnv("DATABASE_NAME", ""); v != "" {
		cfg.Database.Database = v
	}

	// Security
	if v := getEnv("JWT_SECRET", ""); v != "" {
		cfg.Security.JWTSecret = v
	}
	if v := getEnv("JWT_EXPIRATION", ""); v != "" {
		cfg.Security.JWTExpiration = parseDuration(v, cfg.Security.JWTExpiration)
	}
	if v := getEnv("ENCRYPTION_KEY", ""); v != "" {
		cfg.Security.EncryptionKey = []byte(v)
	}

	// Email
	if v := getEnv("EMAIL_DOMAIN", ""); v != "" {
		cfg.Email.Domain = v
	}

	// Network
	if v := getEnv("ENABLE_I2P", ""); v != "" {
		cfg.Network.EnableI2P = parseBool(v)
	}
	if v := getEnv("ENABLE_TOR", ""); v != "" {
		cfg.Network.EnableTor = parseBool(v)
	}

	// Storage
	if v := getEnv("STORAGE_BACKEND", ""); v != "" {
		cfg.Storage.Backend = v
	}
	if v := getEnv("STORAGE_LOCAL_PATH", ""); v != "" {
		cfg.Storage.LocalPath = v
	}

	// Logging
	if v := getEnv("LOG_LEVEL", ""); v != "" {
		cfg.Logging.Level = v
	}
	if v := getEnv("LOG_FORMAT", ""); v != "" {
		cfg.Logging.Format = v
	}
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	// Server validation
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	// Database validation
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", cfg.Database.Port)
	}

	// Security validation
	if len(cfg.Security.JWTSecret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters")
	}
	if cfg.Security.JWTExpiration <= 0 {
		return fmt.Errorf("JWT expiration must be positive")
	}
	if len(cfg.Security.EncryptionKey) < 32 {
		return fmt.Errorf("encryption key must be at least 32 bytes")
	}

	// Email validation
	if cfg.Email.Domain == "" {
		return fmt.Errorf("email domain is required")
	}

	// Storage validation
	if cfg.Storage.LocalPath == "" {
		return fmt.Errorf("storage local path is required")
	}

	return nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return value
}

func parseBool(s string) bool {
	if s == "" {
		return false
	}
	value, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return value
}

func parseDuration(s string, defaultValue time.Duration) time.Duration {
	if s == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return defaultValue
	}
	return duration
}

// LoadFromBytes loads configuration from YAML bytes
func LoadFromBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}
	return &cfg, nil
}

// String returns a string representation of the config (with sensitive data redacted)
func (c *Config) String() string {
	cfg := *c
	cfg.Database.Password = "[REDACTED]"
	cfg.Security.JWTSecret = "[REDACTED]"
	cfg.Security.EncryptionKey = []byte("[REDACTED]")

	data, _ := yaml.Marshal(cfg)
	return string(data)
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(getEnv("ENVIRONMENT", "production")) == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return strings.ToLower(getEnv("ENVIRONMENT", "production")) == "production"
}
