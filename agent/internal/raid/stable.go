package raid

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bytebay/bytebay/agent/internal/config"
)

type binding struct {
	UUID string `json:"uuid"`
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

type bindingsFile struct {
	Bindings []binding `json:"bindings"`
}

func bindingsPath() string {
	return filepath.Join(config.StateDir, "raid-bindings.json")
}

func loadBindings() ([]binding, error) {
	b, err := os.ReadFile(bindingsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bf bindingsFile
	if err := json.Unmarshal(b, &bf); err != nil {
		return nil, err
	}
	return bf.Bindings, nil
}

func RecordBinding(uuid, path, name string) error {
	uuid = UUIDToMdadm(uuid)
	if uuid == "" || path == "" {
		return nil
	}
	if !strings.HasPrefix(path, "/dev/") {
		path = "/dev/" + path
	}
	list, err := loadBindings()
	if err != nil {
		return err
	}
	norm := normalizeUUID(uuid)
	found := false
	for i := range list {
		if normalizeUUID(list[i].UUID) == norm {
			list[i].Path = path
			if name != "" {
				list[i].Name = name
			}
			found = true
			break
		}
	}
	if !found {
		list = append(list, binding{UUID: UUIDToMdadm(uuid), Path: path, Name: name})
	}
	return saveBindings(list)
}

func saveBindings(list []binding) error {
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(bindingsFile{Bindings: list}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bindingsPath(), b, 0o644)
}

// PreferredPath retourne le chemin /dev/mdX enregistré pour cet UUID (ex. /dev/md0).
func PreferredPath(uuid string) string {
	uuid = UUIDToMdadm(uuid)
	if uuid == "" {
		return ""
	}
	norm := normalizeUUID(uuid)
	list, _ := loadBindings()
	for _, b := range list {
		if normalizeUUID(b.UUID) == norm && b.Path != "" {
			return b.Path
		}
	}
	if p, ok := confPathForUUID(uuid); ok {
		return p
	}
	return ""
}

// StableByIDPath retourne le lien stable /dev/disk/by-id/md-uuid-…
func StableByIDPath(uuid string) string {
	uuid = UUIDToMdadm(uuid)
	if uuid == "" {
		return ""
	}
	return filepath.Join("/dev/disk/by-id", "md-uuid-"+uuid)
}

// UUIDToMdadm normalise un UUID sysfs ou mdadm vers le format mdadm (avec ':').
func UUIDToMdadm(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if u == "" {
		return ""
	}
	if strings.Contains(u, ":") {
		return u
	}
	u = strings.ReplaceAll(u, "-", "")
	if len(u) != 32 {
		return u
	}
	return fmt.Sprintf("%s:%s:%s:%s", u[0:8], u[8:16], u[16:24], u[24:32])
}

func UUIDFromMDName(mdName string) string {
	raw, err := readSysString(filepath.Join("/sys/block", mdName, "md/uuid"))
	if err != nil {
		return ""
	}
	return UUIDToMdadm(strings.TrimSpace(raw))
}

func UUIDFromMDPath(mdPath string) string {
	return UUIDFromMDName(filepath.Base(strings.TrimPrefix(mdPath, "/dev/")))
}

func UUIDFromDevice(dev string) string {
	out, err := exec.Command("mdadm", "--examine", dev).CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Array UUID") {
			return UUIDToMdadm(strings.TrimSpace(strings.TrimPrefix(line, "Array UUID :")))
		}
	}
	return ""
}

// FindMDNameByUUID retourne le nom sysfs (ex. md127) d'un array actif ou inactif.
func FindMDNameByUUID(uuid string) string {
	want := normalizeUUID(uuid)
	for _, name := range listMDNames() {
		if normalizeUUID(UUIDFromMDName(name)) == want {
			return name
		}
	}
	return ""
}

// ResolveMDSource résout un ancien /dev/mdN vers le périphérique actuel (de préférence by-id).
func ResolveMDSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	if !strings.HasPrefix(source, "/dev/") {
		source = "/dev/" + strings.TrimPrefix(source, "/dev/")
	}
	if _, err := os.Stat(source); err == nil {
		return source
	}
	if !strings.HasPrefix(filepath.Base(source), "md") {
		return source
	}

	uuid := ""
	list, _ := loadBindings()
	for _, b := range list {
		if b.Path == source && b.UUID != "" {
			uuid = b.UUID
			break
		}
	}
	if uuid == "" {
		uuid = uuidForStaleMDSource(source)
	}
	if uuid == "" {
		return source
	}
	if stable := StableByIDPath(uuid); stable != "" {
		if _, err := os.Stat(stable); err == nil {
			return stable
		}
	}
	if name := FindMDNameByUUID(uuid); name != "" {
		return "/dev/" + name
	}
	if preferred := PreferredPath(uuid); preferred != "" {
		if _, err := os.Stat(preferred); err == nil {
			return preferred
		}
	}
	return source
}

func uuidForStaleMDSource(source string) string {
	list, _ := loadBindings()
	for _, b := range list {
		if b.Path == source {
			return b.UUID
		}
	}
	names := listMDNames()
	if len(names) == 1 {
		return UUIDFromMDName(names[0])
	}
	return ""
}

// EnsureBinding enregistre le chemin préféré d'un array actif s'il est nouveau.
func EnsureBinding(mdPath string) {
	uuid := UUIDFromMDPath(mdPath)
	if uuid == "" {
		return
	}
	if PreferredPath(uuid) != "" {
		return
	}
	name := ""
	if detail, err := exec.Command("mdadm", "-D", mdPath).CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(detail), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name :") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "Name :"))
				break
			}
		}
	}
	_ = RecordBinding(uuid, mdPath, name)
	_ = persistArrayInConf(mdPath, uuid)
}

// SeedBindingFromLegacySource enregistre un binding quand un montage référence /dev/md0.
func SeedBindingFromLegacySource(legacyPath, uuid string) {
	if legacyPath == "" || uuid == "" {
		return
	}
	if PreferredPath(uuid) != "" {
		return
	}
	_ = RecordBinding(uuid, legacyPath, "")
}
