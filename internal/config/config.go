// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Email    EmailConfig    `yaml:"email"`
	Network  NetworkConfig  `yaml:"network"`
	Storage  StorageConfig  `yaml:"storage"`
	Logging  LoggingConfig  `yaml:"logging"`
	Redis    RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	TLS          TLSConfig     `yaml:"tls"`
	CORS         CORSConfig    `yaml:"cors"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

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
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

type SecurityConfig struct {
	JWTSecret             string          `yaml:"jwt_secret"`
	JWTExpiration         time.Duration   `yaml:"jwt_expiration"`
	EncryptionKey         string          `yaml:"encryption_key"`
	PasswordMinLength     int             `yaml:"password_min_length"`
	PasswordRequireSymbol bool            `yaml:"password_require_symbol"`
	PasswordRequireNumber bool            `yaml:"password_require_number"`
	PasswordRequireUpper  bool            `yaml:"password_require_upper"`
	MaxLoginAttempts      int             `yaml:"max_login_attempts"`
	LockoutDuration       time.Duration   `yaml:"lockout_duration"`
	RateLimit             RateLimitConfig `yaml:"rate_limit"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	Burst             int `yaml:"burst"`
}

type EmailConfig struct {
	Domain            string        `yaml:"domain"`
	SMTPPort          int           `yaml:"smtp_port"`
	SMTPSPort         int           `yaml:"smtps_port"`
	IMAPPort          int           `yaml:"imap_port"`
	POP3Port          int           `yaml:"pop3_port"`
	MaxMessageSize    int64         `yaml:"max_message_size"`
	EnableDKIM        bool          `yaml:"enable_dkim"`
	EnableSPF         bool          `yaml:"enable_spf"`
	EnableDMARC       bool          `yaml:"enable_dmarc"`
	DKIMSelector      string        `yaml:"dkim_selector"`
	DKIMPrivateKey    string        `yaml:"dkim_private_key"`
	DefaultFrom       string        `yaml:"default_from"`
	VerificationEmail EmailTemplate `yaml:"verification_email"`
	ResetEmail        EmailTemplate `yaml:"reset_email"`
}

type EmailTemplate struct {
	Subject string `yaml:"subject"`
	Body    string `yaml:"body"`
}

type NetworkConfig struct {
	EnableI2P      bool   `yaml:"enable_i2p"`
	I2PRouterHost  string `yaml:"i2p_router_host"`
	I2PRouterPort  int    `yaml:"i2p_router_port"`
	EnableTor      bool   `yaml:"enable_tor"`
	TorProxyHost   string `yaml:"tor_proxy_host"`
	TorProxyPort   int    `yaml:"tor_proxy_port"`
	EnableIPFS     bool   `yaml:"enable_ipfs"`
	IPFSGateway    string `yaml:"ipfs_gateway"`
	DefaultNetwork string `yaml:"default_network"`
}

type StorageConfig struct {
	Backend     string     `yaml:"backend"`
	LocalPath   string     `yaml:"local_path"`
	S3          S3Config   `yaml:"s3"`
	IPFS        IPFSConfig `yaml:"ipfs"`
	MaxFileSize int64      `yaml:"max_file_size"`
	Encryption  bool       `yaml:"encryption"`
}

type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

type IPFSConfig struct {
	Gateway    string `yaml:"gateway"`
	APIAddress string `yaml:"api_address"`
}

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

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables only")
	}

	// Default config
	config := &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			TLS: TLSConfig{
				Enabled: false,
			},
			CORS: CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"*"},
				AllowCredentials: true,
				MaxAge:           86400,
			},
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "mxil",
			Database:        "mxil",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
		Security: SecurityConfig{
			PasswordMinLength:     12,
			PasswordRequireSymbol: true,
			PasswordRequireNumber: true,
			PasswordRequireUpper:  true,
			MaxLoginAttempts:      5,
			LockoutDuration:       15 * time.Minute,
			JWTExpiration:         24 * time.Hour,
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 60,
				Burst:             100,
			},
		},
		Email: EmailConfig{
			Domain:         "mxil.example.com",
			SMTPPort:       587,
			SMTPSPort:      465,
			IMAPPort:       993,
			POP3Port:       995,
			MaxMessageSize: 52428800,
			EnableDKIM:     true,
			EnableSPF:      true,
			EnableDMARC:    true,
			DKIMSelector:   "default",
			DefaultFrom:    "noreply@mxil.example.com",
			VerificationEmail: EmailTemplate{
				Subject: "Verify your MXIL account",
				Body:    "Click the link to verify your account: {{.URL}}",
			},
			ResetEmail: EmailTemplate{
				Subject: "Reset your MXIL password",
				Body:    "Click the link to reset your password: {{.URL}}",
			},
		},
		Network: NetworkConfig{
			EnableI2P:      false,
			I2PRouterHost:  "localhost",
			I2PRouterPort:  7657,
			EnableTor:      false,
			TorProxyHost:   "localhost",
			TorProxyPort:   9050,
			EnableIPFS:     false,
			IPFSGateway:    "https://ipfs.io",
			DefaultNetwork: "clearnet",
		},
		Storage: StorageConfig{
			Backend:     "local",
			LocalPath:   "./data/storage",
			MaxFileSize: 104857600,
			Encryption:  true,
			S3: S3Config{
				Region: "us-east-1",
				UseSSL: true,
			},
			IPFS: IPFSConfig{
				Gateway:    "https://ipfs.io",
				APIAddress: "/ip4/127.0.0.1/tcp/5001",
			},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			FilePath:   "./logs/mxil.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: 6379,
			DB:   0,
		},
	}

	// Load from config file if exists
	configPaths := []string{
		"config.yml",
		"config/config.yml",
		"/etc/mxil/config.yml",
		filepath.Join(os.Getenv("HOME"), ".mxil", "config.yml"),
	}

	var configFile string
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			configFile = path
			break
		}
	}

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	overrideFromEnv(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func overrideFromEnv(cfg *Config) {
	// Server
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		port := 0
		fmt.Sscanf(v, "%d", &port)
		if port > 0 {
			cfg.Server.Port = port
		}
	}

	// Database
	if v := os.Getenv("DATABASE_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DATABASE_PORT"); v != "" {
		port := 0
		fmt.Sscanf(v, "%d", &port)
		if port > 0 {
			cfg.Database.Port = port
		}
	}
	if v := os.Getenv("DATABASE_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DATABASE_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DATABASE_NAME"); v != "" {
		cfg.Database.Database = v
	}
	if v := os.Getenv("DATABASE_SSL_MODE"); v != "" {
		cfg.Database.SSLMode = v
	}

	// Security
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Security.JWTSecret = v
	}
	if v := os.Getenv("ENCRYPTION_KEY"); v != "" {
		cfg.Security.EncryptionKey = v
	}
	if v := os.Getenv("JWT_EXPIRATION"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			cfg.Security.JWTExpiration = dur
		}
	}

	// Email
	if v := os.Getenv("EMAIL_DOMAIN"); v != "" {
		cfg.Email.Domain = v
	}
	if v := os.Getenv("DEFAULT_FROM"); v != "" {
		cfg.Email.DefaultFrom = v
	}

	// Network
	if v := os.Getenv("ENABLE_I2P"); v != "" {
		cfg.Network.EnableI2P = v == "true"
	}
	if v := os.Getenv("ENABLE_TOR"); v != "" {
		cfg.Network.EnableTor = v == "true"
	}

	// Storage
	if v := os.Getenv("STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("STORAGE_PATH"); v != "" {
		cfg.Storage.LocalPath = v
	}

	// Redis
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		port := 0
		fmt.Sscanf(v, "%d", &port)
		if port > 0 {
			cfg.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
}

func validateConfig(cfg *Config) error {
	// Validate security settings
	if cfg.Security.JWTSecret == "" || cfg.Security.JWTSecret == "change_me_to_random_string_at_least_32_chars" {
		return fmt.Errorf("JWT secret must be set and secure")
	}
	if cfg.Security.EncryptionKey == "" || cfg.Security.EncryptionKey == "32_byte_encryption_key_change_me_in_production" {
		return fmt.Errorf("encryption key must be set and secure")
	}
	if len(cfg.Security.JWTSecret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters")
	}

	// Validate database settings
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if cfg.Database.Password == "" {
		return fmt.Errorf("database password is required")
	}
	if cfg.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate email settings
	if cfg.Email.Domain == "" || cfg.Email.Domain == "mxil.example.com" {
		return fmt.Errorf("email domain must be configured")
	}

	return nil
}

// GetDSN returns database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// GetRedisAddr returns Redis address
func (c *RedisConfig) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
