package users

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bytebay/bytebay/engine/internal/config"
	"github.com/bytebay/bytebay/engine/internal/shares"
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

func syncPath() string {
	return filepath.Join(config.StateDir, "sync.json")
}

func passwdFile() string {
	return filepath.Join(config.StateDir, "ftp-passwd.json")
}

func HasPersistedSync() bool {
	_, err := os.Stat(syncPath())
	return err == nil
}

func RestorePersisted() error {
	b, err := os.ReadFile(syncPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var p SyncPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	return applySync(p, false)
}

func Sync(p SyncPayload) error {
	return applySync(mergePasswords(p), true)
}

func mergePasswords(p SyncPayload) SyncPayload {
	b, err := os.ReadFile(syncPath())
	if err != nil {
		return p
	}
	var prev SyncPayload
	if json.Unmarshal(b, &prev) != nil {
		return p
	}
	prevPwd := make(map[string]string, len(prev.Users))
	for _, u := range prev.Users {
		if u.Password != "" {
			prevPwd[u.Username] = u.Password
		}
	}
	for i := range p.Users {
		if p.Users[i].Password == "" {
			p.Users[i].Password = prevPwd[p.Users[i].Username]
		}
	}
	return p
}

func applySync(p SyncPayload, persist bool) error {
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		return err
	}
	for _, u := range p.Users {
		if u.Samba {
			if err := ensureSambaUser(u.Username, u.Password); err != nil {
				return err
			}
		} else if sambaUserExists(u.Username) {
			if err := disableSambaUser(u.Username); err != nil {
				log.Printf("samba disable %s: %v", u.Username, err)
			}
		}
		if u.FTP {
			if err := ensureFTPUser(u.Username, u.Password); err != nil {
				return err
			}
		}
	}
	if err := writeFTPUsers(p.Users); err != nil {
		return err
	}
	if err := applyFilesystemACL(p.ACL); err != nil {
		return err
	}
	if persist {
		b, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(syncPath(), b, 0o600); err != nil {
			return err
		}
	}
	if err := shares.RefreshSamba(); err != nil {
		log.Printf("samba refresh: %v", err)
	}
	if err := shares.RefreshFTP(); err != nil {
		log.Printf("ftp refresh: %v", err)
	}
	return nil
}

func ensureSambaUser(username, password string) error {
	if username == "" {
		return fmt.Errorf("nom d'utilisateur vide")
	}
	_ = exec.Command("useradd", "-M", "-s", "/usr/sbin/nologin", username).Run()

	exists := sambaUserExists(username)
	if password == "" {
		if exists {
			if err := enableSambaUser(username); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("mot de passe requis pour activer Samba (%s)", username)
	}

	args := []string{"-s", username}
	if !exists {
		args = []string{"-a", "-s", username}
	}
	cmd := exec.Command("smbpasswd", args...)
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return enableSambaUser(username)
}

func sambaUserExists(username string) bool {
	out, err := exec.Command("pdbedit", "-L").Output()
	if err != nil {
		return false
	}
	prefix := username + ":"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func enableSambaUser(username string) error {
	cmd := exec.Command("smbpasswd", "-e", username)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd -e: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func disableSambaUser(username string) error {
	cmd := exec.Command("smbpasswd", "-d", username)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smbpasswd -d: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func ensureFTPUser(username, password string) error {
	if username == "" {
		return fmt.Errorf("nom d'utilisateur vide")
	}
	home := filepath.Join("/srv", username)
	_ = exec.Command("useradd", "-M", "-d", home, "-s", "/usr/sbin/nologin", username).Run()
	if unixUserExists(username) {
		_ = exec.Command("usermod", "-d", home, "-s", "/usr/sbin/nologin", username).Run()
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", home, err)
	}
	_ = exec.Command("chown", username+":"+username, home).Run()
	if password == "" {
		if unixUserExists(username) {
			return nil
		}
		return fmt.Errorf("mot de passe requis pour activer FTP (%s)", username)
	}
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(username + ":" + password + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chpasswd: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func unixUserExists(username string) bool {
	_, err := exec.Command("id", "-u", username).CombinedOutput()
	return err == nil
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

func applyFilesystemACL(acl []ACLRule) error {
	for _, a := range acl {
		if a.Username == "" || a.Path == "" {
			continue
		}
		if _, err := os.Stat(a.Path); err != nil {
			continue
		}
		if !unixUserExists(a.Username) {
			continue
		}
		entry := "u:" + a.Username
		if !a.CanRead {
			_ = exec.Command("setfacl", "-x", entry, a.Path).Run()
			_ = exec.Command("setfacl", "-d", "-x", entry, a.Path).Run()
			continue
		}
		perms := "r-x"
		if a.CanWrite {
			perms = "rwx"
		}
		spec := entry + ":" + perms
		if out, err := exec.Command("setfacl", "-m", spec, a.Path).CombinedOutput(); err != nil {
			log.Printf("setfacl %s on %s: %s", a.Username, a.Path, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command("setfacl", "-d", "-m", spec, a.Path).CombinedOutput(); err != nil {
			log.Printf("setfacl default %s on %s: %s", a.Username, a.Path, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
