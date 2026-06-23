package raid

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const mdadmConfPath = "/etc/mdadm/mdadm.conf"

func confPathForUUID(uuid string) (string, bool) {
	want := normalizeUUID(uuid)
	f, err := os.Open(mdadmConfPath)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "ARRAY ") {
			continue
		}
		if !strings.Contains(line, "UUID="+UUIDToMdadm(uuid)) && !strings.Contains(line, "UUID="+want) {
			// also try without normalizing in line
			found := false
			for _, f := range strings.Fields(line) {
				if strings.HasPrefix(f, "UUID=") {
					if normalizeUUID(strings.TrimPrefix(f, "UUID=")) == want {
						found = true
						break
					}
				}
			}
			if !found {
				continue
			}
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "ARRAY "))
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			return rest, true
		}
		return rest[:sp], true
	}
	return "", false
}

func persistArrayInConf(mdPath, uuid string) error {
	uuid = UUIDToMdadm(uuid)
	if mdPath == "" || uuid == "" {
		return nil
	}
	line := fmt.Sprintf("ARRAY %s metadata=1.2 UUID=%s", mdPath, uuid)
	b, err := os.ReadFile(mdadmConfPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(b)
	if strings.Contains(content, "UUID="+uuid) {
		return replaceArrayLine(uuid, line)
	}
	f, err := os.OpenFile(mdadmConfPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(line + "\n")
	return err
}

func replaceArrayLine(uuid, newLine string) error {
	b, err := os.ReadFile(mdadmConfPath)
	if err != nil {
		return err
	}
	uuid = UUIDToMdadm(uuid)
	var out []string
	replaced := false
	for _, line := range strings.Split(string(b), "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "ARRAY ") && strings.Contains(trim, "UUID="+uuid) {
			if !replaced {
				out = append(out, newLine)
				replaced = true
			}
			continue
		}
		out = append(out, line)
	}
	if !replaced {
		out = append(out, newLine)
	}
	return os.WriteFile(mdadmConfPath, []byte(strings.Join(out, "\n")), 0o644)
}

// syncMdadmConfFromScan met à jour mdadm.conf via mdadm --detail --scan (sans doublon UUID).
func syncMdadmConfFromScan() error {
	out, err := exec.Command("mdadm", "--detail", "--scan").Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ARRAY ") {
			continue
		}
		uuid := ""
		for _, f := range strings.Fields(line) {
			if strings.HasPrefix(f, "UUID=") {
				uuid = strings.TrimPrefix(f, "UUID=")
				break
			}
		}
		if uuid == "" {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "ARRAY "))
		sp := strings.IndexByte(rest, ' ')
		path := rest
		if sp > 0 {
			path = rest[:sp]
		}
		_ = persistArrayInConf(path, uuid)
	}
	return nil
}
