//go:build linux

package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const unitTemplate = `[Unit]
Description=Kronos Scheduled Task Manager
After=network.target

[Service]
ExecStart={{.ExecStart}} serve
Restart=on-failure
RestartSec=5
Environment="HOME=%h"
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`

type unitData struct {
	ExecStart string
}

type linuxService struct{}

// NewServiceManager returns a ServiceManager backed by systemd user service.
func NewServiceManager() ServiceManager {
	return &linuxService{}
}

// unitFilePath returns the path to the systemd user unit file.
func unitFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", "kronos.service"), nil
}

// Install writes the systemd unit file and enables the service.
func (s *linuxService) Install() error {
	exePath, err := executablePath()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	unitPath, err := unitFilePath()
	if err != nil {
		return err
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("cannot create systemd user directory: %w", err)
	}

	// Render the unit file template.
	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return fmt.Errorf("cannot parse unit template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, unitData{ExecStart: exePath}); err != nil {
		return fmt.Errorf("cannot render unit template: %w", err)
	}

	if err := os.WriteFile(unitPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("cannot write unit file: %w", err)
	}

	// Reload systemd and enable the service.
	if err := systemctlUser("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload failed: %w", err)
	}
	if err := systemctlUser("enable", "kronos"); err != nil {
		return fmt.Errorf("enable kronos failed: %w", err)
	}

	return nil
}

// Uninstall stops (if running), disables, and removes the systemd unit file.
func (s *linuxService) Uninstall() error {
	// Stop first (ignore error — may not be running).
	_ = systemctlUser("stop", "kronos")

	if err := systemctlUser("disable", "kronos"); err != nil {
		// Ignore "not found / not enabled" errors — unit may never have been enabled.
		if !isNotFoundError(err) {
			return fmt.Errorf("disable kronos failed: %w", err)
		}
	}

	unitPath, err := unitFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove unit file: %w", err)
	}

	if err := systemctlUser("daemon-reload"); err != nil {
		return fmt.Errorf("daemon-reload failed: %w", err)
	}

	return nil
}

// Start starts the kronos service via systemd.
func (s *linuxService) Start() error {
	if err := systemctlUser("start", "kronos"); err != nil {
		return fmt.Errorf("start kronos failed: %w", err)
	}
	return nil
}

// Stop stops the kronos service via systemd.
func (s *linuxService) Stop() error {
	if err := systemctlUser("stop", "kronos"); err != nil {
		return fmt.Errorf("stop kronos failed: %w", err)
	}
	return nil
}

// Status returns the current running state and PID of the kronos service.
func (s *linuxService) Status() (ServiceStatus, error) {
	// Determine running state.
	activeOut, err := exec.Command("systemctl", "--user", "is-active", "kronos").Output()
	active := strings.TrimSpace(string(activeOut))
	running := err == nil && active == "active"

	// Query MainPID via `systemctl --user show`.
	showOut, showErr := exec.Command("systemctl", "--user", "show", "kronos", "--property=MainPID").Output()
	pid := 0
	if showErr == nil {
		for _, line := range strings.Split(string(showOut), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "MainPID=") {
				val := strings.TrimPrefix(line, "MainPID=")
				if n, parseErr := strconv.Atoi(val); parseErr == nil {
					pid = n
				}
			}
		}
	}

	msg := active
	if msg == "" {
		msg = "unknown"
	}

	return ServiceStatus{
		Running: running,
		PID:     pid,
		Message: msg,
	}, nil
}

// systemctlUser runs `systemctl --user <args...>` and returns any error.
func systemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isNotFoundError returns true when the systemctl error message indicates that
// the unit was not found or not loaded — so we can safely ignore it during
// Uninstall when the unit may never have been installed.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "not loaded")
}
