package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables, set via -ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Kronos version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Kronos %s\n", Version)
			fmt.Printf("  commit:  %s\n", GitCommit)
			fmt.Printf("  built:   %s\n", BuildTime)
		},
	}
}
