package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the top-level configuration struct for Kronos.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	MCP       MCPConfig       `mapstructure:"mcp"`
	Log       LogConfig       `mapstructure:"log"`
	Notifier  NotifierConfig  `mapstructure:"notifier"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type AuthConfig struct {
	JWTSecret       string        `mapstructure:"jwt_secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
}

type DatabaseConfig struct {
	SQLite SQLiteConfig `mapstructure:"sqlite"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type MySQLConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type SchedulerConfig struct {
	MaxConcurrent int `mapstructure:"max_concurrent"`
	// Cleanup settings for old task run records.
	CleanupRetentionDays int `mapstructure:"cleanup_retention_days"` // 0 = disabled
	CleanupIntervalHours int `mapstructure:"cleanup_interval_hours"`
}

type MCPConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Token   string `mapstructure:"token"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type NotifierConfig struct {
	Feishu  FeishuNotifierConfig  `mapstructure:"feishu"`
	Webhook WebhookNotifierConfig `mapstructure:"webhook"`
	Wechat  WechatNotifierConfig  `mapstructure:"wechat"`
}

type FeishuNotifierConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
}

type WebhookNotifierConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	URL     string            `mapstructure:"url"`
	Headers map[string]string `mapstructure:"headers"`
}

type WechatNotifierConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Token   string `mapstructure:"token"`
}

// setDefaults sets all default configuration values.
func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8360)
	viper.SetDefault("server.mode", "release")

	viper.SetDefault("auth.jwt_secret", "")
	viper.SetDefault("auth.access_token_ttl", "12h")
	viper.SetDefault("auth.refresh_token_ttl", "168h") // 7 days

	viper.SetDefault("database.sqlite.path", "~/.kronos/kronos.db")
	viper.SetDefault("database.mysql.enabled", false)
	viper.SetDefault("database.mysql.port", 3306)
	viper.SetDefault("database.redis.enabled", false)
	viper.SetDefault("database.redis.port", 6379)
	viper.SetDefault("database.redis.db", 0)

	viper.SetDefault("scheduler.max_concurrent", 10)
	viper.SetDefault("scheduler.cleanup_retention_days", 30) // keep 30 days by default
	viper.SetDefault("scheduler.cleanup_interval_hours", 6)  // run cleanup every 6 hours

	viper.SetDefault("mcp.enabled", true)
	viper.SetDefault("mcp.host", "127.0.0.1")
	viper.SetDefault("mcp.port", 8361)
	viper.SetDefault("mcp.token", "")

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")

	viper.SetDefault("notifier.feishu.enabled", false)
	viper.SetDefault("notifier.feishu.webhook_url", "")
	viper.SetDefault("notifier.webhook.enabled", false)
	viper.SetDefault("notifier.webhook.url", "")
	viper.SetDefault("notifier.wechat.enabled", false)
	viper.SetDefault("notifier.wechat.url", "https://api.ossec.cn/v1/send")
	viper.SetDefault("notifier.wechat.token", "")
}

// kronosDir returns the path to ~/.kronos/, creating it if needed.
func kronosDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	dir := filepath.Join(home, ".kronos")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}

// generateToken produces a 32-byte hex-encoded random string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Load reads configuration using the following priority:
//
//	--config flag > ./kronos.yaml > ~/.kronos/kronos.yaml
//
// It auto-generates jwt_secret and mcp.token if they are empty,
// writing the updated config back to the file.
func Load(cfgFile string) (*Config, error) {
	setDefaults()

	// Environment variable binding: KRONOS_ prefix
	viper.SetEnvPrefix("KRONOS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	configUsed := ""

	if cfgFile != "" {
		// Explicit --config flag
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %s: %w", cfgFile, err)
		}
		configUsed = cfgFile
	} else {
		// Try ./kronos.yaml first, then ~/.kronos/kronos.yaml.
		// Use explicit file paths instead of SetConfigName + AddConfigPath
		// to avoid viper matching non-YAML files (e.g. a "kronos" binary)
		// that happen to share the same base name.
		kDir, err := kronosDir()
		if err != nil {
			return nil, err
		}

		candidates := []string{
			"kronos.yaml",
			filepath.Join(kDir, "kronos.yaml"),
		}

		found := false
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				viper.SetConfigFile(candidate)
				if err := viper.ReadInConfig(); err != nil {
					return nil, fmt.Errorf("read config %s: %w", candidate, err)
				}
				configUsed = candidate
				found = true
				break
			}
		}

		if !found {
			// No config file found; write defaults to ~/.kronos/kronos.yaml
			viper.SetConfigType("yaml")
			configUsed = filepath.Join(kDir, "kronos.yaml")
		}
	}

	// Auto-generate secrets if empty
	needsWrite := false
	if viper.GetString("auth.jwt_secret") == "" {
		token, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate jwt_secret: %w", err)
		}
		viper.Set("auth.jwt_secret", token)
		needsWrite = true
	}
	if viper.GetString("mcp.token") == "" {
		token, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate mcp.token: %w", err)
		}
		viper.Set("mcp.token", token)
		needsWrite = true
	}

	// Write back if secrets were generated
	if needsWrite && configUsed != "" {
		if err := viper.WriteConfigAs(configUsed); err != nil {
			return nil, fmt.Errorf("write config %s: %w", configUsed, err)
		}
		slog.Info("config file updated with generated secrets", "path", configUsed)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

// SetupLogging configures slog based on the log config.
func SetupLogging(cfg LogConfig) {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
