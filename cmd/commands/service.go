package commands

import (
	"fmt"
	"runtime"

	"github.com/pstrr/kronos/internal/service"
	"github.com/spf13/cobra"
)

// mechanismName returns the human-readable name of the service mechanism for
// the current platform.
func mechanismName() string {
	switch runtime.GOOS {
	case "darwin":
		return "LaunchAgent"
	case "linux":
		return "systemd"
	case "windows":
		return "Windows Service"
	default:
		return "system service"
	}
}

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the Kronos system service",
		Long:  "Install, uninstall, start, stop, or check the status of the Kronos daemon as a native system service.",
	}

	cmd.AddCommand(newServiceInstallCmd())
	cmd.AddCommand(newServiceUninstallCmd())
	cmd.AddCommand(newServiceStartCmd())
	cmd.AddCommand(newServiceStopCmd())
	cmd.AddCommand(newServiceStatusCmd())

	return cmd
}

func newServiceInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Kronos as a system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := service.NewServiceManager()
			if err := mgr.Install(); err != nil {
				return fmt.Errorf("install service: %w", err)
			}
			logDir := service.LogDir()
			fmt.Println("Kronos service installed. It will start automatically on login.")
			fmt.Printf("Log files: %s\n", logDir)
			return nil
		},
	}
}

func newServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the Kronos system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := service.NewServiceManager()
			if err := mgr.Uninstall(); err != nil {
				return fmt.Errorf("uninstall service: %w", err)
			}
			fmt.Println("Kronos service removed.")
			return nil
		},
	}
}

func newServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Kronos system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := service.NewServiceManager()
			if err := mgr.Start(); err != nil {
				return fmt.Errorf("start service: %w", err)
			}
			fmt.Println("Kronos service started.")
			return nil
		},
	}
}

func newServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Kronos system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := service.NewServiceManager()
			if err := mgr.Stop(); err != nil {
				return fmt.Errorf("stop service: %w", err)
			}
			fmt.Println("Kronos service stopped.")
			return nil
		},
	}
}

func newServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of the Kronos system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := service.NewServiceManager()
			st, err := mgr.Status()
			if err != nil {
				return fmt.Errorf("query service status: %w", err)
			}

			mechanism := mechanismName()
			runningStr := "stopped"
			if st.Running {
				runningStr = "running"
			}

			fmt.Println("Kronos Service Status")
			fmt.Println("---------------------")
			fmt.Printf("  Mechanism: %s\n", mechanism)
			fmt.Printf("  State:     %s\n", runningStr)
			if st.PID > 0 {
				fmt.Printf("  PID:       %d\n", st.PID)
			}
			if st.Message != "" {
				fmt.Printf("  Message:   %s\n", st.Message)
			}

			return nil
		},
	}
}
