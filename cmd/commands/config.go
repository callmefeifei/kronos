package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfigYAML = `server:
  host: 0.0.0.0
  port: 8360
  mode: release              # debug / release

auth:
  jwt_secret: ""             # auto-generate if empty
  access_token_ttl: 12h
  refresh_token_ttl: 168h    # 7 days

database:
  sqlite:
    path: ~/.kronos/kronos.db
  mysql:
    enabled: false
    host: ""
    port: 3306
    user: ""
    password: ""
    database: ""
  redis:
    enabled: false
    host: ""
    port: 6379
    password: ""
    db: 0

scheduler:
  max_concurrent: 10

mcp:
  enabled: true
  host: 127.0.0.1
  port: 8361
  token: ""                  # auto-generate if empty

log:
  level: info                # debug / info / warn / error
  format: text               # text / json

notifier:
  feishu:
    enabled: false
    webhook_url: ""
  webhook:
    enabled: false
    url: ""
    headers: {}
`

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}

	cmd.AddCommand(newConfigInitCmd())

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write default kronos.yaml to ~/.kronos/kronos.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}

			dir := filepath.Join(home, ".kronos")
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", dir, err)
			}

			cfgPath := filepath.Join(dir, "kronos.yaml")

			// Check if file already exists.
			if _, err := os.Stat(cfgPath); err == nil {
				fmt.Printf("Config file already exists: %s\n", cfgPath)
				fmt.Println("Remove it first if you want to regenerate.")
				return nil
			}

			if err := os.WriteFile(cfgPath, []byte(defaultConfigYAML), 0644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Printf("Default configuration written to %s\n", cfgPath)
			return nil
		},
	}
}
