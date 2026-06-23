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
)

var syncRe = regexp.MustCompile(`(recovery|resync|reshape|check)\s*=\s*([\d.]+)%`)

type Member struct {
	Slot   int    `json:"slot"`
	Device string `json:"device"`
	State  string `json:"state"`
}

type ArrayDetail struct {
	Array
	MDState         string   `json:"md_state"`
	ActiveDevices   int      `json:"active_devices"`
	WorkingDevices  int      `json:"working_devices"`
	FailedDevices   int      `json:"failed_devices"`
	SpareDevices    int      `json:"spare_devices"`
	TotalDevices    int      `json:"total_devices"`
	SlotMap         string   `json:"slot_map,omitempty"`
	SyncPercent     float64  `json:"sync_percent,omitempty"`
	SyncAction      string   `json:"sync_action,omitempty"`
	RebuildStatus   string   `json:"rebuild_status,omitempty"`
	Members         []Member `json:"members"`
	DegradedReasons []string `json:"degraded_reasons"`
	UUID            string   `json:"uuid,omitempty"`
}

func Detail(name string) (*ArrayDetail, error) {
	path := name
	if !strings.HasPrefix(path, "/dev/") {
		path = "/dev/" + path
	}
	base := filepath.Base(path)

	arr, err := readArrayPtr(base)
	if err != nil {
		return nil, err
	}
	mdMap, _ := parseMdstat()
	if m, ok := mdMap[base]; ok {
		if m.State != "" {
			arr.State = m.State
		}
		if m.Level != "" {
			arr.Level = m.Level
		}
		arr.Degraded = m.Degraded
	}
	arr.Level = normalizeLevel(arr.Level)

	detail := &ArrayDetail{Array: *arr}
	parseMdstatExtras(base, detail)

	out, err := exec.Command("mdadm", "--detail", path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mdadm --detail: %s: %w", strings.TrimSpace(string(out)), err)
	}
	parseMdadmDetail(string(out), detail)
	detail.DegradedReasons = buildDegradedReasons(detail)
	return detail, nil
}

func parseMdstatExtras(name string, d *ArrayDetail) {
	f, err := os.Open("/proc/mdstat")
	if err != nil {
		return
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, name+" ") && !strings.HasPrefix(line, name+":") {
			continue
		}
		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if m := mdStatusRe.FindAllStringSubmatch(next, -1); len(m) > 0 {
				d.SlotMap = m[len(m)-1][1]
			}
		}
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			syncLine := strings.TrimSpace(lines[j])
			if m := syncRe.FindStringSubmatch(syncLine); len(m) == 3 {
				d.SyncAction = m[1]
				d.SyncPercent, _ = strconv.ParseFloat(m[2], 64)
			}
		}
		break
	}
}

func parseMdadmDetail(text string, d *ArrayDetail) {
	var inTable bool
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "State :") {
			d.MDState = strings.TrimSpace(strings.TrimPrefix(line, "State :"))
			d.Degraded = strings.Contains(strings.ToLower(d.MDState), "degraded")
		}
		if strings.HasPrefix(line, "Active Devices :") {
			d.ActiveDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Working Devices :") {
			d.WorkingDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Failed Devices :") {
			d.FailedDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Spare Devices :") {
			d.SpareDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Total Devices :") {
			d.TotalDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Raid Devices :") {
			d.RaidDevices = parseIntSuffix(line)
		}
		if strings.HasPrefix(line, "Rebuild Status :") {
			d.RebuildStatus = strings.TrimSpace(strings.TrimPrefix(line, "Rebuild Status :"))
		}
		if strings.HasPrefix(line, "UUID :") {
			d.UUID = strings.TrimSpace(strings.TrimPrefix(line, "UUID :"))
		}
		if strings.HasPrefix(line, "Number") && strings.Contains(line, "RaidDevice") {
			inTable = true
			continue
		}
		if inTable {
			m := parseMemberLine(line)
			if m != nil {
				d.Members = append(d.Members, *m)
			}
		}
	}
}

func parseIntSuffix(line string) int {
	i := strings.LastIndex(line, ":")
	if i < 0 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(line[i+1:]))
	return n
}

func parseMemberLine(line string) *Member {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return nil
	}
	slot, err := strconv.Atoi(fields[3])
	if err != nil {
		return nil
	}
	m := Member{Slot: slot}
	if fields[0] == "-" {
		m.Device = ""
		m.State = "removed"
		return &m
	}
	// Number Major Minor RaidDevice State... /dev/sdX
	if len(fields) >= 6 && strings.HasPrefix(fields[len(fields)-1], "/dev/") {
		m.Device = fields[len(fields)-1]
		m.State = strings.Join(fields[4:len(fields)-1], " ")
	} else if len(fields) >= 5 {
		m.State = strings.Join(fields[4:], " ")
	}
	return &m
}

func buildDegradedReasons(d *ArrayDetail) []string {
	var reasons []string
	for _, m := range d.Members {
		switch {
		case m.State == "removed" || m.Device == "":
			reasons = append(reasons, fmt.Sprintf("Emplacement %d : disque manquant", m.Slot))
		case strings.Contains(m.State, "faulty") || strings.Contains(m.State, "failed"):
			reasons = append(reasons, fmt.Sprintf("%s (slot %d) : en échec — %s", devLabel(m.Device), m.Slot, m.State))
		case strings.Contains(m.State, "spare"):
			if strings.Contains(m.State, "rebuilding") {
				reasons = append(reasons, fmt.Sprintf("%s : spare en reconstruction (slot %d)", devLabel(m.Device), m.Slot))
			}
		}
	}
	if d.FailedDevices > 0 {
		reasons = append(reasons, fmt.Sprintf("%d disque(s) signalé(s) en échec par mdadm", d.FailedDevices))
	}
	if d.RaidDevices > 0 && d.ActiveDevices < d.RaidDevices {
		reasons = append(reasons, fmt.Sprintf(
			"Seulement %d/%d disques actifs (niveau RAID %s)",
			d.ActiveDevices, d.RaidDevices, d.Level,
		))
	}
	if d.SlotMap != "" && strings.Contains(d.SlotMap, "_") {
		missing := strings.Count(d.SlotMap, "_")
		reasons = append(reasons, fmt.Sprintf("Carte d'état mdstat [%s] : %d emplacement(s) absent(s)", d.SlotMap, missing))
	}
	if d.SyncAction != "" && d.SyncPercent > 0 && d.SyncPercent < 100 {
		reasons = append(reasons, fmt.Sprintf("%s en cours : %.1f%%", d.SyncAction, d.SyncPercent))
	}
	if len(reasons) == 0 && d.Degraded {
		reasons = append(reasons, "Array marqué dégradé par le noyau (vérifiez mdadm --detail)")
	}
	return reasons
}

func devLabel(path string) string {
	if path == "" {
		return "—"
	}
	return path
}
