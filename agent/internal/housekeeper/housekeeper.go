package housekeeper

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytebay/bytebay/agent/internal/disks"
	"github.com/bytebay/bytebay/agent/internal/mounts"
	"github.com/bytebay/bytebay/agent/internal/raid"
)

type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityWarn   Severity = "warn"
	SeverityAction Severity = "action"
)

type Item struct {
	Kind     string                 `json:"kind"`
	ID       string                 `json:"id,omitempty"`
	Message  string                 `json:"message"`
	Progress int                    `json:"progress,omitempty"`
	Severity Severity               `json:"severity"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

type Report struct {
	Items     []Item `json:"items"`
	CheckedAt string `json:"checked_at"`
}

func Scan() (*Report, error) {
	report := &Report{CheckedAt: time.Now().UTC().Format(time.RFC3339)}

	for _, j := range raid.ListActiveCreateJobs() {
		report.Items = append(report.Items, Item{
			Kind:     "raid_job",
			ID:       j.ID,
			Message:  j.Message,
			Progress: j.Progress,
			Severity: SeverityInfo,
			Details: map[string]interface{}{
				"status": string(j.Status),
			},
		})
	}

	for _, j := range mounts.ListActiveJobs() {
		report.Items = append(report.Items, Item{
			Kind:     "mount_job",
			ID:       j.ID,
			Message:  j.Message,
			Progress: j.Progress,
			Severity: SeverityInfo,
			Details: map[string]interface{}{
				"status": string(j.Status),
			},
		})
	}

	for _, item := range detectFormatting() {
		report.Items = append(report.Items, item)
	}

	inactiveUUIDs := make(map[string]bool)
	for _, item := range detectInactiveRAID() {
		if u, ok := item.Details["uuid"].(string); ok {
			inactiveUUIDs[normalizeRAIDUUID(u)] = true
		}
		report.Items = append(report.Items, item)
	}

	for _, orphan := range detectOrphanRAID(inactiveUUIDs) {
		report.Items = append(report.Items, orphan)
	}

	for _, item := range detectRAIDResync() {
		report.Items = append(report.Items, item)
	}

	return report, nil
}

// RecoverRAID démarre un array inactif (déjà assemblé ou orphelin). force autorise le mode dégradé.
func RecoverRAID(uuid string, force bool) (*raid.Array, error) {
	norm := normalizeRAIDUUID(uuid)
	if path, ok := raid.FindInactiveByUUID(uuid); ok {
		return raid.StartInactive(path)
	}
	for _, in := range detectInactiveRAIDRaw() {
		if normalizeRAIDUUID(in.UUID) == norm {
			return raid.StartInactive(in.MDPath)
		}
	}
	orphans := detectOrphanRAIDRaw(nil)
	var target *orphanArray
	for i := range orphans {
		if normalizeRAIDUUID(orphans[i].UUID) == norm {
			target = &orphans[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("aucun array RAID inactif avec UUID %s", uuid)
	}
	if force {
		return raid.AssembleInactiveForce(target.MDPath, target.Devices)
	}
	return raid.AssembleInactive(target.MDPath, target.Devices)
}

type orphanArray struct {
	UUID       string
	MDPath     string
	Level      string
	Devices    []string
	TotalDisks int
}

func detectOrphanRAID(skipUUIDs map[string]bool) []Item {
	var items []Item
	for _, o := range detectOrphanRAIDRaw(skipUUIDs) {
		items = append(items, Item{
			Kind:     "raid_orphan",
			Message:  fmt.Sprintf("Métadonnées RAID sans volume actif (%s, %d disque(s))", raidLevelLabel(o.Level), len(o.Devices)),
			Severity: SeverityAction,
			Details: map[string]interface{}{
				"uuid":    o.UUID,
				"md_path": o.MDPath,
				"level":   o.Level,
				"devices": o.Devices,
			},
		})
	}
	return items
}

func detectInactiveRAID() []Item {
	var items []Item
	for _, in := range detectInactiveRAIDRaw() {
		msg := fmt.Sprintf("Array RAID %s inactif (%s, %d disque(s))", in.MDPath, raidLevelLabel(in.Level), len(in.Devices))
		if in.TotalDisks > 0 && len(in.Devices) < in.TotalDisks {
			msg = fmt.Sprintf("Array RAID %s inactif (%s, %d/%d disques — démarrage dégradé possible)", in.MDPath, raidLevelLabel(in.Level), len(in.Devices), in.TotalDisks)
		}
		items = append(items, Item{
			Kind:     "raid_inactive",
			Message:  msg,
			Severity: SeverityAction,
			Details: map[string]interface{}{
				"uuid":    in.UUID,
				"md_path": in.MDPath,
				"level":   in.Level,
				"devices": in.Devices,
			},
		})
	}
	return items
}

func detectInactiveRAIDRaw() []orphanArray {
	var out []orphanArray
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "md") {
			continue
		}
		name := e.Name()
		if isMDActive(name) {
			continue
		}
		uuidBytes, err := os.ReadFile(filepath.Join("/sys/block", name, "md/uuid"))
		if err != nil {
			continue
		}
		level, _ := os.ReadFile(filepath.Join("/sys/block", name, "md/level"))
		raidDevs, _ := os.ReadFile(filepath.Join("/sys/block", name, "md/raid_disks"))
		total := 0
		fmt.Sscanf(strings.TrimSpace(string(raidDevs)), "%d", &total)
		devs := mdMemberDevices(name)
		uuid := strings.TrimSpace(string(uuidBytes))
		uuidColons := sysfsUUIDToMdadm(uuid)
		out = append(out, orphanArray{
			UUID:       uuidColons,
			MDPath:     "/dev/" + name,
			Level:      strings.TrimSpace(string(level)),
			Devices:    devs,
			TotalDisks: total,
		})
	}
	return out
}

func isMDActive(name string) bool {
	state, err := os.ReadFile(filepath.Join("/sys/block", name, "md/array_state"))
	if err != nil {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(string(state)))
	return s != "inactive" && s != "clear" && s != ""
}

func mdMemberDevices(mdName string) []string {
	out, err := exec.Command("mdadm", "-D", "/dev/"+mdName).CombinedOutput()
	if err != nil {
		return nil
	}
	var devs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/dev/") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				devs = append(devs, fields[0])
			}
		}
	}
	return devs
}

func detectOrphanRAIDRaw(skipUUIDs map[string]bool) []orphanArray {
	active := activeRAIDUUIDs()
	inactive := make(map[string]bool)
	for _, in := range detectInactiveRAIDRaw() {
		inactive[normalizeRAIDUUID(in.UUID)] = true
	}
	seen := make(map[string]bool)
	var out []orphanArray

	cmd := exec.Command("mdadm", "--examine", "--scan")
	outBytes, err := cmd.Output()
	if err != nil {
		return nil
	}
	sc := bufio.NewScanner(strings.NewReader(string(outBytes)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "ARRAY ") {
			continue
		}
		o := parseExamineScanLine(line)
		if o.UUID == "" || seen[o.UUID] {
			continue
		}
		seen[o.UUID] = true
		nu := normalizeRAIDUUID(o.UUID)
		if skipUUIDs != nil && skipUUIDs[nu] {
			continue
		}
		if inactive[nu] {
			continue
		}
		if _, active := active[nu]; active {
			continue
		}
		if len(o.Devices) == 0 {
			continue
		}
		out = append(out, o)
	}
	return out
}

func parseExamineScanLine(line string) orphanArray {
	o := orphanArray{}
	if !strings.HasPrefix(line, "ARRAY ") {
		return o
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "ARRAY "))
	sp := strings.IndexByte(rest, ' ')
	if sp < 0 {
		o.MDPath = rest
	} else {
		o.MDPath = rest[:sp]
		parseScanMeta(rest[sp+1:], &o)
	}
	if !strings.HasPrefix(o.MDPath, "/dev/") {
		o.MDPath = "/dev/" + o.MDPath
	}
	if o.UUID != "" && len(o.Devices) == 0 {
		o.Devices = devicesWithUUID(o.UUID)
	}
	return o
}

func parseScanMeta(meta string, o *orphanArray) {
	for _, f := range strings.Fields(meta) {
		switch {
		case strings.HasPrefix(f, "UUID="):
			o.UUID = strings.TrimPrefix(f, "UUID=")
		case strings.HasPrefix(f, "level="):
			o.Level = strings.TrimPrefix(f, "level=")
		case strings.HasPrefix(f, "devices="):
			for _, d := range strings.Split(strings.TrimPrefix(f, "devices="), ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					o.Devices = append(o.Devices, d)
				}
			}
		}
	}
}

func devicesWithUUID(uuid string) []string {
	all, err := disks.List()
	if err != nil {
		return nil
	}
	var devs []string
	for _, d := range all {
		if strings.HasPrefix(d.Name, "md") || strings.HasPrefix(d.Name, "zram") ||
			strings.HasPrefix(d.Name, "mmcblk") || strings.HasPrefix(d.Name, "mtd") {
			continue
		}
		out, err := exec.Command("mdadm", "--examine", d.Path).CombinedOutput()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), uuid) {
			devs = append(devs, d.Path)
		}
	}
	return devs
}

func activeRAIDUUIDs() map[string]string {
	out := make(map[string]string)
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "md") {
			continue
		}
		uuid, err := os.ReadFile(filepath.Join("/sys/block", e.Name(), "md/uuid"))
		if err != nil {
			continue
		}
		u := normalizeRAIDUUID(strings.TrimSpace(string(uuid)))
		if u != "" {
			out[u] = e.Name()
		}
	}
	return out
}

func detectFormatting() []Item {
	patterns := []string{"mkfs.ext4", "mkfs.xfs", "mkfs.btrfs"}
	var items []Item
	seen := make(map[string]bool)
	for _, pat := range patterns {
		cmd := exec.Command("pgrep", "-af", pat)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			items = append(items, Item{
				Kind:     "mount_format",
				Message:  "Formatage de volume en cours sur l'hôte",
				Severity: SeverityWarn,
				Details: map[string]interface{}{
					"process": line,
				},
			})
		}
	}
	return items
}

func detectRAIDResync() []Item {
	arrays, err := raid.List()
	if err != nil {
		return nil
	}
	var items []Item
	for _, a := range arrays {
		action, pct, syncing := raid.SyncProgress(a.Name)
		if !syncing {
			continue
		}
		items = append(items, Item{
			Kind:     "raid_resync",
			Message:  fmt.Sprintf("%s : %s %.1f%%", a.Path, action, pct),
			Progress: int(pct),
			Severity: SeverityInfo,
			Details: map[string]interface{}{
				"name":   a.Name,
				"path":   a.Path,
				"action": action,
				"percent": pct,
			},
		})
	}
	return items
}

// normalizeRAIDUUID compare les UUID sysfs (tirets) et mdadm (deux-points).
func normalizeRAIDUUID(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.ReplaceAll(u, ":", "")
	u = strings.ReplaceAll(u, "-", "")
	return u
}

func sysfsUUIDToMdadm(uuid string) string {
	u := strings.ReplaceAll(strings.ToLower(uuid), "-", "")
	if len(u) != 32 {
		return uuid
	}
	return u[0:8] + ":" + u[8:16] + ":" + u[16:24] + ":" + u[24:32]
}

func raidLevelLabel(level string) string {
	level = strings.TrimPrefix(strings.ToLower(level), "raid")
	if level == "" {
		return "RAID"
	}
	return "RAID " + level
}

// RunPeriodic lance le scan à intervalle régulier (logs des actions requises).
func RunPeriodic(interval time.Duration) {
	tick := func() {
		r, err := Scan()
		if err != nil {
			return
		}
		for _, item := range r.Items {
			if item.Severity == SeverityAction {
				// Log only; recovery is explicit via API/UI.
				_ = item
			}
		}
	}
	tick()
	for range time.Tick(interval) {
		tick()
	}
}
