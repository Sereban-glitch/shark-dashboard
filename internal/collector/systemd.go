package collector

import (
        "bufio"
        "context"
        "fmt"
        "log"
        "os/exec"
        "strconv"
        "strings"
        "time"

        "shark-dashboard/internal/model"
)

// SystemdCollector reads service status via systemctl.
// Implements the ProcessManager interface.
//
// Two-phase collection:
//  1. systemctl list-units — get service names, states, descriptions
//  2. systemctl show (bulk) — get MainPID, MemoryCurrent, ActiveEnterTimestamp
//
// Zero external dependencies — uses only os/exec to call systemctl.
type SystemdCollector struct {
        timeout time.Duration // max time for systemctl command
}

// NewSystemdCollector creates a new systemd service collector.
func NewSystemdCollector() *SystemdCollector {
        return &SystemdCollector{
                timeout: 5 * time.Second,
        }
}

// GetName returns the process manager name for UI display.
func (c *SystemdCollector) GetName() string {
        return "systemd"
}

// IsAvailable checks if systemctl is accessible on this system.
// Implements ProcessManager interface.
func (c *SystemdCollector) IsAvailable() bool {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()

        cmd := exec.CommandContext(ctx, "systemctl", "--version")
        if err := cmd.Run(); err != nil {
                return false
        }
        return true
}

// GetProcesses returns a list of running and failed systemd services with PID and RAM.
// Implements ProcessManager interface.
// Returns a non-nil slice (empty if systemctl unavailable) — never null in JSON.
func (c *SystemdCollector) GetProcesses() ([]model.ProcessInfo, error) {
        defer func() {
                if r := recover(); r != nil {
                        log.Printf("SystemdCollector: recovered from panic: %v", r)
                }
        }()

        processes := make([]model.ProcessInfo, 0)

        // Phase 1: Get service list with states
        units, err := c.listUnits()
        if err != nil {
                return processes, fmt.Errorf("systemctl list-units failed: %w", err)
        }
        if len(units) == 0 {
                return processes, nil
        }

        // Phase 2: Bulk fetch PID, RAM, and start time via systemctl show
        c.enrichWithDetails(units)

        return units, nil
}

// listUnits calls systemctl list-units and parses the output into ProcessInfo structs.
func (c *SystemdCollector) listUnits() ([]model.ProcessInfo, error) {
        ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
        defer cancel()

        cmd := exec.CommandContext(ctx,
                "systemctl", "list-units",
                "--type=service",
                "--state=running,failed",
                "--no-pager",
                "--no-legend",
        )

        out, err := cmd.Output()
        if err != nil {
                // systemctl not found (PRoot/Termux) or no systemd — this is normal
                return make([]model.ProcessInfo, 0), err
        }

        return c.parseListUnits(string(out)), nil
}

// parseListUnits converts systemctl list-units text output into ProcessInfo slices.
//
// Example input line:
//
//      agrobot.service       loaded active running Agrobot Telegram Bot
//      nginx.service         loaded active running A high performance web server
//      fwupd.service         loaded failed  failed  Firmware update daemon
//
// Format: NAME LOAD ACTIVE SUB DESCRIPTION
func (c *SystemdCollector) parseListUnits(output string) []model.ProcessInfo {
        processes := make([]model.ProcessInfo, 0)
        scanner := bufio.NewScanner(strings.NewReader(output))

        for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                if line == "" {
                        continue
                }

                fields := strings.Fields(line)
                if len(fields) < 4 {
                        continue
                }

                name := fields[0]
                activeState := fields[2] // "active", "inactive", "failed", "activating", "deactivating"
                subState := fields[3]    // "running", "failed", "exited", "dead", "auto-restart", ...
                description := ""
                if len(fields) > 4 {
                        description = strings.Join(fields[4:], " ")
                }

                stateCode, stateName := mapSystemdState(activeState, subState)

                processes = append(processes, model.ProcessInfo{
                        Name:        name,
                        Group:       "system",
                        PID:         0,
                        State:       stateCode,
                        StateName:   stateName,
                        RAM:         "-",
                        StartTime:   0,
                        ExitStatus:  0,
                        Description: description,
                })
        }

        return processes
}

// enrichWithDetails performs a bulk systemctl show call to fetch PID, MemoryCurrent,
// and ActiveEnterTimestamp for all services at once. This is much more efficient
// than calling systemctl show per-service.
//
// systemctl show supports multiple units at once:
//
//      systemctl show unit1.service unit2.service ... -p Id,MainPID,MemoryCurrent,ActiveEnterTimestamp
//
// Output is separated by blank lines between units, each property on its own line:
//
//      Id=agrobot.service
//      MainPID=12345
//      MemoryCurrent=54525952
//      ActiveEnterTimestamp=Fri 2024-01-12 10:30:00 UTC
//
//      Id=nginx.service
//      MainPID=23456
//      MemoryCurrent=18432
//      ActiveEnterTimestamp=Fri 2024-01-12 11:00:00 UTC
func (c *SystemdCollector) enrichWithDetails(processes []model.ProcessInfo) {
        if len(processes) == 0 {
                return
        }

        // Build unit list for bulk query
        // systemctl show can handle many units at once, but limit to 100 for safety
        unitNames := make([]string, 0, len(processes))
        for _, p := range processes {
                unitNames = append(unitNames, p.Name)
                if len(unitNames) >= 100 {
                        break
                }
        }

        args := []string{"show"}
        args = append(args, unitNames...)
        args = append(args,
                "--property=Id,MainPID,MemoryCurrent,ActiveEnterTimestamp",
                "--no-pager",
        )

        ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
        defer cancel()

        cmd := exec.CommandContext(ctx, "systemctl", args...)
        out, err := cmd.Output()
        if err != nil {
                // Non-fatal — we just won't have PID/RAM data
                log.Printf("SystemdCollector: systemctl show failed (non-fatal): %v", err)
                return
        }

        // Parse the output: group by blank lines, extract properties
        c.parseShowOutput(string(out), processes)
}

// parseShowOutput parses the bulk systemctl show output and enriches
// the processes slice with PID, RAM, and StartTime data.
func (c *SystemdCollector) parseShowOutput(output string, processes []model.ProcessInfo) {
        // Build a map from service name to ProcessInfo pointer for fast lookup
        procMap := make(map[string]*model.ProcessInfo, len(processes))
        for i := range processes {
                procMap[processes[i].Name] = &processes[i]
        }

        // Split output into blocks separated by blank lines
        blocks := strings.Split(output, "\n\n")

        for _, block := range blocks {
                block = strings.TrimSpace(block)
                if block == "" {
                        continue
                }

                var id, mainPID, memCurrent, timestamp string

                for _, line := range strings.Split(block, "\n") {
                        line = strings.TrimSpace(line)
                        if line == "" {
                                continue
                        }

                        // Split on first = only
                        idx := strings.Index(line, "=")
                        if idx < 0 {
                                continue
                        }
                        key := line[:idx]
                        value := line[idx+1:]

                        switch key {
                        case "Id":
                                id = value
                        case "MainPID":
                                mainPID = value
                        case "MemoryCurrent":
                                memCurrent = value
                        case "ActiveEnterTimestamp":
                                timestamp = value
                        }
                }

                if id == "" {
                        continue
                }

                // Find the matching ProcessInfo
                proc, ok := procMap[id]
                if !ok {
                        continue
                }

                // Set PID
                if mainPID != "" && mainPID != "0" {
                        if pid, err := strconv.Atoi(mainPID); err == nil && pid > 0 {
                                proc.PID = pid
                        }
                }

                // Set RAM — MemoryCurrent is in bytes, may be "[not set]" or "18446744073709551615" (UINT64_MAX = not available)
                if memCurrent != "" && memCurrent != "[not set]" && memCurrent != "18446744073709551615" {
                        if bytes, err := strconv.ParseInt(memCurrent, 10, 64); err == nil && bytes > 0 {
                                proc.RAMBytes = bytes
                                proc.RAM = model.FormatBytesMB(bytes)
                        }
                }

                // Set StartTime
                if timestamp != "" && timestamp != "n/a" {
                        // Try common systemd timestamp formats
                        for _, layout := range []string{
                                "Mon 2006-01-02 15:04:05 MST",
                                "Mon 2006-01-02 15:04:05 -0700",
                                "2006-01-02 15:04:05 MST",
                        } {
                                t, err := time.Parse(layout, timestamp)
                                if err == nil {
                                        proc.StartTime = t.Unix()
                                        break
                                }
                        }
                }
        }
}

// mapSystemdState converts systemd active/sub state pair to unified state codes.
//
// Unified codes (compatible with Supervisord):
//   - 20 = RUNNING  (systemd: active/running, active/exited)
//   - 200 = FATAL   (systemd: failed)
//   - 10 = STARTING (systemd: activating)
//   - 0 = STOPPED   (systemd: inactive, deactivating)
func mapSystemdState(active, sub string) (code int, name string) {
        switch active {
        case "active":
                switch sub {
                case "running":
                        return 20, "RUNNING"
                case "exited":
                        return 20, "RUNNING"
                case "waiting":
                        return 20, "RUNNING"
                default:
                        return 20, "RUNNING"
                }
        case "failed":
                return 200, "FAILED"
        case "activating":
                return 10, "STARTING"
        case "deactivating":
                return 40, "STOPPING"
        case "inactive":
                return 0, "STOPPED"
        default:
                return 0, strings.ToUpper(active)
        }
}
