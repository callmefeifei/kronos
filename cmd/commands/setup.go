package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type mcpPlatform struct {
	Name       string
	ConfigPath string // relative to home dir
}

var mcpPlatforms = []mcpPlatform{
	{Name: "claude", ConfigPath: ".claude.json"},
	{Name: "cursor", ConfigPath: filepath.Join(".cursor", "mcp.json")},
	{Name: "opencode", ConfigPath: filepath.Join(".config", "opencode", "config.json")},
	{Name: "windsurf", ConfigPath: filepath.Join(".codeium", "windsurf", "mcp_config.json")},
}

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup integrations and configurations",
	}

	cmd.AddCommand(newSetupMCPCmd())

	return cmd
}

func newSetupMCPCmd() *cobra.Command {
	var (
		platform string
		printOnly bool
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Generate MCP client configuration for AI tool platforms",
		Long: `Generate and write MCP client configuration for AI tool platforms.

Supported platforms: claude, cursor, opencode, windsurf.

Without --platform, presents a numbered list for interactive selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetupMCP(platform, printOnly, force)
		},
	}

	cmd.Flags().StringVarP(&platform, "platform", "p", "", "target platform (claude/cursor/opencode/windsurf)")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print configuration to stdout only")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing kronos entry")

	return cmd
}

func runSetupMCP(platform string, printOnly, force bool) error {
	// Resolve kronos binary path.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("detect kronos binary: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolve kronos binary path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configFlag := filepath.Join("~", ".kronos", "kronos.yaml")

	// Build the kronos MCP entry.
	kronosEntry := map[string]interface{}{
		"command": exePath,
		"args":    []string{"mcp", "--config", configFlag},
	}

	if printOnly {
		wrapper := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"kronos": kronosEntry,
			},
		}
		data, err := json.MarshalIndent(wrapper, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Determine target platform.
	var target mcpPlatform
	if platform != "" {
		found := false
		for _, p := range mcpPlatforms {
			if strings.EqualFold(p.Name, platform) {
				target = p
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown platform %q; supported: claude, cursor, opencode, windsurf", platform)
		}
	} else {
		// Interactive selection.
		fmt.Println("Select target platform:")
		for i, p := range mcpPlatforms {
			fmt.Printf("  %d) %s  (%s)\n", i+1, p.Name, filepath.Join("~", p.ConfigPath))
		}
		fmt.Print("\nEnter number: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		input = strings.TrimSpace(input)
		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(mcpPlatforms) {
			return fmt.Errorf("invalid selection: %s", input)
		}
		target = mcpPlatforms[num-1]
	}

	configPath := filepath.Join(home, target.ConfigPath)

	// Merge or create config file.
	return writeMCPConfig(configPath, kronosEntry, force, target.Name)
}

func writeMCPConfig(configPath string, kronosEntry map[string]interface{}, force bool, platformName string) error {
	var root map[string]interface{}

	data, err := os.ReadFile(configPath)
	if err == nil {
		// File exists — parse it.
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse existing %s: %w", configPath, err)
		}

		// Check for existing kronos key.
		servers, ok := root["mcpServers"].(map[string]interface{})
		if ok {
			if _, exists := servers["kronos"]; exists && !force {
				fmt.Printf("⚠ kronos entry already exists in %s\n", configPath)
				fmt.Println("Use --force to overwrite.")
				return nil
			}
		}
		if servers == nil {
			servers = make(map[string]interface{})
		}
		servers["kronos"] = kronosEntry
		root["mcpServers"] = servers
	} else if os.IsNotExist(err) {
		// Create new file.
		if dir := filepath.Dir(configPath); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", dir, err)
			}
		}
		root = map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"kronos": kronosEntry,
			},
		}
	} else {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	if err := os.WriteFile(configPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	fmt.Printf("MCP configuration written to %s (platform: %s)\n", configPath, platformName)
	return nil
}
