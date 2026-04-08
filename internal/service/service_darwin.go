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

const (
	serviceLabel = "com.pstrr.kronos"
)

// plistTemplate is the launchd plist XML template for the kronos LaunchAgent.
var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>

	<key>ProgramArguments</key>
	<array>
		<string>{{.ExecPath}}</string>
		<string>serve</string>
	</array>

	<key>RunAtLoad</key>
	<true/>

	<key>KeepAlive</key>
	<true/>

	<key>EnvironmentVariables</key>
	<dict>
		<key>HOME</key>
		<string>{{.HomeDir}}</string>
	</dict>

	<key>StandardOutPath</key>
	<string>{{.StdoutLog}}</string>

	<key>StandardErrorPath</key>
	<string>{{.StderrLog}}</string>
</dict>
</plist>
`))

type plistData struct {
	Label     string
	ExecPath  string
	HomeDir   string
	StdoutLog string
	StderrLog string
}

// darwinService implements ServiceManager for macOS using launchd LaunchAgents.
type darwinService struct{}

// NewServiceManager returns a ServiceManager for macOS.
func NewServiceManager() ServiceManager {
	return &darwinService{}
}

// plistPath returns the path to the LaunchAgent plist file.
func (s *darwinService) plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist"), nil
}

// Install generates the plist and loads the LaunchAgent.
func (s *darwinService) Install() error {
	execPath, err := executablePath()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	logDir := LogDir()
	data := plistData{
		Label:     serviceLabel,
		ExecPath:  execPath,
		HomeDir:   home,
		StdoutLog: filepath.Join(logDir, "kronos.stdout.log"),
		StderrLog: filepath.Join(logDir, "kronos.stderr.log"),
	}

	var buf bytes.Buffer
	if err := plistTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render plist template: %w", err)
	}

	plistPath, err := s.plistPath()
	if err != nil {
		return err
	}

	// Ensure the LaunchAgents directory exists.
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("cannot create LaunchAgents directory: %w", err)
	}

	// Warn if plist already exists; overwrite it.
	if _, statErr := os.Stat(plistPath); statErr == nil {
		fmt.Fprintf(os.Stderr, "warning: plist already exists at %s, overwriting\n", plistPath)
		// Unload the existing service before overwriting so launchctl picks up
		// changes on the subsequent load.
		_ = exec.Command("launchctl", "unload", plistPath).Run()
	}

	if err := os.WriteFile(plistPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// Uninstall stops the service (if running), unloads it, and removes the plist.
func (s *darwinService) Uninstall() error {
	plistPath, err := s.plistPath()
	if err != nil {
		return err
	}

	// Stop if currently running.
	if st, stErr := s.Status(); stErr == nil && st.Running {
		_ = s.Stop()
	}

	// Unload even if not running (idempotent).
	if _, statErr := os.Stat(plistPath); statErr == nil {
		if out, err := exec.Command("launchctl", "unload", plistPath).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl unload failed: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

// Start starts the LaunchAgent service.
func (s *darwinService) Start() error {
	if out, err := exec.Command("launchctl", "start", serviceLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Stop stops the LaunchAgent service.
func (s *darwinService) Stop() error {
	if out, err := exec.Command("launchctl", "stop", serviceLabel).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl stop failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Status queries launchctl to determine if the service is running and its PID.
//
// `launchctl list <label>` outputs a dictionary format like:
//
//	{
//		"PID" = 12345;
//		"Label" = "com.pstrr.kronos";
//		...
//	};
//
// When the service is not loaded the command exits non-zero.
func (s *darwinService) Status() (ServiceStatus, error) {
	out, err := exec.Command("launchctl", "list", serviceLabel).CombinedOutput()
	if err != nil {
		return ServiceStatus{
			Running: false,
			Message: "service not loaded",
		}, nil
	}

	output := string(out)

	// Parse PID from dictionary output: "PID" = 12345;
	var pid int
	var running bool
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"PID"`) {
			// "PID" = 12345;
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.TrimSuffix(val, ";")
				val = strings.TrimSpace(val)
				if p, parseErr := strconv.Atoi(val); parseErr == nil && p > 0 {
					pid = p
					running = true
				}
			}
			break
		}
	}

	msg := "loaded but not running"
	if running {
		msg = fmt.Sprintf("running (PID %d)", pid)
	}

	return ServiceStatus{
		Running: running,
		PID:     pid,
		Message: msg,
	}, nil
}
