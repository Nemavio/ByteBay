package raid

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedSyncActions = map[string]bool{
	"check":  true,
	"repair": true,
	"idle":   true,
}

// SetSyncAction lance ou arrête check / repair / idle sur un array actif.
func SetSyncAction(name, action string) (*ArrayDetail, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if !allowedSyncActions[action] {
		return nil, fmt.Errorf("action invalide : %s (check, repair ou idle)", action)
	}
	path := name
	if !strings.HasPrefix(path, "/dev/") {
		path = "/dev/" + path
	}
	base := filepath.Base(path)
	if _, err := os.Stat(filepath.Join("/sys/block", base)); err != nil {
		return nil, fmt.Errorf("array %s introuvable", path)
	}
	if !isArrayActive(base) {
		return nil, fmt.Errorf("array %s inactif", path)
	}
	if action == "repair" {
		if cnt, err := readMismatchCount(base); err == nil && cnt == 0 {
			return nil, fmt.Errorf("aucune incohérence détectée (mismatch_cnt=0) : lancez d'abord une vérification (check)")
		}
	}
	if action != "idle" {
		current, _ := readSyncAction(base)
		if current != "" && current != "idle" && current != action {
			return nil, fmt.Errorf("opération %s déjà en cours sur %s", current, path)
		}
	}
	out, err := runMdadm("--action="+action, path)
	if err != nil {
		return nil, fmt.Errorf("mdadm --action=%s: %s: %w", action, strings.TrimSpace(string(out)), err)
	}
	return Detail(base)
}

func readSyncAction(mdName string) (string, error) {
	return readSysString(filepath.Join("/sys/block", mdName, "md/sync_action"))
}

func readMismatchCount(mdName string) (int64, error) {
	raw, err := readSysString(filepath.Join("/sys/block", mdName, "md/mismatch_cnt"))
	if err != nil {
		return 0, err
	}
	var n int64
	_, err = fmt.Sscanf(strings.TrimSpace(raw), "%d", &n)
	return n, err
}

// CurrentSyncAction retourne l'action de synchronisation en cours (idle si aucune).
func CurrentSyncAction(name string) string {
	base := filepath.Base(strings.TrimPrefix(name, "/dev/"))
	s, err := readSyncAction(base)
	if err != nil || s == "" {
		return "idle"
	}
	return s
}
