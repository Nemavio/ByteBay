package system

import (
	"log"
	"os/exec"
)

// Run tries each command variant; logs failures but does not fail hard (services may be absent).
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("system: %s %v: %s", name, args, string(out))
		return err
	}
	return nil
}

func RunQuiet(name string, args ...string) {
	_ = Run(name, args...)
}
