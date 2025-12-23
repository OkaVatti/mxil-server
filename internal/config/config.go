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

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Email    EmailConfig    `yaml:"email"`
	Network  NetworkConfig  `yaml:"network"`
	Storage  StorageConfig  `yaml:"storage"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig contains server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	TLSEnabled   bool          `yaml:"tls_enabled"`
	TLSCertPath  string        `yaml:"tls_cert_path"`
	TLSKeyPath   string        `yaml:"tls_key_path"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// SecurityConfig contains security configuration
type SecurityConfig struct {
	JWTSecret             string        `yaml:"jwt_secret"`
	JWTExpiration         time.Duration `yaml:"jwt_expiration"`
	EncryptionKey         []byte        `yaml:"encryption_key"`
	PasswordMinLength     int           `yaml:"password_min_length"`
	PasswordRequireSymbol bool          `yaml:"password_require_symbol"`
	PasswordRequireNumber bool          `yaml:"password_require_number"`
	PasswordRequireUpper  bool          `yaml:"password_require_upper"`
	MaxLoginAttempts      int           `yaml:"max_login_attempts"`
	LockoutDuration       time.Duration `yaml:"lockout_duration"`
}

// EmailConfig contains email configuration
type EmailConfig struct {
	Domain         string `yaml:"domain"`
	SMTPPort       int    `yaml:"smtp_port"`
	SMTPSPort      int    `yaml:"smtps_port"`
	IMAPPort       int    `yaml:"imap_port"`
	POP3Port       int    `yaml:"pop3_port"`
	MaxMessageSize int64  `yaml:"max_message_size"`
	EnableDKIM     bool   `yaml:"enable_dkim"`
	EnableSPF      bool   `yaml:"enable_spf"`
	EnableDMARC    bool   `yaml:"enable_dmarc"`
	DKIMSelector   string `yaml:"dkim_selector"`
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	EnableI2P     bool   `yaml:"enable_i2p"`
	I2PRouterHost string `yaml:"i2p_router_host"`
	I2PRouterPort int    `yaml:"i2p_router_port"`
	EnableTor     bool   `yaml:"enable_tor"`
	TorProxyHost  string `yaml:"tor_proxy_host"`
	TorProxyPort  int    `yaml:"tor_proxy_port"`
}

// StorageConfig contains storage configuration
type StorageConfig struct {
	Backend     string `yaml:"backend"`
	LocalPath   string `yaml:"local_path"`
	MaxFileSize int64  `yaml:"max_file_size"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Read config file
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Override with environment variables if present
	overrideFromEnv(&cfg)

	return &cfg, nil
}

// overrideFromEnv overrides configuration with environment variables
func overrideFromEnv(cfg *Config) {
	if host := os.Getenv("SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.Port)
	}

	// Database overrides
	if host := os.Getenv("DATABASE_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("DATABASE_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Database.Port)
	}
	if user := os.Getenv("DATABASE_USER"); user != "" {
		cfg.Database.User = user
	}
	if password := os.Getenv("DATABASE_PASSWORD"); password != "" {
		cfg.Database.Password = password
	}
	if database := os.Getenv("DATABASE_NAME"); database != "" {
		cfg.Database.Database = database
	}

	// Security overrides
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.Security.JWTSecret = secret
	}
	if key := os.Getenv("ENCRYPTION_KEY"); key != "" {
		cfg.Security.EncryptionKey = []byte(key)
	}
}
