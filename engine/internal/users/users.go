package users

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bytebay/bytebay/engine/internal/config"
)

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Samba    bool   `json:"samba"`
	FTP      bool   `json:"ftp"`
}

type ACLRule struct {
	Path     string `json:"path"`
	Username string `json:"username"`
	CanRead  bool   `json:"can_read"`
	CanWrite bool   `json:"can_write"`
}

type SyncPayload struct {
	Users []Account `json:"users"`
	ACL   []ACLRule `json:"acl"`
}

func passwdFile() string {
	return filepath.Join(config.StateDir, "ftp-passwd.json")
}

func Sync(p SyncPayload) error {
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		return err
	}
	for _, u := range p.Users {
		if u.Samba {
			if err := ensureSambaUser(u.Username, u.Password); err != nil {
				return err
			}
		}
	}
	if err := writeFTPUsers(p.Users); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(config.StateDir, "sync.json"), b, 0o600)
}

func ensureSambaUser(username, password string) error {
	_ = exec.Command("useradd", "-M", "-s", "/usr/sbin/nologin", username).Run()
	cmd := exec.Command("smbpasswd", "-a", "-s", username)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd: %s: %w", string(out), err)
	}
	return nil
}

func writeFTPUsers(users []Account) error {
	var enabled []Account
	for _, u := range users {
		if u.FTP {
			enabled = append(enabled, u)
		}
	}
	b, err := json.Marshal(enabled)
	if err != nil {
		return err
	}
	return os.WriteFile(passwdFile(), b, 0o600)
}
