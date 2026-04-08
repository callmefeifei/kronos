package commands

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
)

// cliConfig holds the minimal config needed by CLI commands.
type cliConfig struct {
	Auth struct {
		JWTSecret string `mapstructure:"jwt_secret"`
	} `mapstructure:"auth"`
}

// loadConfigForCLI loads config using viper, matching the same search paths as config.Load.
func loadConfigForCLI() (*cliConfig, error) {
	v := viper.New()
	v.SetConfigName("kronos")
	v.SetConfigType("yaml")

	// Explicit --config flag
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(".")
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".kronos"))
		}
	}

	v.SetEnvPrefix("KRONOS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// If config not found, try env vars only
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg cliConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// generateAdminJWT creates a short-lived admin JWT for CLI usage.
func generateAdminJWT(secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":  0,
		"username": "cli-admin",
		"role":     "admin",
		"type":     "access",
		"iss":      "kronos",
		"iat":      now.Unix(),
		"exp":      now.Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
