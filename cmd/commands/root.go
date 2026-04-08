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

	root.AddCommand(newServeCmd())
	root.AddCommand(newVersionCmd())

	return root
}
