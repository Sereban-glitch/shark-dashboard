package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"shark-dashboard/internal/collector"
	"shark-dashboard/internal/handler"
)

//go:embed web/templates/index.html
var templateFS embed.FS

func main() {
	// GC tuning — allow more RAM (up to 30-40 MB) to reduce GC frequency.
	// Default GOGC=100 means GC runs when heap doubles. With 200, it runs when
	// heap triples. Less CPU spent on GC → less heat on the phone.
	debug.SetGCPercent(200)

	port := flag.Int("port", 8081, "HTTP server port")
	addr := flag.String("addr", "0.0.0.0", "bind address")
	svAddr := flag.String("supervisor", "http://127.0.0.1:9001", "Supervisord XML-RPC address")
	interval := flag.Int("interval", 3, "metrics collection interval in seconds")
	flag.Parse()

	// Read embedded template
	tmplBytes, err := templateFS.ReadFile("web/templates/index.html")
	if err != nil {
		log.Fatalf("Failed to read template: %v", err)
	}

	// ===== Initialize Process Managers (ordered by priority) =====
	// Hub will auto-detect the first available manager.
	// Order: Supervisord → systemd → PM2
	managers := []collector.ProcessManager{
		collector.NewSupervisordAdapter(*svAddr),
		collector.NewSystemdCollector(),
		collector.NewPM2Collector(),
	}

	// Log availability at startup
	log.Println("Process manager auto-detection:")
	for _, mgr := range managers {
		avail := mgr.IsAvailable()
		status := "UNAVAILABLE"
		if avail {
			status = "AVAILABLE"
		}
		log.Printf("  - %-12s: %s", mgr.GetName(), status)
	}

	// Initialize collector hub — starts background collection goroutine
	collectInterval := time.Duration(*interval) * time.Second
	hub := collector.NewHub(managers, collectInterval)

	// Log which manager was auto-detected as active
	activeName := hub.GetActiveManagerName()
	if activeName != "" {
		log.Printf("Active process manager: %s", activeName)
	} else {
		log.Printf("WARNING: No process manager detected — process monitoring will show empty list")
	}

	// Initialize handlers
	pageHandler, err := handler.NewPageHandler(string(tmplBytes))
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	sseHandler := handler.NewSSEHandler(hub)
	metricsHandler := handler.NewMetricsHandler(hub)

	// Setup routes
	mux := http.NewServeMux()
	mux.Handle("/", pageHandler)
	mux.Handle("/api/events", sseHandler)
	mux.Handle("/api/metrics", metricsHandler)

	// Serve
	bindAddr := fmt.Sprintf("%s:%d", *addr, *port)
	log.Printf("Shark Dashboard starting on http://%s", bindAddr)
	log.Printf("   Metrics interval: %ds | GC percent: 200", *interval)

	// Build manager summary string for log
	var mgrSummary []string
	for _, mgr := range managers {
		mgrSummary = append(mgrSummary, fmt.Sprintf("%s=%v", mgr.GetName(), mgr.IsAvailable()))
	}
	log.Printf("   Process managers: [%s] → active: %s", strings.Join(mgrSummary, ", "), activeName)

	server := &http.Server{
		Addr:         bindAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ==================== Graceful Shutdown ====================
	// Listen for SIGINT (Ctrl+C) and SIGTERM (supervisorctl stop)
	// This ensures collectLoop and SSE goroutines are stopped cleanly,
	// and in-flight HTTP requests complete before exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down gracefully...", sig)

		// 1. Stop collecting metrics
		hub.Stop()

		// 2. Stop SSE broadcaster
		sseHandler.Stop()

		// 3. Shutdown HTTP server with 5-second timeout for in-flight requests
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		log.Println("Shark Dashboard stopped.")
	}()

	// ListenAndServe blocks until Shutdown() is called
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
