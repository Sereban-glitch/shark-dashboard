package collector

import (
        "log"
        "sync"
        "time"

        "shark-dashboard/internal/model"
        "shark-dashboard/internal/supervisor"
)

// Hub aggregates all metric collectors and provides thread-safe
// access to the latest metrics snapshot.
//
// Architecture:
//   - Background goroutine: runs collectAndStore() on a ticker → writes under WriteLock
//   - SSE/HTTP handlers: read cached snapshot under ReadLock via GetMetrics()
//
// This ensures zero data races between the collector goroutine and HTTP handlers.
type Hub struct {
        mu       sync.RWMutex
        last     model.Metrics // latest snapshot, protected by mu
        cpu      *CPUCollector
        memory   *MemoryCollector
        disk     *DiskCollector
        network  *NetCollector
        battery  *BatteryCollector
        uptime   *UptimeCollector
        svClient *supervisor.Client
        stopCh   chan struct{}
}

// NewHub creates a new collector hub and starts the background collection goroutine.
func NewHub(supervisorAddr string, interval time.Duration) *Hub {
        h := &Hub{
                cpu:      NewCPUCollector(),
                memory:   NewMemoryCollector(),
                disk:     NewDiskCollector(),
                network:  NewNetCollector(),
                battery:  NewBatteryCollector(),
                uptime:   NewUptimeCollector(),
                svClient: supervisor.NewClient(supervisorAddr),
                stopCh:   make(chan struct{}),
        }

        // Perform initial collection synchronously so data is available immediately
        h.collectAndStore()

        // Start background collection goroutine
        go h.collectLoop(interval)

        return h
}

// collectLoop runs the collection on a ticker in a background goroutine.
func (h *Hub) collectLoop(interval time.Duration) {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
                select {
                case <-ticker.C:
                        h.collectAndStore()
                case <-h.stopCh:
                        return
                }
        }
}

// collectAndStore gathers all metrics and stores them under a write lock.
// Individual collector panics (e.g. permission denied on /proc) are caught
// by each collector's own recover(). This method also has a top-level recover
// as a last resort to keep the collectLoop alive.
func (h *Hub) collectAndStore() {
        defer func() {
                if r := recover(); r != nil {
                        log.Printf("Hub: recovered from panic in collectAndStore: %v", r)
                }
        }()

        m := model.Metrics{
                CPU:       h.cpu.Collect(),
                Memory:    h.memory.Collect(),
                Disks:     h.disk.Collect(),       // always returns [] (never nil)
                Network:   h.network.Collect(),    // always returns NetMetrics with non-nil Interfaces
                Uptime:    h.uptime.Collect(),
                Battery:   h.battery.Collect(),     // may be nil — uses omitempty
                Processes: make([]model.ProcessInfo, 0), // init as empty slice, never null in JSON
        }

        // Fetch Supervisord processes (best-effort, 2s timeout)
        processes, err := h.svClient.GetAllProcessInfo()
        if err == nil && len(processes) > 0 {
                m.Processes = processes
        }

        h.mu.Lock()
        h.last = m
        h.mu.Unlock()
}

// GetMetrics returns the latest metrics snapshot under a read lock.
// This is called by SSE handlers and HTTP API handlers — never blocks the collector.
func (h *Hub) GetMetrics() model.Metrics {
        h.mu.RLock()
        defer h.mu.RUnlock()
        return h.last
}

// VerifySupervisor checks if Supervisord is reachable.
func (h *Hub) VerifySupervisor() error {
        return h.svClient.VerifyConnection()
}

// Stop terminates the background collection goroutine.
func (h *Hub) Stop() {
        close(h.stopCh)
        log.Println("Hub: collection goroutine stopped")
}
