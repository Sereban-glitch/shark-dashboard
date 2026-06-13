package collector

import (
	"fmt"
	"log"

	"shark-dashboard/internal/model"
	"shark-dashboard/internal/supervisor"
)

// SupervisordAdapter wraps the supervisor.Client to implement the ProcessManager interface.
// This keeps the supervisor package clean (no dependency on collector) while
// allowing the Hub to treat all managers uniformly.
type SupervisordAdapter struct {
	client *supervisor.Client
}

// NewSupervisordAdapter creates a new ProcessManager adapter for Supervisord.
func NewSupervisordAdapter(addr string) *SupervisordAdapter {
	return &SupervisordAdapter{
		client: supervisor.NewClient(addr),
	}
}

// GetName returns the process manager name for UI display.
func (a *SupervisordAdapter) GetName() string {
	return "supervisord"
}

// IsAvailable checks if Supervisord is reachable via XML-RPC.
// Implements ProcessManager interface.
func (a *SupervisordAdapter) IsAvailable() bool {
	return a.client.VerifyConnection() == nil
}

// GetProcesses fetches all managed processes from Supervisord.
// Implements ProcessManager interface.
// Returns a non-nil slice (empty on error) — never null in JSON.
func (a *SupervisordAdapter) GetProcesses() ([]model.ProcessInfo, error) {
	processes, err := a.client.GetAllProcessInfo()
	if err != nil {
		return make([]model.ProcessInfo, 0), fmt.Errorf("supervisord: %w", err)
	}

	// Supervisord doesn't provide per-process memory via XML-RPC.
	// We could enrich via /proc/<pid>/status, but that adds complexity.
	// For now, set RAM to "-" (not available from XML-RPC).
	for i := range processes {
		if processes[i].RAM == "" {
			processes[i].RAM = "-"
		}
	}

	log.Printf("SupervisordAdapter: collected %d processes", len(processes))
	return processes, nil
}
