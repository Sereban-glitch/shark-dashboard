# 🦈 Shark Dashboard (v2.0 Universal)

**An ultra-lightweight, zero-config, zero-dependency, edge-native monitoring dashboard built with Go, HTMX, and SSE.**

Designed specifically for extreme environments like **Termux / PRoot on Android**, Raspberry Pi, and low-resource Edge servers, but **scales perfectly to Cloud environments (GCP/AWS)**. It consumes **< 10MB RAM**, requires ~0.4% CPU, and gracefully handles restricted kernel syscalls where heavy solutions crash.

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Architecture](https://img.shields.io/badge/Arch-ARM64%20%7C%20AMD64-blue)
![License](https://img.shields.io/badge/License-MIT-green)

---

## 📸 Screenshots (v2.0 UI)

<details>
<summary>💻 Desktop View (Smart RAM Sorting & Systemd Integration)</summary>
<br>
<div align="center">
  <img src="assets/extra_1.jpg" width="100%" alt="Desktop View V2 Part 1" />
  <img src="assets/extra_2.jpg" width="100%" alt="Desktop View V2 Part 2" />
  <br>
  <i>Notice the amber RAM capsules and automatic sorting (heaviest processes on top).</i>
</div>
</details>

<details>
<summary>📱 Mobile View (Termux Edge Node)</summary>
<br>
<div align="center">
  <img src="assets/top_mobile.jpg" width="48%" alt="Mobile View Top" />
  <img src="assets/bottom_mobile.jpg" width="48%" alt="Mobile View Bottom" />
</div>
</details>

---

## 🚀 What's New in v2.0 (Cloud Universal Edition)?

Shark Dashboard is now a true "Swiss Army Knife" for any server:
- **Universal Process Managers:** Auto-detects and monitors **Systemd**, **PM2**, and **Supervisord**. Just drop the binary on any server, and it finds your projects automatically.
- **Smart RAM Sorting:** Automatically extracts RAM consumption (from cgroups or PM2 metrics) and sorts processes so the heaviest memory consumers are always at the top.
- **Zero Configuration — Just Works:** Detects battery status (charging/discharging), memory, swap, CPU temperature — no config files, no setup. Install and go.
- **Defensive Programming:** Graceful fallbacks for restricted Android environments. If `/proc/stat` is spoofed or restricted, it falls back to `/proc/loadavg`.

## 🛠️ Tech Stack
- **Backend:** Go (Golang), `os/exec` (Zero Dependencies)
- **Frontend:** HTML5, HTMX (No heavy JS frameworks)
- **Styling:** Custom CSS (GitHub Dark Theme, Glassmorphism, Amber badges)

## 📦 Installation & Build

For Cloud Servers (amd64) or Android PRoot (arm64):

```bash
# Clone the repository
git clone https://github.com/Sereban-glitch/shark-dashboard.git
cd shark-dashboard

# Build
go build -ldflags="-s -w" -o shark-dashboard main.go

# Run the dashboard
export GOGC=200 # Recommended to save battery on mobile nodes
./shark-dashboard -port 8081
```

## ⚙️ Configuration (Flags)
| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8081` | HTTP server port |
| `-addr` | `0.0.0.0` | Binding address |
| `-interval` | `3` | Metrics collection interval (seconds) |

## 🌍 Real-World Validation
- **Mobile Nodes:** Proven on a **Redmi Note 9 (Android 11, MediaTek Helio G85)** running Debian via PRoot in Termux.
- **Cloud Nodes:** Proven on **Google Cloud Platform (GCP)** Debian instances monitoring 30+ Systemd and PM2 microservices in real-time.

## 🤝 Contributing
Pull requests are welcome! If you're building mobile-server infrastructure or IoT edge nodes, feel free to contribute.
