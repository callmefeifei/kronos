package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pstrr/kronos/internal/apiclient"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}

	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskInfoCmd())
	cmd.AddCommand(newTaskRunCmd())
	cmd.AddCommand(newTaskEnableCmd())
	cmd.AddCommand(newTaskDisableCmd())
	cmd.AddCommand(newTaskDeleteCmd())
	cmd.AddCommand(newTaskLogsCmd())

	return cmd
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Get("/api/v1/tasks?size=100")
			if err != nil {
				return err
			}

			items, err := apiclient.ExtractPagedItems(resp)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tTYPE\tSCHEDULE\tENABLED\tLAST STATUS\tLAST RUN")
			fmt.Fprintln(w, "--\t----\t----\t--------\t-------\t-----------\t--------")

			for _, item := range items {
				m := item.(map[string]interface{})
				id := formatID(m["id"])
				name := formatStr(m["name"])
				typ := formatStr(m["type"])
				schedExpr := formatStr(m["schedule_expr"])
				enabled := "no"
				if b, ok := m["enabled"].(bool); ok && b {
					enabled = "yes"
				}
				lastStatus := formatStr(m["last_status"])
				if lastStatus == "" {
					lastStatus = "-"
				}
				lastRun := "-"
				if lr, ok := m["last_run_at"].(string); ok && lr != "" {
					if t, err := time.Parse(time.RFC3339, lr); err == nil {
						lastRun = t.Local().Format("2006-01-02 15:04:05")
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					id, name, typ, schedExpr, enabled, lastStatus, lastRun)
			}
			w.Flush()
			return nil
		},
	}
}

func newTaskCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new task (interactive or from YAML file)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			var body map[string]interface{}

			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read file %s: %w", file, err)
				}
				if err := yaml.Unmarshal(data, &body); err != nil {
					return fmt.Errorf("parse YAML: %w", err)
				}
			} else {
				body, err = promptTaskCreate()
				if err != nil {
					return err
				}
			}

			jsonData, _ := json.Marshal(body)
			resp, err := client.Post("/api/v1/tasks", jsonData)
			if err != nil {
				return err
			}

			data, err := apiclient.ExtractData(resp)
			if err != nil {
				return err
			}

			m := data.(map[string]interface{})
			fmt.Printf("Task created successfully (ID: %s)\n", formatID(m["id"]))
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "YAML file with task definition")
	return cmd
}

func newTaskInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <id>",
		Short: "Show detailed task information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Get("/api/v1/tasks/" + args[0])
			if err != nil {
				return err
			}

			data, err := apiclient.ExtractData(resp)
			if err != nil {
				return err
			}

			m := data.(map[string]interface{})
			fmt.Printf("Task #%s\n", formatID(m["id"]))
			fmt.Printf("  Name:           %s\n", formatStr(m["name"]))
			fmt.Printf("  Type:           %s\n", formatStr(m["type"]))
			fmt.Printf("  Schedule Type:  %s\n", formatStr(m["schedule_type"]))
			fmt.Printf("  Schedule Expr:  %s\n", formatStr(m["schedule_expr"]))
			fmt.Printf("  Target:         %s\n", formatStr(m["target"]))
			fmt.Printf("  Timeout:        %ss\n", formatNum(m["timeout"]))
			fmt.Printf("  Retry Count:    %s\n", formatNum(m["retry_count"]))
			fmt.Printf("  Retry Interval: %ss\n", formatNum(m["retry_interval"]))
			enabled := "no"
			if b, ok := m["enabled"].(bool); ok && b {
				enabled = "yes"
			}
			fmt.Printf("  Enabled:        %s\n", enabled)
			fmt.Printf("  Last Status:    %s\n", formatStr(m["last_status"]))
			fmt.Printf("  Notify Channel: %s\n", formatStr(m["notify_channel"]))
			fmt.Printf("  User ID:        %s\n", formatNum(m["user_id"]))
			if t, ok := m["created_at"].(string); ok {
				fmt.Printf("  Created At:     %s\n", t)
			}
			if t, ok := m["next_run_at"].(string); ok && t != "" {
				fmt.Printf("  Next Run At:    %s\n", t)
			}
			if t, ok := m["last_run_at"].(string); ok && t != "" {
				fmt.Printf("  Last Run At:    %s\n", t)
			}
			if a := m["args"]; a != nil {
				j, _ := json.MarshalIndent(a, "                  ", "  ")
				fmt.Printf("  Args:           %s\n", string(j))
			}
			return nil
		},
	}
}

func newTaskRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <id>",
		Short: "Trigger immediate execution of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Post("/api/v1/tasks/"+args[0]+"/run", nil)
			if err != nil {
				return err
			}

			if err := apiclient.CheckResponse(resp); err != nil {
				return err
			}

			fmt.Printf("Task %s execution triggered.\n", args[0])
			return nil
		},
	}
}

func newTaskEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Post("/api/v1/tasks/"+args[0]+"/enable", nil)
			if err != nil {
				return err
			}

			if err := apiclient.CheckResponse(resp); err != nil {
				return err
			}

			fmt.Printf("Task %s enabled.\n", args[0])
			return nil
		},
	}
}

func newTaskDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Post("/api/v1/tasks/"+args[0]+"/disable", nil)
			if err != nil {
				return err
			}

			if err := apiclient.CheckResponse(resp); err != nil {
				return err
			}

			fmt.Printf("Task %s disabled.\n", args[0])
			return nil
		},
	}
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Delete("/api/v1/tasks/" + args[0])
			if err != nil {
				return err
			}

			if err := apiclient.CheckResponse(resp); err != nil {
				return err
			}

			fmt.Printf("Task %s deleted.\n", args[0])
			return nil
		},
	}
}

func newTaskLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <id>",
		Short: "Show last 10 runs of a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient()
			if err != nil {
				return err
			}

			resp, err := client.Get("/api/v1/tasks/" + args[0] + "/runs?size=10&page=1")
			if err != nil {
				return err
			}

			items, err := apiclient.ExtractPagedItems(resp)
			if err != nil {
				return err
			}

			if len(items) == 0 {
				fmt.Println("No runs found for this task.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "RUN ID\tSTATUS\tSTARTED\tDURATION\tOUTPUT PREVIEW")
			fmt.Fprintln(w, "------\t------\t-------\t--------\t--------------")

			for _, item := range items {
				m := item.(map[string]interface{})
				id := formatID(m["id"])
				status := formatStr(m["status"])
				started := "-"
				if s, ok := m["started_at"].(string); ok && s != "" {
					if t, err := time.Parse(time.RFC3339, s); err == nil {
						started = t.Local().Format("2006-01-02 15:04:05")
					}
				}
				duration := "-"
				if d, ok := m["duration_ms"].(float64); ok && d > 0 {
					duration = fmt.Sprintf("%dms", int(d))
				}
				output := formatStr(m["output"])
				output = strings.ReplaceAll(output, "\n", " ")
				if len(output) > 60 {
					output = output[:57] + "..."
				}
				if output == "" {
					output = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					id, status, started, duration, output)
			}
			w.Flush()
			return nil
		},
	}
}

// promptTaskCreate prompts the user interactively for task fields.
func promptTaskCreate() (map[string]interface{}, error) {
	reader := bufio.NewReader(os.Stdin)

	name, err := prompt(reader, "Name: ")
	if err != nil {
		return nil, err
	}

	fmt.Println("Type (remind/script/agent):")
	typ, err := prompt(reader, "> ")
	if err != nil {
		return nil, err
	}

	fmt.Println("Schedule type (cron/interval/once):")
	schedType, err := prompt(reader, "> ")
	if err != nil {
		return nil, err
	}

	fmt.Println("Schedule expression:")
	schedExpr, err := prompt(reader, "> ")
	if err != nil {
		return nil, err
	}

	fmt.Println("Target (script path, message, or agent prompt):")
	target, err := prompt(reader, "> ")
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"name":          name,
		"type":          typ,
		"schedule_type": schedType,
		"schedule_expr": schedExpr,
		"target":        target,
	}
	return body, nil
}

func prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// --- API client helpers (delegates to apiclient package) ---

var serverURL string

func newAPIClient() (*apiclient.Client, error) {
	cfg, err := loadConfigForCLI()
	if err != nil {
		return nil, err
	}

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret not configured; run 'kronos serve' first to auto-generate")
	}

	return apiclient.New(serverURL, cfg.Auth.JWTSecret)
}

// --- Formatting helpers (used by task, user, status commands) ---

func formatID(v interface{}) string {
	if v == nil {
		return "-"
	}
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatStr(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func formatNum(v interface{}) string {
	if v == nil {
		return "0"
	}
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
