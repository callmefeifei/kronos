package commands

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server over stdio (for AI tool integration)",
		Long: `Start a Model Context Protocol (MCP) server that communicates over
stdin/stdout using JSON-RPC. This is intended for integration with AI
assistants (e.g. Claude Desktop, Cursor) via the stdio transport.

If a Kronos daemon is running, tools proxy through its REST API (proxy mode).
Otherwise the server opens the database directly (embedded mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Force logging to stderr so stdout stays clean for MCP protocol.
			setupStderrLogging(cfg.Log)

			return mcp.StartStdio(cfg, Version)
		},
	}
}

// setupStderrLogging configures slog to write to stderr instead of stdout.
func setupStderrLogging(logCfg config.LogConfig) {
	var level slog.Level
	switch strings.ToLower(logCfg.Level) {
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
	if strings.ToLower(logCfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}
