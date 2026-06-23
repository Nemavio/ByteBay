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
	"github.com/bytebay/bytebay/agent/internal/mounts"
)

var (
	mdCountRe  = regexp.MustCompile(`\[(\d+)/(\d+)\]`)
	mdStatusRe = regexp.MustCompile(`\[([U_]+)\]`)
	syncRe     = regexp.MustCompile(`(recovery|resync|reshape|check)\s*=\s*([\d.]+)%`)
)

type Array struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	StablePath  string   `json:"stable_path,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
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
	basePath := filepath.Join("/sys/block", name)
	if _, err := os.Stat(basePath); err != nil {
		return Array{}, fmt.Errorf("array %s not found", name)
	}
	base := filepath.Join(basePath, "md")
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
	uuidRaw, _ := readSysString(base + "/uuid")
	uuid := UUIDToMdadm(strings.TrimSpace(uuidRaw))
	return Array{
		Name:        name,
		Path:        "/dev/" + name,
		StablePath:  StableByIDPath(uuid),
		UUID:        uuid,
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
	plan, err := planCreate(req)
	if err != nil {
		return nil, err
	}
	return runCreate(plan)
}

type createPlan struct {
	req         CreateRequest
	level       string
	raidDevices int
	present     []string
	mdPath      string
	mdName      string
}

func planCreate(req CreateRequest) (*createPlan, error) {
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

	return &createPlan{
		req:         req,
		level:       level,
		raidDevices: raidDevices,
		present:     present,
		mdPath:      md,
		mdName:      filepath.Base(md),
	}, nil
}

func runCreate(plan *createPlan) (*Array, error) {
	for _, d := range plan.present {
		if err := prepareDeviceForRaid(d); err != nil {
			return nil, err
		}
	}
	return mdadmCreate(plan)
}

func mdadmCreate(plan *createPlan) (*Array, error) {
	args := []string{
		"--create", plan.mdPath,
		"--level=" + plan.level,
		"--raid-devices=" + strconv.Itoa(plan.raidDevices),
		"--bitmap", "internal",
		"--force",
		"--run",
	}
	for _, d := range plan.req.Devices {
		if d == "" || d == "missing" {
			args = append(args, "missing")
		} else {
			args = append(args, d)
		}
	}
	for i := len(plan.req.Devices); i < plan.raidDevices; i++ {
		args = append(args, "missing")
	}

	out, err := runMdadm(args...)
	if err != nil {
		return nil, fmt.Errorf("mdadm: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if err := ensureArrayStarted(plan); err != nil {
		return nil, err
	}

	arr, err := readArrayPtr(plan.mdName)
	if err != nil {
		return nil, err
	}
	if arr.Level == "" {
		return nil, fmt.Errorf("array %s not active after create", plan.mdPath)
	}
	arr.Degraded = plan.raidDevices > len(plan.present)
	_ = RecordBinding(arr.UUID, plan.mdPath, plan.req.Name)
	_ = persistArrayInConf(plan.mdPath, arr.UUID)
	return arr, nil
}

// ensureArrayStarted démarre l'array si mdadm --create n'a écrit que les métadonnées (RAID dégradé).
func ensureArrayStarted(plan *createPlan) error {
	if isArrayActive(plan.mdName) {
		return nil
	}
	args := []string{"--assemble", "--force", "--run", plan.mdPath}
	args = append(args, plan.present...)
	out, err := runMdadm(args...)
	if err != nil {
		return fmt.Errorf("mdadm --assemble: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if !isArrayActive(plan.mdName) {
		return fmt.Errorf("array %s not started (vérifiez le nombre de disques pour RAID %s)", plan.mdPath, plan.level)
	}
	return nil
}

// StartInactive démarre un array inactif, y compris en mode dégradé (disques manquants).
func StartInactive(mdPath string) (*Array, error) {
	return startInactiveDegraded(mdPath)
}

func startInactiveDegraded(mdPath string) (*Array, error) {
	if !strings.HasPrefix(mdPath, "/dev/") {
		mdPath = "/dev/" + mdPath
	}
	name := filepath.Base(mdPath)
	if isArrayWritable(name) {
		EnsureBinding(mdPath)
		return readArrayPtr(name)
	}

	devices := presentMemberDevices(mdPath)
	uuid := UUIDFromMDPath(mdPath)
	if uuid == "" && len(devices) > 0 {
		uuid = UUIDFromDevice(devices[0])
	}
	targetPath := resolveTargetAssemblyPath(mdPath, uuid)

	sysfsExists := false
	if _, err := os.Stat(filepath.Join("/sys/block", name)); err == nil {
		sysfsExists = true
	}

	if len(devices) == 0 && !sysfsExists {
		return nil, fmt.Errorf("aucun disque membre disponible pour %s", mdPath)
	}
	if len(devices) > 0 {
		if err := validateDegradedStart(mdPath, len(devices)); err != nil {
			return nil, err
		}
	}

	targetName := filepath.Base(targetPath)
	if uuid != "" && targetName != name {
		if err := rehomeArrayUUID(uuid, targetPath); err != nil {
			return nil, err
		}
		mdPath = targetPath
		name = targetName
		sysfsExists = false
		if _, err := os.Stat(filepath.Join("/sys/block", name)); err == nil {
			sysfsExists = true
		}
		devices = presentMemberDevices(mdPath)
	}

	// Disques déjà rattachés à cet array : activer sans les ré-attacher (évite "is busy").
	if sysfsExists && len(devices) > 0 && allDevicesHeldByMD(name, devices) {
		if arr, err := activateExistingInactive(mdPath); err == nil {
			finishAssembly(mdPath, arr.UUID)
			return arr, nil
		}
	}

	if sysfsExists {
		if out, err := stopMDArray(mdPath); err != nil {
			if len(devices) > 0 && allDevicesHeldByMD(name, devices) {
				if arr, err2 := activateExistingInactive(mdPath); err2 == nil {
					finishAssembly(mdPath, arr.UUID)
					return arr, nil
				}
			}
			return nil, fmt.Errorf("mdadm --stop %s: %s", mdPath, strings.TrimSpace(string(out)))
		}
		sysfsExists = false
	}

	if len(devices) == 0 {
		devices = presentMemberDevices(mdPath)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("aucun disque membre disponible pour %s", mdPath)
	}

	args := []string{"--assemble", "--force", "--run"}
	if uuid != "" {
		args = append(args, "--uuid="+UUIDToMdadm(uuid))
	}
	args = append(args, mdPath)
	args = append(args, devices...)
	out, err := runMdadm(args...)
	if err != nil {
		if allDevicesHeldByMD(name, devices) {
			if arr, err2 := activateExistingInactive(mdPath); err2 == nil {
				finishAssembly(mdPath, arr.UUID)
				return arr, nil
			}
		}
		return nil, formatAssembleError(mdPath, out, err)
	}
	if !isArrayWritable(name) {
		return nil, formatAssembleError(mdPath, out, fmt.Errorf("array not started after degraded assemble"))
	}
	arr, err := readArrayPtr(name)
	if err != nil {
		return nil, err
	}
	finishAssembly(mdPath, arr.UUID)
	return arr, nil
}

func resolveTargetAssemblyPath(currentPath, uuid string) string {
	if preferred := PreferredPath(uuid); preferred != "" {
		return preferred
	}
	if currentPath != "" {
		return currentPath
	}
	return ""
}

func rehomeArrayUUID(uuid, targetPath string) error {
	current := FindMDNameByUUID(uuid)
	if current == "" {
		return nil
	}
	if current == filepath.Base(targetPath) {
		return nil
	}
	if isMDSyncing(current) {
		return fmt.Errorf(
			"le RAID est actif sur /dev/%s (resync en cours) : attendez la fin avant de le rattacher à %s",
			current, targetPath,
		)
	}
	_, err := stopMDArray("/dev/" + current)
	return err
}

func isMDSyncing(mdName string) bool {
	action, _, syncing := SyncProgress(mdName)
	return syncing || action != ""
}

func finishAssembly(mdPath, uuid string) {
	if uuid == "" {
		uuid = UUIDFromMDPath(mdPath)
	}
	if uuid == "" {
		return
	}
	EnsureBinding(mdPath)
	_ = persistArrayInConf(mdPath, uuid)
}

func stopMDArray(mdPath string) ([]byte, error) {
	out, err := runMdadm("--stop", mdPath)
	if err == nil {
		return out, nil
	}
	return runMdadm("--stop", "--force", mdPath)
}

// activateExistingInactive démarre un array déjà assemblé (disques déjà slaves).
func activateExistingInactive(mdPath string) (*Array, error) {
	name := filepath.Base(mdPath)
	if isArrayWritable(name) {
		return readArrayPtr(name)
	}
	try := [][]string{
		{"--assemble", "--force", "--run", mdPath},
		{"--run", "--force", mdPath},
		{"-R", mdPath},
	}
	var lastOut []byte
	var lastErr error
	for _, args := range try {
		out, err := runMdadm(args...)
		lastOut, lastErr = out, err
		if err == nil && isArrayWritable(name) {
			return readArrayPtr(name)
		}
	}
	if lastErr != nil {
		return nil, formatAssembleError(mdPath, lastOut, lastErr)
	}
	return nil, formatAssembleError(mdPath, lastOut, fmt.Errorf("array not started"))
}

func allDevicesHeldByMD(mdName string, devices []string) bool {
	if len(devices) == 0 {
		return false
	}
	for _, d := range devices {
		base := filepath.Base(strings.TrimPrefix(d, "/dev/"))
		holders, err := os.ReadDir(filepath.Join("/sys/block", base, "holders"))
		if err != nil {
			return false
		}
		held := false
		for _, h := range holders {
			if h.Name() == mdName {
				held = true
				break
			}
		}
		if !held {
			return false
		}
	}
	return true
}

// presentMemberDevices liste les disques membres présents (ignore les manquants).
func presentMemberDevices(mdPath string) []string {
	name := filepath.Base(strings.TrimPrefix(mdPath, "/dev/"))
	var candidates []string

	if slaves, err := os.ReadDir(filepath.Join("/sys/block", name, "slaves")); err == nil {
		for _, s := range slaves {
			candidates = append(candidates, "/dev/"+s.Name())
		}
	}
	if len(candidates) == 0 {
		candidates = mdadmMemberPaths(mdPath)
	}
	if len(candidates) == 0 {
		candidates = devicesFromMdstat(name)
	}

	seen := make(map[string]bool)
	var out []string
	for _, d := range candidates {
		if !strings.HasPrefix(d, "/dev/") {
			d = "/dev/" + strings.TrimPrefix(d, "/dev/")
		}
		if seen[d] {
			continue
		}
		if _, err := os.Stat(d); err != nil {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func mdadmMemberPaths(mdPath string) []string {
	detail, err := exec.Command("mdadm", "-D", mdPath).CombinedOutput()
	if err != nil {
		return nil
	}
	var devs []string
	for _, line := range strings.Split(string(detail), "\n") {
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

func devicesFromMdstat(mdName string) []string {
	f, err := os.Open("/proc/mdstat")
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, mdName+":") && !strings.HasPrefix(line, mdName+" ") {
			continue
		}
		var devs []string
		for _, f := range strings.Fields(line) {
			if idx := strings.Index(f, "["); idx > 0 {
				dev := f[:idx]
				if strings.HasPrefix(dev, "s") || strings.HasPrefix(dev, "nvme") || strings.HasPrefix(dev, "mmc") {
					devs = append(devs, "/dev/"+dev)
				}
			}
		}
		return devs
	}
	return nil
}

func validateDegradedStart(mdPath string, present int) error {
	detail, err := exec.Command("mdadm", "-D", mdPath).CombinedOutput()
	if err != nil {
		// array peut être arrêté ; lire level depuis examine des membres
		return nil
	}
	level := ""
	total := 0
	for _, line := range strings.Split(string(detail), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Raid Level :") {
			level = strings.TrimSpace(strings.TrimPrefix(line, "Raid Level :"))
		}
		if strings.HasPrefix(line, "Raid Devices :") {
			fmt.Sscanf(strings.TrimPrefix(line, "Raid Devices :"), "%d", &total)
		}
	}
	if total == 0 {
		return nil
	}
	min := minDevicesForLevel(level, total)
	if present < min {
		return fmt.Errorf(
			"impossible de démarrer %s en %s : %d disque(s) présent(s), minimum %d sur %d configurés",
			mdPath, level, present, min, total,
		)
	}
	return nil
}

func minDevicesForLevel(level string, total int) int {
	level = strings.ToLower(strings.TrimPrefix(level, "raid"))
	switch level {
	case "0":
		return total
	case "1":
		return 1
	case "5":
		if total > 1 {
			return total - 1
		}
		return 1
	case "6":
		if total > 2 {
			return total - 2
		}
		return 1
	case "10":
		if total > 2 {
			return total / 2
		}
		return 1
	default:
		return 1
	}
}

// FindInactiveByUUID retourne le chemin /dev/mdX d'un array inactif.
func FindInactiveByUUID(uuid string) (string, bool) {
	want := normalizeUUID(uuid)
	for _, name := range listMDNames() {
		if isArrayActive(name) {
			continue
		}
		u, err := readSysString(filepath.Join("/sys/block", name, "md/uuid"))
		if err != nil || normalizeUUID(u) != want {
			continue
		}
		return "/dev/" + name, true
	}
	return "", false
}

// AssembleInactiveForce assemble un RAID orphelin en ne gardant que les disques présents.
func AssembleInactiveForce(mdPath string, devices []string) (*Array, error) {
	if mdPath == "" || len(devices) == 0 {
		return nil, fmt.Errorf("md path and devices required")
	}
	present := filterPresentDevices(devices)
	if len(present) == 0 {
		return nil, fmt.Errorf("aucun disque membre disponible")
	}
	uuid := UUIDFromDevice(present[0])
	if preferred := PreferredPath(uuid); preferred != "" {
		mdPath = preferred
	} else if next, err := nextMD(); err == nil {
		mdPath = next
	} else if !strings.HasPrefix(mdPath, "/dev/") {
		mdPath = "/dev/" + mdPath
	}
	if !strings.HasPrefix(mdPath, "/dev/") {
		mdPath = "/dev/" + mdPath
	}
	name := filepath.Base(mdPath)
	if isArrayWritable(name) {
		EnsureBinding(mdPath)
		return readArrayPtr(name)
	}
	if uuid != "" {
		if err := rehomeArrayUUID(uuid, mdPath); err != nil {
			return nil, err
		}
	}
	if alt := findInactiveMDForDevices(present); alt != "" && alt != name {
		return startInactiveDegraded("/dev/" + alt)
	}
	if _, err := os.Stat(filepath.Join("/sys/block", name)); err == nil {
		return startInactiveDegraded(mdPath)
	}
	if err := validateDegradedStartFromDevices(present); err != nil {
		return nil, err
	}
	args := []string{"--assemble", "--force", "--run"}
	if uuid != "" {
		args = append(args, "--uuid="+UUIDToMdadm(uuid))
	}
	args = append(args, mdPath)
	args = append(args, present...)
	out, err := runMdadm(args...)
	if err != nil {
		return nil, formatAssembleError(mdPath, out, err)
	}
	if !isArrayWritable(name) {
		return nil, formatAssembleError(mdPath, out, fmt.Errorf("array %s not started after degraded assemble", mdPath))
	}
	arr, err := readArrayPtr(name)
	if err != nil {
		return nil, err
	}
	finishAssembly(mdPath, arr.UUID)
	return arr, nil
}

func filterPresentDevices(devices []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, d := range devices {
		if !strings.HasPrefix(d, "/dev/") {
			d = "/dev/" + strings.TrimPrefix(d, "/dev/")
		}
		if seen[d] {
			continue
		}
		if _, err := os.Stat(d); err != nil {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	return out
}

func validateDegradedStartFromDevices(present []string) error {
	if len(present) == 0 {
		return fmt.Errorf("aucun disque membre disponible")
	}
	out, err := exec.Command("mdadm", "--examine", present[0]).CombinedOutput()
	if err != nil {
		return nil
	}
	level := ""
	total := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Raid Level :") {
			level = strings.TrimSpace(strings.TrimPrefix(line, "Raid Level :"))
		}
		if strings.HasPrefix(line, "Raid Devices :") {
			fmt.Sscanf(strings.TrimPrefix(line, "Raid Devices :"), "%d", &total)
		}
	}
	if total == 0 {
		return nil
	}
	min := minDevicesForLevel(level, total)
	if len(present) < min {
		return fmt.Errorf(
			"impossible de démarrer en %s : %d disque(s) présent(s), minimum %d sur %d configurés",
			level, len(present), min, total,
		)
	}
	return nil
}

// AssembleInactive démarre un array dont les superblocs existent mais qui n'est pas actif.
func AssembleInactive(mdPath string, devices []string) (*Array, error) {
	if mdPath == "" || len(devices) == 0 {
		return nil, fmt.Errorf("md path and devices required")
	}
	if !strings.HasPrefix(mdPath, "/dev/") {
		mdPath = "/dev/" + mdPath
	}
	name := filepath.Base(mdPath)
	if isArrayActive(name) {
		return readArrayPtr(name)
	}
	if alt := findInactiveMDForDevices(devices); alt != "" && alt != name {
		return startInactiveDegraded("/dev/" + alt)
	}
	if _, err := os.Stat(filepath.Join("/sys/block", name)); err == nil {
		return startInactiveDegraded(mdPath)
	}
	if alt := findInactiveMDForDevices(devices); alt != "" {
		return startInactiveDegraded("/dev/" + alt)
	}
	args := []string{"--assemble", "--force", "--run", mdPath}
	args = append(args, devices...)
	out, err := runMdadm(args...)
	if err != nil {
		return nil, formatAssembleError(mdPath, out, err)
	}
	if !isArrayActive(name) {
		return nil, formatAssembleError(mdPath, out, fmt.Errorf("array %s not started after assemble", mdPath))
	}
	return readArrayPtr(name)
}

func listMDNames() []string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "md") {
			names = append(names, e.Name())
		}
	}
	return names
}

func findInactiveMDForDevices(devices []string) string {
	want := make(map[string]bool, len(devices))
	for _, d := range devices {
		want[filepath.Base(strings.TrimPrefix(d, "/dev/"))] = true
	}
	f, err := os.Open("/proc/mdstat")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "inactive") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 2 || !strings.HasPrefix(line, "md") {
			continue
		}
		mdName := strings.TrimSpace(line[:colon])
		matched := 0
		for dev := range want {
			if strings.Contains(line, dev+"[") || strings.Contains(line, dev+" ") {
				matched++
			}
		}
		if matched > 0 && matched >= len(want) {
			return mdName
		}
	}
	return ""
}

func normalizeUUID(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if !strings.Contains(u, ":") {
		return u
	}
	parts := strings.Split(u, ":")
	if len(parts) != 4 {
		return u
	}
	var out []string
	for _, p := range parts {
		if len(p) != 8 {
			return u
		}
		out = append(out, p[:4], p[4:])
	}
	return strings.Join(out, "-")
}

func formatAssembleError(mdPath string, out []byte, err error) error {
	msg := strings.TrimSpace(string(out))
	if msg == "" && err != nil {
		msg = err.Error()
	}
	if strings.Contains(msg, "is busy") {
		msg = strings.TrimSpace(msg) + "\nLes disques sont déjà attachés au volume RAID. Réessayez ou consultez l'app RAID."
	}
	if detail := inactiveArrayDetail(mdPath); detail != "" {
		return fmt.Errorf("%s\n%s", msg, detail)
	}
	if msg != "" {
		return fmt.Errorf("mdadm: %s", msg)
	}
	return err
}

func inactiveArrayDetail(mdPath string) string {
	name := filepath.Base(strings.TrimPrefix(mdPath, "/dev/"))
	detail, err := exec.Command("mdadm", "-D", mdPath).CombinedOutput()
	if err != nil {
		return ""
	}
	var level string
	var total, active int
	for _, line := range strings.Split(string(detail), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Raid Level :") {
			level = strings.TrimSpace(strings.TrimPrefix(line, "Raid Level :"))
		}
		if strings.HasPrefix(line, "Raid Devices :") {
			fmt.Sscanf(strings.TrimPrefix(line, "Raid Devices :"), "%d", &total)
		}
		if strings.HasPrefix(line, "Active Devices :") {
			fmt.Sscanf(strings.TrimPrefix(line, "Active Devices :"), "%d", &active)
		}
	}
	if total > 0 && active < total {
		return fmt.Sprintf(
			"Le RAID %s (%s) a %d/%d disques présents. Un démarrage dégradé est possible depuis le panel.",
			name, level, active, total,
		)
	}
	return ""
}

func isArrayActive(name string) bool {
	state, err := readSysString(filepath.Join("/sys/block", name, "md/array_state"))
	if err != nil {
		return false
	}
	state = strings.ToLower(strings.TrimSpace(state))
	switch state {
	case "inactive", "clear", "":
		return false
	default:
		return true
	}
}

func isArrayWritable(name string) bool {
	state, err := readSysString(filepath.Join("/sys/block", name, "md/array_state"))
	if err != nil {
		return false
	}
	state = strings.ToLower(strings.TrimSpace(state))
	return state == "active" || state == "clean" || strings.HasPrefix(state, "active")
}

// SyncProgress lit la progression resync/recovery depuis /proc/mdstat.
func SyncProgress(name string) (action string, percent float64, syncing bool) {
	f, err := os.Open("/proc/mdstat")
	if err != nil {
		return "", 0, false
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
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			syncLine := strings.TrimSpace(lines[j])
			if m := syncRe.FindStringSubmatch(syncLine); len(m) == 3 {
				pct, _ := strconv.ParseFloat(m[2], 64)
				return m[1], pct, true
			}
		}
		break
	}
	return "", 0, false
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
	if err := prepareDeviceForRaid(device); err != nil {
		return nil, err
	}
	out, err := runMdadm("--add", path, device, "--force", "--run")
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
	if names, kernelMount := mounts.ActiveOnSource(path); len(names) > 0 {
		return fmt.Errorf(
			"point de montage actif « %s » : démontez-le depuis Montages avant d'arrêter le RAID",
			names[0],
		)
	} else if kernelMount != "" {
		return fmt.Errorf(
			"le volume est monté sur %s : démontez-le avant d'arrêter le RAID",
			kernelMount,
		)
	}
	out, err := runMdadm("--stop", path)
	if err != nil {
		return fmt.Errorf("mdadm --stop: %s: %w", strings.TrimSpace(string(out)), err)
	}
	mounts.CleanupForSource(path)
	mounts.PruneOrphans()
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

// prepareDeviceForRaid efface une signature RAID résiduelle (création avortée, array arrêté).
func prepareDeviceForRaid(dev string) error {
	if !deviceHasRaidMetadata(dev) {
		return nil
	}
	out, err := runMdadm("--zero-superblock", "--force", dev)
	if err == nil {
		return nil
	}
	wipeOut, wipeErr := exec.Command("wipefs", "-f", "-a", dev).CombinedOutput()
	if wipeErr != nil {
		return fmt.Errorf(
			"mdadm --zero-superblock %s: %s; wipefs: %s: %w",
			dev, strings.TrimSpace(string(out)), strings.TrimSpace(string(wipeOut)), err,
		)
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
