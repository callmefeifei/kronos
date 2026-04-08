package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show server status and uptime",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.get("/api/v1/status")
			if err != nil {
				return err
			}

			data, err := extractData(resp)
			if err != nil {
				return err
			}

			m, ok := data.(map[string]interface{})
			if !ok {
				return fmt.Errorf("unexpected response format")
			}

			fmt.Println("Kronos Server Status")
			fmt.Println("--------------------")
			fmt.Printf("  Version:     %s\n", formatStr(m["version"]))
			fmt.Printf("  Uptime:      %s\n", formatStr(m["uptime"]))
			fmt.Printf("  Server Time: %s\n", formatStr(m["server_time"]))
			fmt.Printf("  Go Version:  %s\n", formatStr(m["go_version"]))

			return nil
		},
	}
}
