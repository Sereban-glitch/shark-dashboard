package collector

import (
	"encoding/json"
	"os/exec"
	"sync"

	"shark-dashboard/internal/model"
)

// BatteryCollector reads battery status via termux-battery-status.
type BatteryCollector struct {
	mu       sync.Mutex
	lastInfo *model.BatteryInfo
	lastOk   bool
}

// NewBatteryCollector creates a new battery collector.
func NewBatteryCollector() *BatteryCollector {
	return &BatteryCollector{}
}

// Collect attempts to read battery info. Returns nil if unavailable.
func (c *BatteryCollector) Collect() *model.BatteryInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	out, err := exec.Command("termux-battery-status").Output()
	if err != nil {
		// Command not available (we're in PRoot Debian, not direct Termux)
		return nil
	}

	var raw struct {
		Percentage int     `json:"percentage"`
		Status     string  `json:"status"`
		Health     string  `json:"health"`
		Temperature float64 `json:"temperature"`
	}

	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	info := &model.BatteryInfo{
		Percentage: raw.Percentage,
		Status:     raw.Status,
		Health:     raw.Health,
		Temp:       raw.Temperature,
	}

	c.lastInfo = info
	c.lastOk = true
	return info
}
