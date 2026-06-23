package raid

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bytebay/bytebay/agent/internal/disks"
)

var (
	mdCountRe  = regexp.MustCompile(`\[(\d+)/(\d+)\]`)
	mdStatusRe = regexp.MustCompile(`\[([U_]+)\]`)
)

type Array struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Level       string   `json:"level"`
	State       string   `json:"state"`
	SizeBytes   uint64   `json:"size_bytes"`
	Devices     []string `json:"devices"`
	RaidDevices int      `json:"raid_devices,omitempty"`
	Degraded    bool     `json:"degraded,omitempty"`
}

type CreateRequest struct {
	Level       string   `json:"level"`
	Devices     []string `json:"devices"`
	RaidDevices int      `json:"raid_devices,omitempty"`
	Name        string   `json:"name,omitempty"`
}

type AddRequest struct {
	Device string `json:"device"`
}

func List() ([]Array, error) {
	mdMap, _ := parseMdstat()

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}
	var out []Array
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "md") {
			continue
		}
		arr, err := readArray(e.Name())
		if err != nil {
			continue
		}
		if m, ok := mdMap[e.Name()]; ok {
			if m.State != "" {
				arr.State = m.State
			}
			if m.Level != "" {
				arr.Level = m.Level
			}
			arr.Degraded = m.Degraded || strings.Contains(strings.ToLower(m.State), "degraded")
		}
		arr.Level = normalizeLevel(arr.Level)
		out = append(out, arr)
	}
	return out, nil
}

func readArray(name string) (Array, error) {
	base := filepath.Join("/sys/block", name, "md")
	level, _ := readSysString(base + "/level")
	state, _ := readSysString(base + "/array_state")
	raidDev, _ := readSysString(base + "/raid_disks")
	sizeSectors, _ := readSysString(filepath.Join("/sys/block", name, "size"))

	var devs []string
	slavesDir := filepath.Join("/sys/block", name, "slaves")
	if entries, err := os.ReadDir(slavesDir); err == nil {
		for _, s := range entries {
			devs = append(devs, "/dev/"+s.Name())
		}
	}

	size, _ := strconv.ParseUint(strings.TrimSpace(sizeSectors), 10, 64)
	rd, _ := strconv.Atoi(strings.TrimSpace(raidDev))
	return Array{
		Name:        name,
		Path:        "/dev/" + name,
		Level:       normalizeLevel(strings.TrimSpace(level)),
		State:       strings.TrimSpace(state),
		SizeBytes:   size * 512,
		Devices:     devs,
		RaidDevices: rd,
	}, nil
}

func normalizeLevel(level string) string {
	level = strings.TrimSpace(level)
	if level == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(level), "raid")
}

func Create(req CreateRequest) (*Array, error) {
	level := strings.TrimPrefix(req.Level, "raid")
	if level == "" || len(req.Devices) < 1 {
		return nil, fmt.Errorf("level and at least 1 device required")
	}

	raidDevices := req.RaidDevices
	if raidDevices == 0 {
		raidDevices = len(req.Devices)
	}
	if raidDevices < len(req.Devices) {
		return nil, fmt.Errorf("raid_devices must be >= number of devices")
	}

	var present []string
	for i, d := range req.Devices {
		d = strings.TrimSpace(d)
		if d == "" || d == "missing" {
			continue
		}
		if !strings.HasPrefix(d, "/dev/") {
			d = "/dev/" + strings.TrimPrefix(d, "/dev/")
		}
		req.Devices[i] = d
		present = append(present, d)
	}

	if err := validateDevices(present); err != nil {
		return nil, err
	}

	md := req.Name
	if md == "" {
		var err error
		md, err = nextMD()
		if err != nil {
			return nil, err
		}
	}
	if !strings.HasPrefix(md, "/dev/") {
		md = "/dev/" + md
	}

	args := []string{
		"--create", md,
		"--level=" + level,
		"--raid-devices=" + strconv.Itoa(raidDevices),
	}
	for _, d := range req.Devices {
		if d == "" || d == "missing" {
			args = append(args, "missing")
		} else {
			args = append(args, d)
		}
	}
	for i := len(req.Devices); i < raidDevices; i++ {
		args = append(args, "missing")
	}

	cmd := exec.Command("mdadm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mdadm: %s: %w", strings.TrimSpace(string(out)), err)
	}

	name := filepath.Base(md)
	arr, err := readArrayPtr(name)
	if err != nil {
		return nil, err
	}
	arr.Degraded = raidDevices > len(present)
	return arr, nil
}

func Add(name, device string) (*Array, error) {
	path := name
	if !strings.HasPrefix(path, "/dev/") {
		path = "/dev/" + path
	}
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + strings.TrimPrefix(device, "/dev/")
	}
	if err := validateDevices([]string{device}); err != nil {
		return nil, err
	}
	out, err := exec.Command("mdadm", "--add", path, device).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mdadm --add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return readArrayPtr(filepath.Base(path))
}

func Stop(name string) error {
	path := name
	if !strings.HasPrefix(path, "/dev/") {
		path = "/dev/" + path
	}
	out, err := exec.Command("mdadm", "--stop", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mdadm --stop: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func validateDevices(devs []string) error {
	all, err := disks.List()
	if err != nil {
		return err
	}
	byPath := make(map[string]disks.Disk)
	for _, d := range all {
		byPath[d.Path] = d
	}
	for _, dev := range devs {
		if dev == "missing" || dev == "" {
			continue
		}
		d, ok := byPath[dev]
		if !ok {
			return fmt.Errorf("unknown device: %s", dev)
		}
		if d.Mountpoint != "" {
			return fmt.Errorf("%s is mounted at %s", dev, d.Mountpoint)
		}
		if d.InRaid {
			return fmt.Errorf("%s is already in a RAID array", dev)
		}
	}
	return nil
}

func nextMD() (string, error) {
	for i := 0; i < 128; i++ {
		if _, err := os.Stat("/sys/block/md" + strconv.Itoa(i)); os.IsNotExist(err) {
			return fmt.Sprintf("/dev/md%d", i), nil
		}
	}
	return "", fmt.Errorf("no free md device")
}

func readArrayPtr(name string) (*Array, error) {
	a, err := readArray(name)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func readSysString(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseMdstat() (map[string]Array, error) {
	f, err := os.Open("/proc/mdstat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]Array)
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "md") || !strings.Contains(line, ":") {
			continue
		}
		colon := strings.Index(line, ":")
		name := strings.TrimSpace(line[:colon])
		fields := strings.Fields(line[colon+1:])
		if len(fields) < 1 {
			continue
		}

		a := Array{Name: name, Path: "/dev/" + name}
		for _, p := range fields {
			if strings.HasPrefix(p, "raid") {
				a.Level = normalizeLevel(p)
			} else if p == "active" || p == "inactive" || strings.HasPrefix(p, "resync") || strings.HasPrefix(p, "recovery") {
				if a.State == "" {
					a.State = p
				}
			}
		}

		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if !strings.HasPrefix(next, "md") && next != "" && !strings.HasPrefix(next, "unused") {
				if m := mdCountRe.FindStringSubmatch(next); len(m) == 3 {
					total, _ := strconv.Atoi(m[1])
					active, _ := strconv.Atoi(m[2])
					if a.State == "" {
						a.State = "active"
					}
					a.State = fmt.Sprintf("%s (%d/%d)", a.State, active, total)
					if active < total {
						a.Degraded = true
					}
				}
				if m := mdStatusRe.FindAllStringSubmatch(next, -1); len(m) > 0 {
					slots := m[len(m)-1][1]
					missing := strings.Count(slots, "_")
					if missing > 0 {
						a.Degraded = true
					}
				}
				if strings.Contains(next, "recovery") || strings.Contains(next, "resync") {
					a.State = "resync"
				}
				i++
			}
		}
		if a.State == "" {
			a.State = "active"
		}
		out[name] = a
	}
	return out, nil
}
