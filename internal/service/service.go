package service

import (
	"os"
	"path/filepath"
)

// ServiceStatus holds the current status of the kronos daemon service.
type ServiceStatus struct {
	Running bool
	PID     int
	Message string
}

// ServiceManager defines the interface for installing, uninstalling, and
// controlling the kronos daemon as a native system service.
type ServiceManager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (ServiceStatus, error)
}

// executablePath returns the absolute, symlink-resolved path to the running
// kronos binary.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// kronosHome returns the path to the kronos home directory (~/.kronos).
func kronosHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".kronos")
	}
	return filepath.Join(home, ".kronos")
}

// LogDir returns the path to the kronos log directory (~/.kronos/logs) and
// ensures the directory exists.
func LogDir() string {
	dir := filepath.Join(kronosHome(), "logs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
