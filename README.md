# 🦈 Shark Dashboard

**An ultra-lightweight, zero-dependency, edge-native monitoring dashboard built with Go, HTMX, and SSE.**

Designed specifically for extreme environments like **Termux / PRoot on Android**, Raspberry Pi, and low-resource Edge servers. It consumes **< 10MB RAM**, requires ~0.4% CPU, and gracefully handles restricted kernel syscalls where heavy solutions like Netdata or Prometheus crash.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Architecture](https://img.shields.io/badge/Arch-ARM64%20%7C%20AMD64-blue)
![License](https://img.shields.io/badge/License-MIT-green)

---

## 📸 Screenshots

<details>
<summary>📱 Mobile View (Termux Edge Node)</summary>
<br>
<div align="center">
  <img src="assets/top_mobile.jpg" width="48%" alt="Mobile View Top" />
  <img src="assets/bottom_mobile.jpg" width="48%" alt="Mobile View Bottom" />
</div>
</details>

<details>
<summary>💻 Desktop View (Monitoring Hub)</summary>
<br>
<img src="assets/desktop_view.jpg" width="100%" alt="Desktop Full View" />
</details>

---

## 🚀 Why Shark Dashboard?

Traditional monitoring tools are too heavy for mobile-as-a-server setups (like Android running Debian via PRoot). They fork processes, rely on `systemd` (which doesn't exist in PRoot), and crash when Android's kernel restricts access to `/proc/net/dev`.

**Shark Dashboard solves this:**
- **Zero Dependencies:** Compiles to a single static binary with embedded HTML (`go:embed`). No external assets.
- **Defensive Programming:** Graceful fallbacks for restricted Android environments. If `/proc/stat` is spoofed or restricted, it falls back to `/proc/loadavg`. If network stats are blocked, the UI degrades elegantly without crashing.
- **Battery & Thermal Friendly:** Designed for mobile chips. Built with `GOGC=200` to minimize Garbage Collection spikes. Collects data in a background goroutine (`sync.RWMutex`) to prevent data races and reduce I/O.
- **Real-Time SSE:** Uses Server-Sent Events (SSE) instead of WebSockets for lower overhead and auto-reconnection.
- **Supervisord Native:** Built-in XML-RPC client connects directly to `supervisord` TCP sockets to manage background processes.

## 🛠️ Tech Stack
- **Backend:** Go (Golang), `golang.org/x/sys/unix`
- **Frontend:** HTML5, HTMX (No heavy JS frameworks)
- **Styling:** Custom CSS (GitHub Dark Theme, Glassmorphism)

## 📦 Installation & Build

For ARM64 environments (e.g., Termux/PRoot on Android):

```bash
# Clone the repository
git clone https://github.com/Sereban-glitch/shark-dashboard.git
cd shark-dashboard

# Build with limited parallelism to prevent device overheating
go build -p 2 -ldflags="-s -w" -o shark-dashboard-arm64 main.go

# Run the dashboard
export GOGC=200 # Recommended to save battery
./shark-dashboard-arm64 -port 8081
```

## ⚙️ Configuration (Flags)

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8081` | HTTP server port |
| `-addr` | `0.0.0.0` | Binding address |
| `-interval` | `3` | Metrics collection interval (seconds) |
| `-supervisor` | `http://127.0.0.1:9001` | Supervisord XML-RPC endpoint |

## 🤝 Contributing

Pull requests are welcome! If you're building mobile-server infrastructure or IoT edge nodes, feel free to contribute.
