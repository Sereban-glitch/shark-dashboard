package collector

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"shark-dashboard/internal/model"
)

// CPUCollector reads /proc/stat to calculate CPU usage.
// In PRoot environments where /proc/stat may be blocked or unreliable,
// it falls back to /proc/loadavg for load average data.
// All /proc and /sys reads are wrapped in recover() to prevent panics
// from permission denied errors on Android.
type CPUCollector struct {
	prevIdle  uint64
	prevTotal uint64
	statOk    bool // tracks if /proc/stat is usable
}

// NewCPUCollector creates a new CPU collector.
func NewCPUCollector() *CPUCollector {
	c := &CPUCollector{statOk: true}
	// Initial reading to establish baseline
	c.Collect()
	time.Sleep(100 * time.Millisecond)
	return c
}

// Collect returns CPU metrics.
func (c *CPUCollector) Collect() (m model.CPUMetrics) {
	// Catch any panic from permission denied on /proc reads (Android/PRoot)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("CPU collector: recovered from panic: %v", r)
		}
	}()

	// Count CPU cores (also works in most PRoot environments)
	m.CoreCount = getCPUCoreCount()

	// Try /proc/stat first (precise CPU usage percentage)
	usage, ok := c.getCPUUsageFromStat()
	if ok {
		m.Usage = usage
		m.Source = "stat"
		c.statOk = true
	} else {
		// Fallback: estimate from load average
		// loadavg / coreCount gives rough utilization ratio
		c.statOk = false
		load1, load5, load15 := getLoadAvg()
		m.LoadAvg1 = load1
		m.LoadAvg5 = load5
		m.LoadAvg15 = load15

		if m.CoreCount > 0 {
			estUsage := (load1 / float64(m.CoreCount)) * 100.0
			if estUsage > 100 {
				estUsage = 100
			}
			if estUsage < 0 {
				estUsage = 0
			}
			m.Usage = estUsage
		}
		m.Source = "loadavg"
	}

	// Always read loadavg as supplementary data (it's cheap and always available)
	load1, load5, load15 := getLoadAvg()
	m.LoadAvg1 = load1
	m.LoadAvg5 = load5
	m.LoadAvg15 = load15

	// Try to read CPU temperature
	m.Temp = getCPUTemp()

	return m
}

func (c *CPUCollector) getCPUUsageFromStat() (result float64, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("CPU stat: recovered: %v", r)
			result = 0
			ok = false
		}
	}()

	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, false
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return 0, false
	}

	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0, false
	}

	var idle, total uint64
	for i, val := range fields[1:] {
		v, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			continue
		}
		total += v
		if i == 3 || i == 4 { // idle + iowait
			idle += v
		}
	}

	if total == c.prevTotal {
		if c.statOk {
			return 0, true
		}
		return 0, false
	}

	deltaTotal := total - c.prevTotal
	deltaIdle := idle - c.prevIdle

	c.prevTotal = total
	c.prevIdle = idle

	if deltaTotal == 0 {
		return 0, true
	}

	usage := (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage, true
}

// getLoadAvg reads /proc/loadavg for 1, 5, 15 minute load averages.
func getLoadAvg() (float64, float64, float64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("loadavg: recovered: %v", r)
		}
	}()

	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, 0
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 3 {
		return 0, 0, 0
	}

	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)

	return load1, load5, load15
}

func getCPUCoreCount() int {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("cpuinfo: recovered: %v", r)
		}
	}()

	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return getCPUCoreCountFromSys()
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	if count == 0 {
		return getCPUCoreCountFromSys()
	}
	return count
}

func getCPUCoreCountFromSys() int {
	data, err := os.ReadFile("/sys/devices/system/cpu/present")
	if err != nil {
		return 1
	}
	s := strings.TrimSpace(string(data))
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		if len(parts) == 2 {
			max, err := strconv.Atoi(parts[1])
			if err == nil {
				return max + 1
			}
		}
	}
	return 1
}

func getCPUTemp() float64 {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("cputemp: recovered: %v", r)
		}
	}()

	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp", i)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(data))
		tempMilli, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		return tempMilli / 1000.0
	}
	return -1
}
