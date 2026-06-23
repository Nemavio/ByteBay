package disks

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Disk struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	SizeBytes  uint64 `json:"size_bytes"`
	Model      string `json:"model,omitempty"`
	Serial     string `json:"serial,omitempty"`
	Rotational bool   `json:"rotational"`
	Mountpoint string `json:"mountpoint,omitempty"`
	FSType     string `json:"fs_type,omitempty"`
	InRaid     bool   `json:"in_raid"`
	RaidMember string `json:"raid_member,omitempty"`
}

func List() ([]Disk, error) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, fmt.Errorf("read /sys/block: %w", err)
	}

	mounts := parseMounts()
	var out []Disk
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "dm-") {
			continue
		}
		base := filepath.Join("/sys/block", name)
		size, _ := readUint(base + "/size")
		rot, _ := readUint(base + "/queue/rotational")
		d := Disk{
			Name:       name,
			Path:       "/dev/" + name,
			SizeBytes:  size * 512,
			Rotational: rot == 1,
		}
		d.RaidMember = raidHolder(name)
		d.InRaid = d.RaidMember != ""
		if m, ok := mounts["/dev/"+name]; ok {
			d.Mountpoint = m.mount
			d.FSType = m.fstype
		}
		d.Model = readTrim(base + "/device/model")
		d.Serial = readTrim(base + "/device/serial")
		out = append(out, d)
	}
	return out, nil
}

type mountInfo struct {
	mount, fstype string
}

func parseMounts() map[string]mountInfo {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	m := make(map[string]mountInfo)
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 3 {
			continue
		}
		m[fields[0]] = mountInfo{mount: fields[1], fstype: fields[2]}
	}
	return m
}

func raidHolder(name string) string {
	holders, err := os.ReadDir(filepath.Join("/sys/block", name, "holders"))
	if err != nil {
		return ""
	}
	for _, h := range holders {
		if strings.HasPrefix(h.Name(), "md") {
			return "/dev/" + h.Name()
		}
	}
	return ""
}

func readUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var n uint64
	_, err = fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &n)
	return n, err
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
