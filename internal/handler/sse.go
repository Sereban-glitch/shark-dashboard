package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"shark-dashboard/internal/collector"
)

// SSEHandler handles Server-Sent Events connections.
// It reads from Hub.GetMetrics() (thread-safe RLock) and broadcasts
// to all connected SSE clients whenever new data is available.
type SSEHandler struct {
	hub     *collector.Hub
	mu      sync.Mutex
	clients map[chan []byte]struct{}
	stopCh  chan struct{}
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(hub *collector.Hub) *SSEHandler {
	h := &SSEHandler{
		hub:     hub,
		clients: make(map[chan []byte]struct{}),
		stopCh:  make(chan struct{}),
	}
	go h.broadcast()
	return h
}

// ServeHTTP handles GET /api/events requests.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Create client channel
	ch := make(chan []byte, 10)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	// Remove client on disconnect
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	// Send initial snapshot immediately (thread-safe read)
	metrics := h.hub.GetMetrics()
	data, _ := json.Marshal(metrics)
	fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Stream updates
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

// broadcast reads the latest metrics snapshot (via RLock) every 3 seconds
// and sends it to all connected SSE clients.
func (h *SSEHandler) broadcast() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Thread-safe read — uses RLock inside Hub
			metrics := h.hub.GetMetrics()
			data, err := json.Marshal(metrics)
			if err != nil {
				log.Printf("error marshaling metrics: %v", err)
				continue
			}

			h.mu.Lock()
			for ch := range h.clients {
				select {
				case ch <- data:
				default:
					// Client is slow, skip this update
				}
			}
			h.mu.Unlock()

		case <-h.stopCh:
			return
		}
	}
}

// Stop terminates the SSE broadcast goroutine.
func (h *SSEHandler) Stop() {
	close(h.stopCh)
	log.Println("SSE: broadcast goroutine stopped")
}

// MetricsHandler handles GET /api/metrics for one-shot JSON responses.
type MetricsHandler struct {
	hub *collector.Hub
}

// NewMetricsHandler creates a new metrics handler.
func NewMetricsHandler(hub *collector.Hub) *MetricsHandler {
	return &MetricsHandler{hub: hub}
}

// ServeHTTP handles GET /api/metrics requests.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Thread-safe read — uses RLock inside Hub
	metrics := h.hub.GetMetrics()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(metrics)
}
