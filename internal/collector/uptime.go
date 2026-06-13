package collector

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

// UptimeCollector reads /proc/uptime for system uptime.
type UptimeCollector struct{}

// NewUptimeCollector creates a new uptime collector.
func NewUptimeCollector() *UptimeCollector {
	return &UptimeCollector{}
}

// Collect returns system uptime in seconds.
func (c *UptimeCollector) Collect() (result int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Uptime collector: recovered from panic: %v", r)
			result = 0
		}
	}()

	f, err := os.Open("/proc/uptime")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 1 {
		return 0
	}

	uptimeFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}

	return int64(uptimeFloat)
}
