package collector

import (
        "log"
        "sort"
        "sync"
        "time"

        "shark-dashboard/internal/model"
)

// Hub aggregates all metric collectors and provides thread-safe
// access to the latest metrics snapshot.
//
// Process source auto-detection uses the ProcessManager interface:
//   - At startup, tries each manager in order (Supervisord → systemd → PM2)
//   - The first available manager (IsAvailable() == true) is used
//   - If the active manager fails during collection, falls back to the next
//   - The detected source is stored in Metrics.ProcessSource so the
//     frontend can dynamically update the section title
type Hub struct {
        mu      sync.RWMutex
        last    model.Metrics // latest snapshot, protected by mu
        cpu     *CPUCollector
        memory  *MemoryCollector
        disk    *DiskCollector
        network *NetCollector
        battery *BatteryCollector
        uptime  *UptimeCollector
        stopCh  chan struct{}

        // managers is the ordered list of process managers to try.
        // First available one is used. Order: Supervisord → systemd → PM2.
        managers []ProcessManager

        // activeManager tracks which manager was last successfully used.
        // Persisted across collection cycles to avoid flapping.
        activeManager ProcessManager
}

// NewHub creates a new collector hub and starts the background collection goroutine.
func NewHub(managers []ProcessManager, interval time.Duration) *Hub {
        h := &Hub{
                cpu:           NewCPUCollector(),
                memory:        NewMemoryCollector(),
                disk:          NewDiskCollector(),
                network:       NewNetCollector(),
                battery:       NewBatteryCollector(),
                uptime:        NewUptimeCollector(),
                managers:      managers,
                activeManager: nil, // unknown until first collection
                stopCh:        make(chan struct{}),
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
                CPU:           h.cpu.Collect(),
                Memory:        h.memory.Collect(),
                Disks:         h.disk.Collect(),       // always returns [] (never nil)
                Network:       h.network.Collect(),    // always returns NetMetrics with non-nil Interfaces
                Uptime:        h.uptime.Collect(),
                Battery:       h.battery.Collect(),     // may be nil — uses omitempty
                Processes:     make([]model.ProcessInfo, 0), // init as empty slice, never null in JSON
                ProcessSource: "",                      // will be set below
        }

        // ===== Process Source Auto-Detection via ProcessManager interface =====
        // Strategy:
        //  1. Try the current/active manager first (avoid flapping)
        //  2. If it fails, iterate through all managers to find an available one
        //  3. First available manager wins

        collected := false

        // Step 1: Try the active manager first
        if h.activeManager != nil {
                processes, err := h.activeManager.GetProcesses()
                if err == nil && len(processes) > 0 {
                        m.Processes = processes
                        m.ProcessSource = h.activeManager.GetName()
                        collected = true
                } else if err != nil {
                        log.Printf("Hub: %s failed (%v), trying other managers...", h.activeManager.GetName(), err)
                }
        }

        // Step 2: If active manager failed, try all managers in order
        if !collected {
                for _, mgr := range h.managers {
                        // Skip the manager we already tried
                        if h.activeManager != nil && mgr.GetName() == h.activeManager.GetName() {
                                continue
                        }

                        if !mgr.IsAvailable() {
                                continue
                        }

                        processes, err := mgr.GetProcesses()
                        if err == nil && len(processes) > 0 {
                                m.Processes = processes
                                m.ProcessSource = mgr.GetName()
                                h.activeManager = mgr
                                collected = true
                                log.Printf("Hub: switched to %s", mgr.GetName())
                                break
                        } else if err != nil {
                                log.Printf("Hub: %s GetProcesses failed: %v", mgr.GetName(), err)
                        }
                }
        }

        // Step 3: If still nothing, try IsAvailable() on all managers for next cycle
        if !collected {
                for _, mgr := range h.managers {
                        if mgr.IsAvailable() {
                                // Manager exists but returned empty list — keep it as active
                                // so we try it first next time
                                h.activeManager = mgr
                                break
                        }
                }
        }

        // Sort processes by RAM descending (heaviest on top)
        // Processes without RAM data (RAMBytes == 0) sink to the bottom.
        sort.SliceStable(m.Processes, func(i, j int) bool {
                return m.Processes[i].RAMBytes > m.Processes[j].RAMBytes
        })

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

// GetActiveManagerName returns the name of the currently active process manager.
// Returns empty string if none is active yet.
func (h *Hub) GetActiveManagerName() string {
        h.mu.RLock()
        defer h.mu.RUnlock()
        if h.activeManager != nil {
                return h.activeManager.GetName()
        }
        return ""
}

// GetManagerStatuses returns availability info for all registered managers.
// Used by main.go for startup logging.
func (h *Hub) GetManagerStatuses() []ManagerStatus {
        statuses := make([]ManagerStatus, 0, len(h.managers))
        for _, mgr := range h.managers {
                statuses = append(statuses, ManagerStatus{
                        Name:      mgr.GetName(),
                        Available: mgr.IsAvailable(),
                })
        }
        return statuses
}

// ManagerStatus describes the availability of a process manager.
type ManagerStatus struct {
        Name      string
        Available bool
}

// Stop terminates the background collection goroutine.
func (h *Hub) Stop() {
        close(h.stopCh)
        log.Println("Hub: collection goroutine stopped")
}
