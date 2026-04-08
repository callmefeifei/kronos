//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// NOTE: This implementation uses sc.exe for basic Windows Service control.
// For production-grade Windows service support (proper Service Control Handler,
// graceful shutdown via SCM signals, etc.), consider integrating
// golang.org/x/sys/windows/svc instead. The sc.exe approach works because
// `kronos serve` already handles SIGINT/SIGTERM, but the SCM won't be able
// to send pause/continue or custom control codes.

const serviceName = "Kronos"

// windowsService implements ServiceManager using sc.exe.
type windowsService struct{}

// NewServiceManager returns a ServiceManager backed by sc.exe.
func NewServiceManager() ServiceManager {
	return &windowsService{}
}

// Install registers Kronos as a Windows auto-start service.
// Requires Administrator privileges.
func (s *windowsService) Install() error {
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	logDir := LogDir()
	_ = logDir // log directory is prepared; sc.exe itself does not redirect stdout,
	// but the service binary can use logDir (~/.kronos/logs) for its own log files.

	// sc.exe requires spaces after '=' in parameter pairs.
	binPath := fmt.Sprintf(`"%s" serve`, exe)
	out, err := runSC("create", serviceName,
		"binPath=", binPath,
		"start=", "auto",
		"DisplayName=", "Kronos Scheduler",
	)
	if err != nil {
		return adminAwareError("install service", out, err)
	}
	return nil
}

// Uninstall stops the service (if running) and removes it from the SCM.
// Requires Administrator privileges.
func (s *windowsService) Uninstall() error {
	// Best-effort stop before deletion.
	_, _ = runSC("stop", serviceName)

	out, err := runSC("delete", serviceName)
	if err != nil {
		return adminAwareError("uninstall service", out, err)
	}
	return nil
}

// Start starts the Kronos Windows service.
// Requires Administrator privileges.
func (s *windowsService) Start() error {
	out, err := runSC("start", serviceName)
	if err != nil {
		return adminAwareError("start service", out, err)
	}
	return nil
}

// Stop stops the Kronos Windows service.
// Requires Administrator privileges.
func (s *windowsService) Stop() error {
	out, err := runSC("stop", serviceName)
	if err != nil {
		return adminAwareError("stop service", out, err)
	}
	return nil
}

// Status queries the SCM for the current state and PID of the Kronos service.
func (s *windowsService) Status() (ServiceStatus, error) {
	out, err := runSC("query", serviceName)
	if err != nil {
		// "The specified service does not exist" → not installed.
		if strings.Contains(strings.ToLower(out), "does not exist") ||
			strings.Contains(strings.ToLower(out), "1060") {
			return ServiceStatus{Running: false, Message: "service not installed"}, nil
		}
		return ServiceStatus{}, adminAwareError("query service", out, err)
	}

	return parseQueryOutput(out), nil
}

// runSC executes sc.exe with the given arguments and returns combined output.
// sc.exe args must be passed as individual strings; spaces after '=' are
// intentional (sc.exe syntax).
func runSC(args ...string) (string, error) {
	scPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "sc.exe")
	// Fallback: let PATH find sc.exe if SystemRoot is unset.
	if os.Getenv("SystemRoot") == "" {
		scPath = "sc.exe"
	}
	cmd := exec.Command(scPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// adminAwareError wraps an sc.exe error with a hint when it looks like an
// "access denied" / elevation error (exit code 5 or text "Access is denied").
func adminAwareError(op, output string, err error) error {
	lower := strings.ToLower(output)
	// Error code 5 = ERROR_ACCESS_DENIED; sc.exe prints "Error: 5" or "error 5"
	if strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "openscmanager") ||
		strings.Contains(lower, "error: 5") ||
		strings.Contains(lower, "error 5") {
		return fmt.Errorf("%s failed (Administrator privileges required): %w\nsc.exe output: %s", op, err, strings.TrimSpace(output))
	}
	return fmt.Errorf("%s failed: %w\nsc.exe output: %s", op, err, strings.TrimSpace(output))
}

// parseQueryOutput parses the text output of `sc.exe query <name>` and
// extracts the service state and PID.
//
// Example output (relevant lines):
//
//	SERVICE_NAME: Kronos
//	        TYPE               : 10  WIN32_OWN_PROCESS
//	        STATE              : 4  RUNNING
//	                                (STOPPABLE, NOT_PAUSABLE, ACCEPTS_SHUTDOWN)
//	        WIN32_EXIT_CODE    : 0  (0x0)
//	        SERVICE_EXIT_CODE  : 0  (0x0)
//	        CHECKPOINT         : 0x0
//	        WAIT_HINT          : 0x0
//	        PID                : 1234
//	        FLAGS              :
func parseQueryOutput(output string) ServiceStatus {
	var status ServiceStatus
	status.Message = strings.TrimSpace(output)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// STATE line: "STATE              : 4  RUNNING"
		if strings.HasPrefix(upper, "STATE") {
			parts := strings.Fields(trimmed)
			// parts: ["STATE", ":", "<code>", "<name>", ...]
			for _, p := range parts {
				pu := strings.ToUpper(p)
				if pu == "RUNNING" {
					status.Running = true
					status.Message = "running"
				} else if pu == "STOPPED" || pu == "STOP_PENDING" {
					status.Running = false
					status.Message = strings.ToLower(pu)
				} else if pu == "START_PENDING" {
					status.Running = false
					status.Message = "start_pending"
				}
			}
		}

		// PID line: "PID                : 1234"
		if strings.HasPrefix(upper, "PID") {
			parts := strings.Fields(trimmed)
			// parts: ["PID", ":", "<number>"]
			if len(parts) >= 3 {
				if pid, err := strconv.Atoi(parts[2]); err == nil {
					status.PID = pid
				}
			}
		}
	}

	return status
}
