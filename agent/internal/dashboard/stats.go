package dashboard

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/bytebay/bytebay/agent/internal/mounts"
	"github.com/bytebay/bytebay/agent/internal/network"
)

type Snapshot struct {
	CPU        CPUStats         `json:"cpu"`
	Memory     MemoryStats      `json:"memory"`
	Interfaces []IfaceStats     `json:"interfaces"`
	Mounts     []MountUsage     `json:"mounts"`
}

type CPUStats struct {
	Percent float64   `json:"percent"`
	Cores   int       `json:"cores"`
	Load    []float64 `json:"load"`
}

type MemoryStats struct {
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	Percent    float64 `json:"percent"`
}

type IfaceStats struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	MAC     string   `json:"mac,omitempty"`
	IPv4    []string `json:"ipv4"`
	IPv6    []string `json:"ipv6"`
}

type MountUsage struct {
	Name          string  `json:"name"`
	HostPath      string  `json:"host_path"`
	ContainerPath string  `json:"container_path"`
	Mounted       bool    `json:"mounted"`
	TotalBytes    int64   `json:"total_bytes"`
	UsedBytes     int64   `json:"used_bytes"`
	Percent       float64 `json:"percent"`
}

var (
	cpuMu        sync.Mutex
	lastCPUIdle  uint64
	lastCPUTotal uint64
)

func Collect() (*Snapshot, error) {
	ifaces, err := networkInterfaces()
	if err != nil {
		return nil, err
	}
	mountUsage, err := mountStats()
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		CPU:        cpuStats(),
		Memory:     memoryStats(),
		Interfaces: ifaces,
		Mounts:     mountUsage,
	}, nil
}

func cpuStats() CPUStats {
	cores := runtime.NumCPU()
	load := parseLoadAvg()
	return CPUStats{
		Percent: cpuPercent(),
		Cores:   cores,
		Load:    load,
	}
}

func cpuPercent() float64 {
	idle, total := readCPUStat()
	cpuMu.Lock()
	defer cpuMu.Unlock()
	if lastCPUTotal == 0 {
		lastCPUIdle, lastCPUTotal = idle, total
		return 0
	}
	idleDelta := idle - lastCPUIdle
	totalDelta := total - lastCPUTotal
	lastCPUIdle, lastCPUTotal = idle, total
	if totalDelta == 0 {
		return 0
	}
	used := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func readCPUStat() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0
	}
	var vals []uint64
	for _, field := range fields[1:] {
		n, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0
		}
		vals = append(vals, n)
	}
	for _, v := range vals {
		total += v
	}
	if len(vals) > 3 {
		idle = vals[3]
		if len(vals) > 4 {
			idle += vals[4]
		}
	}
	return idle, total
}

func parseLoadAvg() []float64 {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return []float64{0, 0, 0}
	}
	parts := strings.Fields(string(b))
	out := make([]float64, 0, 3)
	for i := 0; i < 3 && i < len(parts); i++ {
		v, _ := strconv.ParseFloat(parts[i], 64)
		out = append(out, v)
	}
	for len(out) < 3 {
		out = append(out, 0)
	}
	return out
}

func memoryStats() MemoryStats {
	vals := parseMemInfo()
	total := vals["MemTotal"]
	avail := vals["MemAvailable"]
	if avail == 0 {
		avail = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	used := total - avail
	if total <= 0 {
		return MemoryStats{}
	}
	pct := float64(used) / float64(total) * 100
	if pct < 0 {
		pct = 0
	}
	return MemoryStats{
		TotalBytes: total,
		UsedBytes:  used,
		Percent:    pct,
	}
}

func parseMemInfo() map[string]int64 {
	out := make(map[string]int64)
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		n, _ := strconv.ParseInt(parts[1], 10, 64)
		out[key] = n * 1024
	}
	return out
}

func networkInterfaces() ([]IfaceStats, error) {
	st, err := network.GetStatus()
	if err != nil {
		return nil, err
	}
	live := map[string]network.Conn{}
	for _, c := range st.Connections {
		live[c.Name] = c
	}
	var out []IfaceStats
	for _, iface := range st.Interfaces {
		row := IfaceStats{
			Name:  iface.Name,
			State: iface.State,
			MAC:   iface.MAC,
		}
		if conn, ok := live[iface.Name]; ok {
			if conn.OperState != "" {
				row.State = conn.OperState
			}
			for _, addr := range conn.Addresses {
				if strings.Contains(addr, ":") {
					row.IPv6 = append(row.IPv6, addr)
				} else {
					row.IPv4 = append(row.IPv4, addr)
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func mountStats() ([]MountUsage, error) {
	list, err := mounts.List()
	if err != nil {
		return nil, err
	}
	out := make([]MountUsage, 0, len(list))
	for _, m := range list {
		total, used, pct := diskUsage(m.HostPath)
		out = append(out, MountUsage{
			Name:          m.Name,
			HostPath:      m.HostPath,
			ContainerPath: m.ContainerPath,
			Mounted:       m.Mounted,
			TotalBytes:    total,
			UsedBytes:     used,
			Percent:       pct,
		})
	}
	return out, nil
}

func diskUsage(path string) (total, used int64, percent float64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, 0
	}
	bsize := int64(st.Frsize)
	if bsize <= 0 {
		bsize = int64(st.Bsize)
	}
	total = int64(st.Blocks) * bsize
	used = int64(st.Blocks-st.Bfree) * bsize
	if total <= 0 {
		return 0, 0, 0
	}
	if used < 0 {
		used = 0
	}
	percent = float64(used) / float64(total) * 100
	return total, used, percent
}
