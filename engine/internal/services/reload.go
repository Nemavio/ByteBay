package services

import (
	"log"
	"os/exec"
)

func ReloadNFS() string {
	if err := run("exportfs", "-ra"); err != nil {
		if err := run("supervisorctl", "restart", "nfs"); err != nil {
			return "config written (nfs reload failed)"
		}
		return "supervisor nfs restarted"
	}
	return "exportfs -ra ok"
}

func ReloadSamba() string {
	if err := run("supervisorctl", "restart", "smbd", "nmbd"); err != nil {
		if err := run("smbcontrol", "all", "reload-config"); err != nil {
			return "config written (smbd reload failed)"
		}
		return "smbcontrol reload ok"
	}
	return "smbd/nmbd restarted"
}

func ReloadFTP() string {
	if err := run("supervisorctl", "restart", "vsftpd"); err != nil {
		return "config written (vsftpd reload failed)"
	}
	return "vsftpd restarted"
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		log.Printf("services: %s %v: %s", name, args, string(out))
		return err
	}
	return nil
}
