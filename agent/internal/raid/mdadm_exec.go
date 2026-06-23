package raid

import (
	"bytes"
	"os/exec"
	"strings"
)

// runMdadm exécute mdadm sans interaction (options + réponses stdin de secours).
func runMdadm(args ...string) ([]byte, error) {
	cmd := exec.Command("mdadm", args...)
	cmd.Stdin = bytes.NewBufferString("yes\nyes\nyes\n")
	return cmd.CombinedOutput()
}

func deviceHasRaidMetadata(dev string) bool {
	out, err := exec.Command("mdadm", "--examine", dev).CombinedOutput()
	if err == nil {
		s := strings.ToLower(string(out))
		if strings.Contains(s, "magic") || strings.Contains(s, "raid level") {
			return true
		}
	}
	out, err = exec.Command("blkid", "-p", "-o", "export", dev).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "TYPE=linux_raid_member")
}
