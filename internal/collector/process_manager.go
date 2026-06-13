package collector

import "shark-dashboard/internal/model"

// ProcessManager is the interface that all process manager collectors must implement.
// This allows the Hub to auto-detect and use whichever manager is available,
// and makes it trivial to add new collectors (Docker, PM2, etc.) in the future.
type ProcessManager interface {
	// IsAvailable checks whether this process manager is present and reachable
	// on the current system. Called once at startup and on each collection cycle
	// when the current manager fails.
	IsAvailable() bool

	// GetName returns a human-readable name for the process manager.
	// Used in logs and in the frontend dynamic header, e.g. "Systemd", "Supervisord", "PM2".
	GetName() string

	// GetProcesses fetches the list of managed processes.
	// Must return a non-nil slice (empty on error) to prevent JSON null.
	GetProcesses() ([]model.ProcessInfo, error)
}
