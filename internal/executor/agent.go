package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/pstrr/kronos/internal/model"
)

// AgentConfig holds optional configuration passed via task.Args.
type AgentConfig struct {
	Provider         string   `json:"provider,omitempty"`             // "mcp_notify" (default) or "cli"
	Model            string   `json:"model,omitempty"`                // claude model (e.g. "sonnet", "opus")
	MaxTurns         int      `json:"max_turns,omitempty"`            // max agentic turns (default 10)
	AllowedTools     []string `json:"allowed_tools,omitempty"`        // tool allowlist
	PermissionMode   string   `json:"permission_mode,omitempty"`      // e.g. "bypassPermissions" for automated use
	WorkDir          string   `json:"work_dir,omitempty"`             // working directory
	SystemPrompt     string   `json:"system_prompt,omitempty"`        // custom system prompt
	MaxBudgetUSD     float64  `json:"max_budget_usd,omitempty"`       // spending cap
	MCPConfig        string   `json:"mcp_config,omitempty"`           // path to MCP config JSON
	AppendPrompt     string   `json:"append_system_prompt,omitempty"` // appended to default system prompt
	CLIPath          string   `json:"cli_path,omitempty"`             // absolute path to claude CLI binary
}

// AgentExecutor runs a prompt via connected MCP clients (default) or the local `claude` CLI.
// The task's Target field contains the natural language instruction (prompt).
// The task's Args field (JSON) contains optional AgentConfig overrides.
type AgentExecutor struct {
	Notifier TaskNotifier   // required for mcp_notify provider
	ResultCh ResultReceiver // required for mcp_notify provider
}

// Execute runs the agent task based on the configured provider.
func (e *AgentExecutor) Execute(ctx context.Context, task *model.Task, runID int64) *RunResult {
	cfg := parseAgentConfig(task.Args)

	if cfg.Provider == "cli" {
		return e.executeCLI(ctx, task, cfg)
	}
	// Default: mcp_notify — remote agent executes, reports result back.
	return e.executeMCPNotify(ctx, task, cfg, runID)
}

// executeMCPNotify pushes the task to connected MCP clients and blocks until the
// remote agent calls report_result (or the context deadline is exceeded).
func (e *AgentExecutor) executeMCPNotify(ctx context.Context, task *model.Task, _ AgentConfig, runID int64) *RunResult {
	start := time.Now()

	if e.Notifier == nil || e.Notifier.ConnectedClients() == 0 {
		return &RunResult{
			Status:   "failed",
			ExitCode: -1,
			Error:    "no MCP clients connected to receive task notification",
			Duration: time.Since(start),
		}
	}

	if e.ResultCh == nil {
		return &RunResult{
			Status:   "failed",
			ExitCode: -1,
			Error:    "result receiver not configured",
			Duration: time.Since(start),
		}
	}

	ch := e.ResultCh.RegisterResultChannel(runID)
	defer e.ResultCh.UnregisterResultChannel(runID)

	e.Notifier.NotifyTaskFired(task, runID)

	select {
	case result := <-ch:
		result.Duration = time.Since(start)
		return result
	case <-ctx.Done():
		status := "timeout"
		if ctx.Err() == context.Canceled {
			status = "failed"
		}
		return &RunResult{
			Status:   status,
			ExitCode: -1,
			Error:    fmt.Sprintf("waiting for remote result: %v", ctx.Err()),
			Duration: time.Since(start),
		}
	}
}

// executeCLI invokes `claude -p "<prompt>"` and captures the output.
func (e *AgentExecutor) executeCLI(ctx context.Context, task *model.Task, cfg AgentConfig) *RunResult {
	start := time.Now()

	args := buildCLIArgs(task.Target, cfg)

	claudeBin := resolveCLIPath(cfg.CLIPath)
	cmd := exec.CommandContext(ctx, claudeBin, args...)

	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	cmd.Env = append(os.Environ(), "CLAUDE_CODE_SIMPLE=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	output := truncateOutput(stdout.String())
	errOutput := stderr.String()

	result := &RunResult{
		Output:   output,
		Duration: duration,
	}

	if err != nil {
		result.Status = "failed"
		if ctx.Err() != nil {
			result.Status = "timeout"
		}
		result.Error = fmt.Sprintf("%s\n%s", err.Error(), truncateOutput(errOutput))
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Status = "success"
		result.ExitCode = 0
	}

	return result
}

// parseAgentConfig extracts AgentConfig from the task's Args JSON.
func parseAgentConfig(data []byte) AgentConfig {
	var cfg AgentConfig
	if len(data) > 0 {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg.Provider == "" {
		cfg.Provider = "mcp_notify" // default: remote agent execution
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 10
	}
	if cfg.PermissionMode == "" {
		cfg.PermissionMode = "bypassPermissions"
	}
	return cfg
}

// resolveCLIPath finds the claude CLI binary.
func resolveCLIPath(override string) string {
	if override != "" {
		return override
	}
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	candidates := []string{
		os.ExpandEnv("$HOME/.nvm/versions/node/v22.22.1/bin/claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
		os.ExpandEnv("$HOME/.local/bin/claude"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "claude"
}

// buildCLIArgs constructs the claude CLI arguments.
func buildCLIArgs(prompt string, cfg AgentConfig) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "text",
		"--max-turns", strconv.Itoa(cfg.MaxTurns),
		"--permission-mode", cfg.PermissionMode,
		"--no-session-persistence",
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", cfg.MaxBudgetUSD))
	}
	if len(cfg.AllowedTools) > 0 {
		for _, tool := range cfg.AllowedTools {
			args = append(args, "--allowedTools", tool)
		}
	}
	if cfg.SystemPrompt != "" {
		args = append(args, "--system-prompt", cfg.SystemPrompt)
	}
	if cfg.AppendPrompt != "" {
		args = append(args, "--append-system-prompt", cfg.AppendPrompt)
	}
	if cfg.MCPConfig != "" {
		args = append(args, "--mcp-config", cfg.MCPConfig)
	}

	return args
}
