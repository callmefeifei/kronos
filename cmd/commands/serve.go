package commands

import (
	"fmt"
	"log/slog"

	"github.com/pstrr/kronos/internal/config"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Kronos server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			config.SetupLogging(cfg.Log)

			slog.Info("configuration loaded successfully")
			slog.Info("server",
				"host", cfg.Server.Host,
				"port", cfg.Server.Port,
				"mode", cfg.Server.Mode)
			slog.Info("database",
				"sqlite_path", cfg.Database.SQLite.Path,
				"mysql_enabled", cfg.Database.MySQL.Enabled,
				"redis_enabled", cfg.Database.Redis.Enabled)
			slog.Info("scheduler",
				"max_concurrent", cfg.Scheduler.MaxConcurrent)
			slog.Info("mcp",
				"enabled", cfg.MCP.Enabled,
				"host", cfg.MCP.Host,
				"port", cfg.MCP.Port)
			slog.Info("auth",
				"access_ttl", cfg.Auth.AccessTokenTTL,
				"refresh_ttl", cfg.Auth.RefreshTokenTTL)
			slog.Info("notifier",
				"feishu", cfg.Notifier.Feishu.Enabled,
				"webhook", cfg.Notifier.Webhook.Enabled)

			// TODO: start HTTP server, scheduler, MCP server
			slog.Info("server not yet implemented, exiting")
			return nil
		},
	}
}
