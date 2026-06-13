package collector

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	"shark-dashboard/internal/model"
)

// MemoryCollector reads /proc/meminfo for RAM and Swap metrics.
// Recover from any panic (e.g. permission denied on Android/PRoot).
type MemoryCollector struct{}

// NewMemoryCollector creates a new memory collector.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Collect returns memory metrics.
func (c *MemoryCollector) Collect() (m model.MemoryMetrics) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Memory collector: recovered from panic: %v", r)
		}
	}()

	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return m
	}
	defer f.Close()

	memValues := make(map[string]uint64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimRight(fields[0], ":")
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		memValues[key] = val // in kB
	}

	// Convert kB to bytes
	kb := uint64(1024)
	m.Total = memValues["MemTotal"] * kb
	m.Available = memValues["MemAvailable"] * kb
	m.Used = m.Total - m.Available
	if m.Total > 0 {
		m.UsedPct = (float64(m.Used) / float64(m.Total)) * 100.0
	}

	m.SwapTotal = memValues["SwapTotal"] * kb
	m.SwapUsed = (memValues["SwapTotal"] - memValues["SwapFree"]) * kb
	if m.SwapTotal > 0 {
		m.SwapUsedPct = (float64(m.SwapUsed) / float64(m.SwapTotal)) * 100.0
	}

	return m
}
