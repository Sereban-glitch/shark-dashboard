package collector

import (
        "bufio"
        "log"
        "os"
        "strconv"
        "strings"
        "sync"
        "time"

        "shark-dashboard/internal/model"
)

// NetCollector reads /proc/net/dev for network statistics.
// The first Collect() call returns zero rates to avoid the initial
// spike that occurs when prevBytes=0 and currentBytes=total.
type NetCollector struct {
        mu        sync.Mutex
        prevStats map[string]netStat
        prevTime  time.Time
        firstTick bool // true until the second Collect() call
}

type netStat struct {
        rxBytes uint64
        txBytes uint64
}

// NewNetCollector creates a new network collector.
func NewNetCollector() *NetCollector {
        c := &NetCollector{
                prevStats: make(map[string]netStat),
                firstTick: true,
        }
        // Initial reading to establish baseline — rates will be zero
        c.prevStats, _ = c.readNetDev()
        c.prevTime = time.Now()
        return c
}

// Collect returns network interface metrics with transfer rates.
// On the first call after initialization, rates are reported as 0
// to prevent the initial spike (e.g. "40 GB/s") caused by computing
// rate from 0 to current total bytes.
// If /proc/net/dev is unavailable (PRoot/Android), returns empty
// NetMetrics with a non-nil Interfaces slice (never null in JSON).
func (c *NetCollector) Collect() model.NetMetrics {
        c.mu.Lock()
        defer c.mu.Unlock()

        // Panic recovery for PRoot environments where /proc reads may fail hard
        defer func() {
                if r := recover(); r != nil {
                        log.Printf("NetCollector: recovered from panic: %v", r)
                }
        }()

        currentStats, _ := c.readNetDev()
        now := time.Now()
        elapsed := now.Sub(c.prevTime).Seconds()

        interfaces := make([]model.NetInterface, 0) // never nil → JSON []

        for name, cur := range currentStats {
                // Skip loopback
                if name == "lo" {
                        continue
                }

                var rxRate, txRate uint64
                // Skip rate calculation on the first real tick to prevent spikes.
                // On startup, prevStats has the initial reading but the delta
                // from 0→initial would be enormous if we calculated rates.
                if !c.firstTick {
                        if prev, ok := c.prevStats[name]; ok && elapsed > 0 {
                                rxRate = uint64(float64(cur.rxBytes-prev.rxBytes) / elapsed)
                                txRate = uint64(float64(cur.txBytes-prev.txBytes) / elapsed)
                        }
                }

                interfaces = append(interfaces, model.NetInterface{
                        Name:    name,
                        RxBytes: cur.rxBytes,
                        TxBytes: cur.txBytes,
                        RxRate:  rxRate,
                        TxRate:  txRate,
                })
        }

        c.prevStats = currentStats
        c.prevTime = now
        c.firstTick = false // After first real Collect, subsequent calls can compute rates

        return model.NetMetrics{Interfaces: interfaces}
}

func (c *NetCollector) readNetDev() (map[string]netStat, error) {
        stats := make(map[string]netStat)

        f, err := os.Open("/proc/net/dev")
        if err != nil {
                return stats, err
        }
        defer f.Close()

        scanner := bufio.NewScanner(f)
        // Skip first 2 header lines
        lineNum := 0
        for scanner.Scan() {
                lineNum++
                if lineNum <= 2 {
                        continue
                }

                line := scanner.Text()
                parts := strings.SplitN(line, ":", 2)
                if len(parts) < 2 {
                        continue
                }

                name := strings.TrimSpace(parts[0])
                fields := strings.Fields(parts[1])
                if len(fields) < 10 {
                        continue
                }

                rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
                txBytes, _ := strconv.ParseUint(fields[8], 10, 64)

                stats[name] = netStat{
                        rxBytes: rxBytes,
                        txBytes: txBytes,
                }
        }

        return stats, nil
}
