package collector

import (
        "context"
        "encoding/json"
        "fmt"
        "log"
        "os/exec"
        "strings"
        "time"

        "shark-dashboard/internal/model"
)

// PM2Collector reads process status via PM2's JSON API.
// Implements the ProcessManager interface.
//
// Uses `pm2 jlist` which returns a JSON array of process descriptors.
// Zero external dependencies beyond pm2 itself — uses only os/exec.
type PM2Collector struct {
        timeout time.Duration // max time for pm2 command
}

// NewPM2Collector creates a new PM2 process collector.
func NewPM2Collector() *PM2Collector {
        return &PM2Collector{
                timeout: 5 * time.Second,
        }
}

// GetName returns the process manager name for UI display.
func (c *PM2Collector) GetName() string {
        return "pm2"
}

// IsAvailable checks if pm2 is installed and accessible.
// Implements ProcessManager interface.
func (c *PM2Collector) IsAvailable() bool {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()

        cmd := exec.CommandContext(ctx, "pm2", "-v")
        if err := cmd.Run(); err != nil {
                return false
        }
        return true
}

// GetProcesses fetches the list of PM2-managed processes.
// Implements ProcessManager interface.
// Returns a non-nil slice (empty if pm2 unavailable) — never null in JSON.
func (c *PM2Collector) GetProcesses() ([]model.ProcessInfo, error) {
        defer func() {
                if r := recover(); r != nil {
                        log.Printf("PM2Collector: recovered from panic: %v", r)
                }
        }()

        processes := make([]model.ProcessInfo, 0)

        ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
        defer cancel()

        cmd := exec.CommandContext(ctx, "pm2", "jlist")
        out, err := cmd.Output()
        if err != nil {
                return processes, fmt.Errorf("pm2 jlist failed: %w", err)
        }

        processes = c.parseJList(string(out))
        return processes, nil
}

// pm2Process represents a single process in pm2 jlist JSON output.
type pm2Process struct {
        Name   string    `json:"name"`
        Pid    int       `json:"pid"`
        PM2Env pm2Env    `json:"pm2_env"`
        Monit  pm2Monit  `json:"monit"`
}

type pm2Env struct {
        Status          string `json:"status"`            // "online", "stopped", "errored", "launching", "stopping"
        PMU             int    `json:"pm_uptime"`         // uptime in milliseconds
        RestartTime     int    `json:"restart_time"`      // number of restarts
        UnstableRestarts int   `json:"unstable_restarts"` // consecutive unstable restarts
}

type pm2Monit struct {
        Memory int64 `json:"memory"` // bytes
        CPU    float64 `json:"cpu"`  // percentage
}

// parseJList converts pm2 jlist JSON output into ProcessInfo slices.
//
// Example JSON structure (simplified):
//
//      [
//        {
//          "name": "my-app",
//          "pid": 12345,
//          "pm2_env": {
//            "status": "online",
//            "pm_uptime": 1700000000000
//          },
//          "monit": {
//            "memory": 54525952,
//            "cpu": 2.5
//          }
//        }
//      ]
func (c *PM2Collector) parseJList(output string) []model.ProcessInfo {
        processes := make([]model.ProcessInfo, 0)

        var pm2List []pm2Process
        if err := json.Unmarshal([]byte(output), &pm2List); err != nil {
                log.Printf("PM2Collector: failed to parse jlist JSON: %v", err)
                return processes
        }

        for _, p := range pm2List {
                stateCode, stateName := mapPM2State(p.PM2Env.Status)

                // Convert pm_uptime from milliseconds to unix timestamp
                var startTime int64
                if p.PM2Env.PMU > 0 {
                        startTime = int64(p.PM2Env.PMU) / 1000 // ms → seconds (unix timestamp)
                }

                // Format memory
                ram := model.FormatBytesMB(p.Monit.Memory)

                processes = append(processes, model.ProcessInfo{
                        Name:        p.Name,
                        Group:       "pm2",
                        PID:         p.Pid,
                        State:       stateCode,
                        StateName:   stateName,
                        RAM:         ram,
                        RAMBytes:    p.Monit.Memory,
                        StartTime:   startTime,
                        ExitStatus:  0,
                        Description: fmt.Sprintf("restarts: %d", p.PM2Env.RestartTime),
                })
        }

        return processes
}

// mapPM2State converts PM2 status strings to unified state codes.
//
// PM2 statuses: "online", "stopping", "stopped", "launching", "errored"
//
// Unified codes (compatible with Supervisord/systemd):
//   - 20 = RUNNING  (PM2: online)
//   - 200 = FATAL   (PM2: errored)
//   - 10 = STARTING (PM2: launching)
//   - 40 = STOPPING (PM2: stopping)
//   - 0 = STOPPED   (PM2: stopped)
func mapPM2State(status string) (code int, name string) {
        switch status {
        case "online":
                return 20, "RUNNING"
        case "errored":
                return 200, "FAILED"
        case "launching":
                return 10, "STARTING"
        case "stopping":
                return 40, "STOPPING"
        case "stopped":
                return 0, "STOPPED"
        default:
                return 0, strings.ToUpper(status)
        }
}
