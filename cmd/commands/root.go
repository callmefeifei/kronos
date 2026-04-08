package commands

import (
	"github.com/spf13/cobra"
)

var cfgFile string

// NewRootCmd creates the root cobra command for Kronos.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kronos",
		Short: "Kronos - Scheduled task management platform",
		Long:  "Kronos is a scheduled task management platform with web UI and MCP support.",
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./kronos.yaml or ~/.kronos/kronos.yaml)")
	root.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8360", "Kronos server URL")

	root.AddCommand(newServeCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newTaskCmd())
	root.AddCommand(newUserCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newSetupCmd())
	root.AddCommand(newServiceCmd())

	return root
}
