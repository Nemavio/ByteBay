package mounts

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytebay/bytebay/agent/internal/config"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,31}$`)

type MountPoint struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	Source        string `json:"source"`
	Fstype        string `json:"fstype"`
	Options       string `json:"options,omitempty"`
	Mounted       bool   `json:"mounted"`
}

type CreateRequest struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Fstype  string `json:"fstype"`
	Format  bool   `json:"format"`
	Options string `json:"options"`
}

type stateFile struct {
	Mounts []MountPoint `json:"mounts"`
}

func VolumesRoot() string {
	if v := os.Getenv("BYTEBAY_VOLUMES_PATH"); v != "" {
		return v
	}
	return "/srv/bytebay-volumes"
}

func ContainerPath(name string) string {
	return "/volumes/" + name
}

func List() ([]MountPoint, error) {
	saved, err := loadState()
	if err != nil {
		return nil, err
	}
	active := parseActiveMounts()
	for i := range saved {
		saved[i].Mounted = active[saved[i].HostPath]
	}
	return saved, nil
}

func Create(req CreateRequest) (*MountPoint, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	if req.Format {
		return nil, fmt.Errorf("use async job for format")
	}
	return createMountOnly(req)
}

func validateCreate(req CreateRequest) error {
	name := strings.TrimSpace(req.Name)
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid name: use letters, digits, - or _")
	}
	if normalizeSource(req.Source) == "" {
		return fmt.Errorf("source device required")
	}
	return nil
}

func normalizeSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if !strings.HasPrefix(source, "/dev/") {
		source = "/dev/" + strings.TrimPrefix(source, "/dev/")
	}
	return resolveStaleMDSource(source)
}

func createMountOnly(req CreateRequest) (*MountPoint, error) {
	name := strings.TrimSpace(req.Name)
	source := normalizeSource(req.Source)
	fstype := strings.TrimSpace(req.Fstype)
	if fstype == "" {
		fstype = "ext4"
	}

	hostPath := filepath.Join(VolumesRoot(), name)
	if err := os.MkdirAll(VolumesRoot(), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(hostPath, 0o755); err != nil {
		return nil, err
	}

	opts := strings.TrimSpace(req.Options)
	if opts == "" {
		opts = "defaults"
	}
	if err := doMount(source, hostPath, fstype, opts); err != nil {
		return nil, err
	}

	mp := MountPoint{
		Name:          name,
		HostPath:      hostPath,
		ContainerPath: ContainerPath(name),
		Source:        source,
		Fstype:        fstype,
		Options:       opts,
		Mounted:       true,
	}
	if err := upsertState(mp); err != nil {
		return nil, err
	}
	return &mp, nil
}

func Remove(name string) error {
	saved, err := loadState()
	if err != nil {
		return err
	}
	var target *MountPoint
	for i := range saved {
		if saved[i].Name == name {
			target = &saved[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("mount %q not found", name)
	}
	if isMounted(target.HostPath) {
		out, err := exec.Command("umount", target.HostPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("umount: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}
	if err := removeHostDir(target.HostPath); err != nil {
		return err
	}
	return deleteState(name)
}

func Restore() error {
	if err := MigrateRaidSources(); err != nil {
		return err
	}
	list, err := loadState()
	if err != nil {
		return err
	}
	for _, mp := range list {
		if isMounted(mp.HostPath) {
			continue
		}
		if err := os.MkdirAll(mp.HostPath, 0o755); err != nil {
			return err
		}
		opts := mp.Options
		if opts == "" {
			opts = "defaults"
		}
		if err := doMount(normalizeSource(mp.Source), mp.HostPath, mp.Fstype, opts); err != nil {
			return fmt.Errorf("restore %s: %w", mp.Name, err)
		}
	}
	return nil
}

func doMount(source, target, fstype, opts string) error {
	if isMounted(target) {
		return nil
	}
	args := []string{"-t", fstype, "-o", opts, source, target}
	out, err := exec.Command("mount", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func isMounted(path string) bool {
	return parseActiveMounts()[path]
}

// ActiveOnSource lists ByteBay mount names and a direct kernel mountpoint for a block device.
func ActiveOnSource(source string) (mountNames []string, kernelMount string) {
	source = normalizeSource(source)
	list, _ := List()
	for _, m := range list {
		if normalizeSource(m.Source) == source && m.Mounted {
			mountNames = append(mountNames, m.Name)
		}
	}
	kernelMount = findDeviceMount(source)
	return mountNames, kernelMount
}

func findDeviceMount(source string) string {
	source = normalizeSource(source)
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == source {
			return fields[1]
		}
	}
	return ""
}

func parseActiveMounts() map[string]bool {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()
	out := make(map[string]bool)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 {
			out[fields[1]] = true
		}
	}
	return out
}

func statePath() string {
	return filepath.Join(config.StateDir, "mounts.json")
}

func loadState() ([]MountPoint, error) {
	b, err := os.ReadFile(statePath())
	if os.IsNotExist(err) {
		return []MountPoint{}, nil
	}
	if err != nil {
		return nil, err
	}
	var sf stateFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return nil, err
	}
	if sf.Mounts == nil {
		sf.Mounts = []MountPoint{}
	}
	return sf.Mounts, nil
}

func saveState(mounts []MountPoint) error {
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stateFile{Mounts: mounts}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), b, 0o644)
}

func upsertState(mp MountPoint) error {
	list, err := loadState()
	if err != nil {
		return err
	}
	found := false
	for i := range list {
		if list[i].Name == mp.Name {
			list[i] = mp
			found = true
			break
		}
	}
	if !found {
		list = append(list, mp)
	}
	return saveState(list)
}

func deleteState(name string) error {
	list, err := loadState()
	if err != nil {
		return err
	}
	var next []MountPoint
	for _, m := range list {
		if m.Name != name {
			next = append(next, m)
		}
	}
	return saveState(next)
}

// CleanupForSource démonte et retire les points de montage liés à un périphérique.
func CleanupForSource(source string) {
	source = normalizeSource(source)
	list, _ := loadState()
	for _, m := range list {
		if normalizeSource(m.Source) == source {
			if err := Remove(m.Name); err != nil {
				_ = removeHostDir(m.HostPath)
				_ = deleteState(m.Name)
			}
		}
	}
}

// PruneOrphans supprime les dossiers vides sous VolumesRoot sans entrée d'état.
func PruneOrphans() {
	known := map[string]bool{}
	list, _ := loadState()
	for _, m := range list {
		known[m.Name] = true
	}
	entries, err := os.ReadDir(VolumesRoot())
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || known[e.Name()] {
			continue
		}
		p := filepath.Join(VolumesRoot(), e.Name())
		if isMounted(p) {
			continue
		}
		_ = os.Remove(p)
	}
}

func removeHostDir(path string) error {
	if path == "" || path == VolumesRoot() || !strings.HasPrefix(path, VolumesRoot()+string(os.PathSeparator)) {
		return nil
	}
	if isMounted(path) {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// MigrateRaidSources remplace les /dev/mdN obsolètes par des chemins stables by-id.
func MigrateRaidSources() error {
	list, err := loadState()
	if err != nil {
		return err
	}
	dirty := false
	for i, mp := range list {
		src := strings.TrimSpace(mp.Source)
		if !strings.HasPrefix(src, "/dev/md") {
			continue
		}
		if _, err := os.Stat(src); err == nil {
			continue
		}
		uuid := inferUUIDFromLegacyMD(src)
		if uuid != "" {
			seedBindingFromLegacySource(src, uuid)
		}
		resolved := resolveStaleMDSource(src)
		if resolved != src && resolved != "" {
			list[i].Source = resolved
			dirty = true
		}
	}
	if dirty {
		return saveState(list)
	}
	return nil
}

func inferUUIDFromLegacyMD(source string) string {
	names := []string{}
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "md") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 1 {
		return uuidFromMDName(names[0])
	}
	return ""
}
