package model

import "fmt"

// Metrics contains all system metrics collected for the dashboard.
type Metrics struct {
        CPU           CPUMetrics    `json:"cpu"`
        Memory        MemoryMetrics `json:"memory"`
        Disks         []DiskInfo    `json:"disks"`
        Network       NetMetrics    `json:"network"`
        Processes     []ProcessInfo `json:"processes"`
        ProcessSource string        `json:"processSource"` // "supervisord", "systemd", "pm2", or "" (none detected)
        Battery       *BatteryInfo  `json:"battery,omitempty"`
        Uptime        int64         `json:"uptime"`
}

// CPUMetrics holds CPU utilization data.
type CPUMetrics struct {
        Usage     float64 `json:"usage"`      // percentage 0-100 (from /proc/stat delta)
        CoreCount int     `json:"coreCount"`
        Temp      float64 `json:"temp"`       // temperature in Celsius, -1 if unavailable
        LoadAvg1  float64 `json:"loadAvg1"`   // 1-min load average (from /proc/loadavg)
        LoadAvg5  float64 `json:"loadAvg5"`   // 5-min load average
        LoadAvg15 float64 `json:"loadAvg15"`  // 15-min load average
        Source    string  `json:"source"`     // "stat" or "loadavg" — indicates which source was used
}

// MemoryMetrics holds RAM and Swap usage data.
type MemoryMetrics struct {
        Total       uint64  `json:"total"`       // bytes
        Used        uint64  `json:"used"`        // bytes
        Available   uint64  `json:"available"`   // bytes
        UsedPct     float64 `json:"usedPct"`     // percentage
        SwapTotal   uint64  `json:"swapTotal"`   // bytes
        SwapUsed    uint64  `json:"swapUsed"`    // bytes
        SwapUsedPct float64 `json:"swapUsedPct"` // percentage
}

// DiskInfo holds disk partition usage data.
type DiskInfo struct {
        Device  string  `json:"device"`
        Total   uint64  `json:"total"`   // bytes
        Used    uint64  `json:"used"`    // bytes
        Avail   uint64  `json:"avail"`   // bytes
        UsedPct float64 `json:"usedPct"` // percentage
}

// NetMetrics holds network interface statistics.
type NetMetrics struct {
        Interfaces []NetInterface `json:"interfaces"`
}

// NetInterface holds per-interface network data.
type NetInterface struct {
        Name    string `json:"name"`
        RxBytes uint64 `json:"rxBytes"`  // total bytes received
        TxBytes uint64 `json:"txBytes"`  // total bytes transmitted
        RxRate  uint64 `json:"rxRate"`   // bytes/s since last measurement
        TxRate  uint64 `json:"txRate"`   // bytes/s since last measurement
}

// ProcessInfo holds information about a managed process.
// Used for all process managers (Supervisord, systemd, PM2) — fields are a superset.
// When source is systemd or PM2: some fields may be zero/empty.
type ProcessInfo struct {
        Name        string `json:"name"`
        Group       string `json:"group"`
        PID         int    `json:"pid"`
        State       int    `json:"state"`       // Unified: 0=STOPPED, 20=RUNNING, 200=FATAL
        StateName   string `json:"stateName"`   // human-readable state
        RAM         string `json:"ram"`         // human-readable RAM usage, e.g. "52.1 MB" or "-"
        RAMBytes    int64  `json:"-"`           // raw bytes for sorting (not serialized to JSON)
        StartTime   int64  `json:"startTime"`   // unix timestamp (0 if unknown)
        ExitStatus  int    `json:"exitStatus"`
        Description string `json:"description"`
}

// BatteryInfo holds battery status data from termux-battery-status.
type BatteryInfo struct {
        Percentage int     `json:"percentage"`
        Status     string  `json:"status"`    // CHARGING, DISCHARGING, FULL, NOT_CHARGING
        Health     string  `json:"health"`
        Temp       float64 `json:"temp"`      // temperature in Celsius
}

// StateNameMap maps state codes to human-readable names.
// Supervisord codes: 0=STOPPED, 10=STARTING, 20=RUNNING, 30=BACKOFF, 40=STOPPING, 100=EXITED, 200=FATAL
// Systemd mapped codes: 20=running, 200=failed, 0=stopped/dead, 10=activating
// PM2 mapped codes: 20=online, 0=stopped, 200=errored
var StateNameMap = map[int]string{
        0:   "STOPPED",
        10:  "STARTING",
        20:  "RUNNING",
        30:  "BACKOFF",
        40:  "STOPPING",
        100: "EXITED",
        200: "FATAL",
}

// FormatBytesMB converts bytes to a human-readable MB string.
// Returns "-" if bytes is 0 or negative (unknown/not applicable).
func FormatBytesMB(bytes int64) string {
        if bytes <= 0 {
                return "-"
        }
        mb := float64(bytes) / 1024.0 / 1024.0
        if mb < 0.1 {
                return "< 0.1 MB"
        }
        return fmt.Sprintf("%.1f MB", mb)
}
