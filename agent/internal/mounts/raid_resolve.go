package mounts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytebay/bytebay/agent/internal/config"
)

type raidBinding struct {
	UUID string `json:"uuid"`
	Path string `json:"path"`
}

type raidBindingsFile struct {
	Bindings []raidBinding `json:"bindings"`
}

func bindingsPath() string {
	return filepath.Join(config.StateDir, "raid-bindings.json")
}

func loadRaidBindings() []raidBinding {
	b, err := os.ReadFile(bindingsPath())
	if err != nil {
		return nil
	}
	var bf raidBindingsFile
	if err := json.Unmarshal(b, &bf); err != nil {
		return nil
	}
	return bf.Bindings
}

func seedBindingFromLegacySource(legacyPath, uuid string) {
	if legacyPath == "" || uuid == "" {
		return
	}
	list := loadRaidBindings()
	for _, b := range list {
		if normalizeRaidUUID(b.UUID) == normalizeRaidUUID(uuid) {
			return
		}
	}
	list = append(list, raidBinding{UUID: uuidToMdadm(uuid), Path: legacyPath})
	_ = os.MkdirAll(config.StateDir, 0o755)
	b, _ := json.MarshalIndent(raidBindingsFile{Bindings: list}, "", "  ")
	_ = os.WriteFile(bindingsPath(), b, 0o644)
}

func resolveStaleMDSource(source string) string {
	if source == "" {
		return ""
	}
	if _, err := os.Stat(source); err == nil {
		return source
	}
	base := filepath.Base(source)
	if !strings.HasPrefix(base, "md") {
		return source
	}

	uuid := ""
	for _, b := range loadRaidBindings() {
		if b.Path == source && b.UUID != "" {
			uuid = b.UUID
			break
		}
	}
	if uuid == "" {
		uuid = inferUUIDFromLegacyMD(source)
	}
	if uuid == "" {
		return source
	}
	stable := filepath.Join("/dev/disk/by-id", "md-uuid-"+uuidToMdadm(uuid))
	if _, err := os.Stat(stable); err == nil {
		return stable
	}
	if name := findMDNameByUUID(uuid); name != "" {
		p := "/dev/" + name
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, b := range loadRaidBindings() {
		if normalizeRaidUUID(b.UUID) == normalizeRaidUUID(uuid) && b.Path != "" {
			if _, err := os.Stat(b.Path); err == nil {
				return b.Path
			}
		}
	}
	return source
}

func findMDNameByUUID(uuid string) string {
	want := normalizeRaidUUID(uuid)
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/sys/block", e.Name(), "md/uuid"))
		if err != nil {
			continue
		}
		if normalizeRaidUUID(string(raw)) == want {
			return e.Name()
		}
	}
	return ""
}

func uuidFromMDName(mdName string) string {
	raw, err := os.ReadFile(filepath.Join("/sys/block", mdName, "md/uuid"))
	if err != nil {
		return ""
	}
	return uuidToMdadm(strings.TrimSpace(string(raw)))
}

func uuidToMdadm(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	if strings.Contains(u, ":") {
		return u
	}
	u = strings.ReplaceAll(u, "-", "")
	if len(u) != 32 {
		return u
	}
	return u[0:8] + ":" + u[8:16] + ":" + u[16:24] + ":" + u[24:32]
}

func normalizeRaidUUID(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.ReplaceAll(u, ":", "")
	u = strings.ReplaceAll(u, "-", "")
	return u
}
