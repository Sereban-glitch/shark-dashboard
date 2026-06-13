package collector

import (
        "bufio"
        "log"
        "os"
        "strings"

        "shark-dashboard/internal/model"

        "golang.org/x/sys/unix"
)

// DiskCollector uses unix.Statfs syscall to collect disk usage.
// No os/exec, no fork — pure syscall, battery-friendly.
type DiskCollector struct {
        mountPoints []string // cached mount points read once from /proc/mounts
}

// NewDiskCollector creates a new disk collector.
func NewDiskCollector() *DiskCollector {
        c := &DiskCollector{}
        c.mountPoints = c.readMountPoints()
        return c
}

// Collect returns disk usage information via Statfs syscall.
// Always returns a non-nil slice (empty [] if no disks), never null in JSON.
func (c *DiskCollector) Collect() []model.DiskInfo {
        // Panic recovery for PRoot environments
        defer func() {
                if r := recover(); r != nil {
                        log.Printf("DiskCollector: recovered from panic: %v", r)
                }
        }()

        // Re-read mount points each time in case of changes (USB, etc.)
        // This is cheap — just reading /proc/mounts
        mounts := c.readMountPoints()
        c.mountPoints = mounts

        disks := make([]model.DiskInfo, 0) // never nil → JSON encodes as [] not null
        seen := make(map[string]bool)

        for _, mp := range mounts {
                if seen[mp] {
                        continue
                }
                seen[mp] = true

                var stat unix.Statfs_t
                if err := unix.Statfs(mp, &stat); err != nil {
                        continue
                }

                // Skip zero-size or pseudo filesystems
                if stat.Blocks == 0 {
                        continue
                }

                // Skip known pseudo/virtual filesystems by magic number
                switch stat.Type {
                case 0x01021994: // TMPFS_MAGIC
                        continue
                case 0x00005846: // DEVFS_SUPER_MAGIC
                        continue
                case 0x61756673: // AUFS_MAGIC
                        continue
                case 0x73727279: // BDEV_MAGIC
                        continue
                case 0x62656572: // SYSFS_MAGIC
                        continue
                case 0x42494e4d: // BINFS_MAGIC
                        continue
                case 0x64626720: // DEBUGFS_MAGIC
                        continue
                case 0x7461636f: // OCFS2_SUPER_MAGIC
                        continue
                case 0x794c7630: // OVERLAYFS_MAGIC
                        continue
                case 0x5346414f: // SELINUX_MAGIC
                        continue
                case 0x3153464a: // JFFS2_SUPER_MAGIC
                        continue
                case 0x73717368: // SQUASHFS_MAGIC
                        continue
                case 0x01021997: // RAMFS_MAGIC
                        continue
                case 0x2bad1dea: // FUSE_CTL_SUPER_MAGIC
                        continue
                case 0x65735543: // CEPH_SUPER_MAGIC
                        continue
                }

                // Calculate sizes
                total := stat.Blocks * uint64(stat.Bsize)
                avail := stat.Bavail * uint64(stat.Bsize) // available to unprivileged users
                free := stat.Bfree * uint64(stat.Bsize)
                used := total - free

                if total == 0 {
                        continue
                }

                // Skip unrealistically large filesystems (>= 100 TB).
                // On Android/PRoot, FUSE and virtual bind mounts may report
                // absurd sizes (e.g. 64 PB). A real Android storage partition
                // is typically 32 GB - 2 TB. 100 TB is a safe upper bound.
                const maxRealisticSize uint64 = 100 * 1024 * 1024 * 1024 * 1024 // 100 TB
                if total >= maxRealisticSize {
                        continue
                }

                usedPct := float64(used) / float64(total) * 100.0

                disks = append(disks, model.DiskInfo{
                        Device:  mp,
                        Total:   total,
                        Used:    used,
                        Avail:   avail,
                        UsedPct: usedPct,
                })
        }

        return disks
}

// readMountPoints reads /proc/mounts and returns a list of real mount points.
func (c *DiskCollector) readMountPoints() []string {
        var mounts []string

        f, err := os.Open("/proc/mounts")
        if err != nil {
                // Fallback: at least check root
                return []string{"/"}
        }
        defer f.Close()

        scanner := bufio.NewScanner(f)
        for scanner.Scan() {
                fields := strings.Fields(scanner.Text())
                if len(fields) < 2 {
                        continue
                }

                fsType := fields[2]
                mountPoint := fields[1]

                // Skip pseudo/virtual filesystems
                switch fsType {
                case "sysfs", "proc", "devtmpfs", "devpts", "tmpfs",
                        "cgroup", "cgroup2", "pstore", "debugfs",
                        "securityfs", "fusectl", "configfs", "efivarfs",
                        "mqueue", "hugetlbfs", "tracefs", "bpf",
                        "overlay", "squashfs", "binfmt_misc":
                        continue
                }

                // Skip Android-specific virtual mount points
                if strings.HasPrefix(mountPoint, "/apex/") ||
                        strings.HasPrefix(mountPoint, "/sys/") ||
                        strings.HasPrefix(mountPoint, "/dev/") ||
                        strings.HasPrefix(mountPoint, "/proc/") ||
                        strings.HasPrefix(mountPoint, "/mnt/") ||
                        mountPoint == "/dev/logfs" {
                        continue
                }

                // Skip loop devices
                device := fields[0]
                if strings.HasPrefix(device, "/dev/loop") {
                        continue
                }

                mounts = append(mounts, mountPoint)
        }

        return mounts
}
