package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pstrr/kronos/internal/apiclient"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users",
	}

	cmd.AddCommand(newUserListCmd())
	cmd.AddCommand(newUserCreateCmd())
	cmd.AddCommand(newUserDeleteCmd())

	return cmd
}

func newUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Get("/api/v1/users?size=100")
			if err != nil {
				return err
			}

			items, err := apiclient.ExtractPagedItems(resp)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tUSERNAME\tDISPLAY NAME\tROLE\tSTATUS")
			fmt.Fprintln(w, "--\t--------\t------------\t----\t------")

			for _, item := range items {
				m := item.(map[string]interface{})
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					formatID(m["id"]),
					formatStr(m["username"]),
					formatStr(m["display_name"]),
					formatStr(m["role"]),
					formatStr(m["status"]),
				)
			}
			w.Flush()
			return nil
		},
	}
}

func newUserCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new user (interactive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			reader := bufio.NewReader(os.Stdin)

			username, err := prompt(reader, "Username: ")
			if err != nil {
				return err
			}

			fmt.Print("Password: ")
			passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			fmt.Println()
			password := string(passBytes)

			displayName, err := prompt(reader, "Display name (optional): ")
			if err != nil {
				return err
			}

			fmt.Println("Role (admin/user) [user]:")
			role, err := prompt(reader, "> ")
			if err != nil {
				return err
			}
			role = strings.TrimSpace(role)
			if role == "" {
				role = "user"
			}

			body := map[string]interface{}{
				"username":     username,
				"password":     password,
				"display_name": displayName,
				"role":         role,
			}
			jsonData, _ := json.Marshal(body)

			resp, err := client.Post("/api/v1/users", jsonData)
			if err != nil {
				return err
			}

			data, err := apiclient.ExtractData(resp)
			if err != nil {
				return err
			}

			m := data.(map[string]interface{})
			fmt.Printf("User created successfully (ID: %s, username: %s)\n",
				formatID(m["id"]), formatStr(m["username"]))
			return nil
		},
	}
}

func newUserDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Delete("/api/v1/users/" + args[0])
			if err != nil {
				return err
			}

			if err := apiclient.CheckResponse(resp); err != nil {
				return err
			}

			fmt.Printf("User %s deleted.\n", args[0])
			return nil
		},
	}
}
