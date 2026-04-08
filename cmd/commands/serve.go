package commands

import (
	"fmt"
	"log/slog"

	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/server"
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

			slog.Info("configuration loaded",
				"host", cfg.Server.Host,
				"port", cfg.Server.Port,
				"mode", cfg.Server.Mode,
				"sqlite_path", cfg.Database.SQLite.Path,
				"mysql_enabled", cfg.Database.MySQL.Enabled,
				"redis_enabled", cfg.Database.Redis.Enabled,
				"max_concurrent", cfg.Scheduler.MaxConcurrent,
			)

			srv := server.New(cfg)
			return srv.Start()
		},
	}
}
