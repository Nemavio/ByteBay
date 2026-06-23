package shares

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytebay/bytebay/engine/internal/config"
	"github.com/bytebay/bytebay/engine/internal/services"
)

type Config struct {
	NFS   []NFSShare   `json:"nfs"`
	Samba []SambaShare `json:"samba"`
	FTP   []FTPShare   `json:"ftp"`
}

type NFSShare struct {
	Path    string `json:"path"`
	Export  string `json:"export,omitempty"`
	Clients string `json:"clients"`
	Options string `json:"options"`
	Enabled bool   `json:"enabled"`
}

type SambaShare struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Browseable bool   `json:"browseable"`
	ReadOnly   bool   `json:"read_only"`
	GuestOK    bool   `json:"guest_ok"`
	Enabled    bool   `json:"enabled"`
}

type FTPShare struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

type ApplyResult struct {
	NFS   string `json:"nfs"`
	Samba string `json:"samba"`
	FTP   string `json:"ftp"`
}

func statePath() string {
	return filepath.Join(config.SharesConfigDir, "shares.json")
}

func Load() (*Config, error) {
	b, err := os.ReadFile(statePath())
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func save(c *Config) error {
	if err := os.MkdirAll(config.SharesConfigDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(statePath(), b, 0o644); err != nil {
		return err
	}
	return apply(c)
}

func Update(kind string, raw json.RawMessage) (*Config, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	switch kind {
	case "nfs":
		if err := json.Unmarshal(raw, &c.NFS); err != nil {
			return nil, err
		}
	case "samba":
		if err := json.Unmarshal(raw, &c.Samba); err != nil {
			return nil, err
		}
	case "ftp":
		if err := json.Unmarshal(raw, &c.FTP); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown share kind: %s", kind)
	}
	if err := validate(c); err != nil {
		return nil, err
	}
	if err := save(c); err != nil {
		return nil, err
	}
	return c, nil
}

func Reapply() (*ApplyResult, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	if err := apply(c); err != nil {
		return nil, err
	}
	return reloadAll()
}

// RestorePersisted réécrit exports/partages depuis shares.json au démarrage.
func RestorePersisted() error {
	if _, err := os.Stat(statePath()); os.IsNotExist(err) {
		return nil
	}
	c, err := Load()
	if err != nil {
		return err
	}
	return apply(c)
}

func apply(c *Config) error {
	if err := writeNFS(c.NFS); err != nil {
		return err
	}
	if err := writeSamba(c.Samba); err != nil {
		return err
	}
	if err := writeFTP(c.FTP); err != nil {
		return err
	}
	_, err := reloadAll()
	return err
}

func reloadAll() (*ApplyResult, error) {
	return &ApplyResult{
		NFS:   services.ReloadNFS(),
		Samba: services.ReloadSamba(),
		FTP:   services.ReloadFTP(),
	}, nil
}

func validate(c *Config) error {
	for _, s := range c.NFS {
		if err := ensurePath(s.Path); err != nil {
			return err
		}
		if err := validateNFSExport(s.Export); err != nil {
			return err
		}
	}
	seenNFSExport := make(map[string]bool)
	for _, s := range c.NFS {
		exp := resolvedNFSExport(s)
		if seenNFSExport[exp] {
			return fmt.Errorf("duplicate NFS export path: %s", exp)
		}
		seenNFSExport[exp] = true
	}
	for _, s := range c.Samba {
		if err := ensurePath(s.Path); err != nil {
			return err
		}
	}
	for _, s := range c.FTP {
		if err := ensurePath(s.Path); err != nil {
			return err
		}
	}
	return nil
}

func ensurePath(p string) error {
	if p == "" {
		return nil
	}
	if !strings.HasPrefix(p, config.DataRoot) && !strings.HasPrefix(p, config.VolumesRoot) {
		return fmt.Errorf("path must be under %s or %s: %s", config.DataRoot, config.VolumesRoot, p)
	}
	return os.MkdirAll(p, 0o755)
}

var nfsExportNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func validateNFSExport(exp string) error {
	exp = strings.TrimSpace(exp)
	if exp == "" {
		return nil
	}
	if strings.Contains(exp, "/") {
		clean := filepath.Clean(exp)
		if !strings.HasPrefix(clean, config.NFSExportRoot+"/") {
			return fmt.Errorf("export NFS must be a short name (e.g. backup) or an absolute path under %s", config.NFSExportRoot)
		}
		return nil
	}
	if !nfsExportNameRe.MatchString(exp) {
		return fmt.Errorf("invalid NFS export name %q: use letters, digits, - or _", exp)
	}
	return nil
}

func resolvedNFSExport(s NFSShare) string {
	exp := strings.TrimSpace(s.Export)
	if exp == "" {
		return s.Path
	}
	if strings.HasPrefix(exp, "/") {
		return filepath.Clean(exp)
	}
	return filepath.Join(config.NFSExportRoot, exp)
}

func syncNFSMounts(shares []NFSShare, useFilter bool) error {
	if err := os.MkdirAll(config.NFSExportRoot, 0o755); err != nil {
		return err
	}
	active := make(map[string]bool)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Path == "" {
			continue
		}
		exportPath := resolvedNFSExport(s)
		if exportPath == s.Path {
			continue
		}
		active[exportPath] = true
		if err := os.MkdirAll(exportPath, 0o755); err != nil {
			return err
		}
		if err := bindMount(s.Path, exportPath); err != nil {
			log.Printf("nfs bind %s -> %s: %v", s.Path, exportPath, err)
		}
	}
	entries, err := os.ReadDir(config.NFSExportRoot)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(config.NFSExportRoot, e.Name())
		if active[p] {
			continue
		}
		_ = exec.Command("umount", "-l", p).Run()
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("nfs cleanup %s: %v", p, err)
		}
	}
	return nil
}

func writeNFS(shares []NFSShare) error {
	return writeGanesha(shares)
}

type syncUser struct {
	Username string `json:"username"`
	Samba    bool   `json:"samba"`
	FTP      bool   `json:"ftp"`
}

type syncACL struct {
	Path     string `json:"path"`
	Username string `json:"username"`
	CanRead  bool   `json:"can_read"`
	CanWrite bool   `json:"can_write"`
}

type syncData struct {
	Users []syncUser `json:"users"`
	ACL   []syncACL  `json:"acl"`
}

func loadSyncData() syncData {
	b, err := os.ReadFile(filepath.Join(config.StateDir, "sync.json"))
	if err != nil {
		return syncData{}
	}
	var d syncData
	if json.Unmarshal(b, &d) != nil {
		return syncData{}
	}
	return d
}

func aclCoversPath(aclPath, target string) bool {
	aclPath = strings.TrimSuffix(aclPath, "/")
	target = strings.TrimSuffix(target, "/")
	return target == aclPath || strings.HasPrefix(target, aclPath+"/")
}

func validUsersForShare(sharePath string, d syncData) string {
	sambaUsers := make(map[string]bool)
	for _, u := range d.Users {
		if u.Samba && u.Username != "" {
			sambaUsers[u.Username] = true
		}
	}
	var names []string
	seen := make(map[string]bool)
	for _, a := range d.ACL {
		if !a.CanRead || a.Username == "" || !sambaUsers[a.Username] {
			continue
		}
		if !aclCoversPath(a.Path, sharePath) {
			continue
		}
		if !seen[a.Username] {
			seen[a.Username] = true
			names = append(names, a.Username)
		}
	}
	return strings.Join(names, " ")
}

func writeListForShare(sharePath string, d syncData) string {
	sambaUsers := make(map[string]bool)
	for _, u := range d.Users {
		if u.Samba && u.Username != "" {
			sambaUsers[u.Username] = true
		}
	}
	var names []string
	seen := make(map[string]bool)
	for _, a := range d.ACL {
		if !a.CanRead || !a.CanWrite || a.Username == "" || !sambaUsers[a.Username] {
			continue
		}
		if !aclCoversPath(a.Path, sharePath) {
			continue
		}
		if !seen[a.Username] {
			seen[a.Username] = true
			names = append(names, a.Username)
		}
	}
	return strings.Join(names, " ")
}

// RefreshSamba réécrit bytebay.conf depuis shares.json et recharge smbd (ex. après sync ACL).
func RefreshSamba() error {
	c, err := Load()
	if err != nil {
		return err
	}
	if err := writeSamba(c.Samba); err != nil {
		return err
	}
	services.ReloadSamba()
	return nil
}

func writeSamba(shares []SambaShare) error {
	sync := loadSyncData()
	var b strings.Builder
	useFilter := anyEnabledSamba(shares)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Name == "" || s.Path == "" {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\n  path = %s\n  browseable = %s\n",
			s.Name, s.Path, yesNo(s.Browseable))
		writers := writeListForShare(s.Path, sync)
		if s.ReadOnly || writers == "" {
			fmt.Fprintf(&b, "  read only = yes\n")
		} else if writers == validUsersForShare(s.Path, sync) {
			fmt.Fprintf(&b, "  read only = no\n")
		} else {
			fmt.Fprintf(&b, "  read only = yes\n  write list = %s\n", writers)
		}
		if s.GuestOK {
			fmt.Fprintf(&b, "  guest ok = yes\n")
		} else {
			fmt.Fprintf(&b, "  guest ok = no\n")
			if users := validUsersForShare(s.Path, sync); users != "" {
				fmt.Fprintf(&b, "  valid users = %s\n", users)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(config.SambaIncludePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.SambaIncludePath, []byte(b.String()), 0o644)
}

// RefreshFTP réécrit les configs vsftpd par utilisateur et recharge le service.
func RefreshFTP() error {
	c, err := Load()
	if err != nil {
		return err
	}
	if err := writeFTP(c.FTP); err != nil {
		return err
	}
	services.ReloadFTP()
	return nil
}

func ftpEnabledUsers(d syncData) map[string]bool {
	out := make(map[string]bool)
	for _, u := range d.Users {
		if u.FTP && u.Username != "" {
			out[u.Username] = true
		}
	}
	return out
}

type ftpRoot struct {
	Name string
	Path string
}

func userHasFTPACL(username, target string, d syncData) bool {
	for _, a := range d.ACL {
		if a.Username != username || !a.CanRead {
			continue
		}
		if aclCoversPath(a.Path, target) {
			return true
		}
	}
	return false
}

func ftpRootsForUser(username string, shares []FTPShare, d syncData, useFilter bool) []ftpRoot {
	var roots []ftpRoot
	seenName := make(map[string]bool)
	seenPath := make(map[string]bool)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Path == "" || !userHasFTPACL(username, s.Path, d) {
			continue
		}
		name := s.Name
		if name == "" || name == username {
			name = filepath.Base(strings.TrimSuffix(s.Path, "/"))
		}
		if name == "" || name == "." {
			name = "partage"
		}
		baseName := name
		for i := 2; seenName[name]; i++ {
			name = fmt.Sprintf("%s-%d", baseName, i)
		}
		if seenPath[s.Path] {
			continue
		}
		seenName[name] = true
		seenPath[s.Path] = true
		roots = append(roots, ftpRoot{Name: name, Path: s.Path})
	}
	return roots
}

func bindMount(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source %s: %w", src, err)
	}
	_ = exec.Command("umount", "-l", dst).Run()
	cmd := exec.Command("mount", "--bind", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "already mounted") || strings.Contains(msg, "busy") {
			return nil
		}
		return fmt.Errorf("mount --bind %s -> %s: %s", src, dst, msg)
	}
	return nil
}

func syncFTPHome(username string, roots []ftpRoot) error {
	home := filepath.Join("/srv", username)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	_ = exec.Command("chown", username+":"+username, home).Run()

	active := make(map[string]bool)
	for _, r := range roots {
		active[r.Name] = true
		mountPoint := filepath.Join(home, r.Name)
		if err := os.MkdirAll(mountPoint, 0o755); err != nil {
			return err
		}
		if err := bindMount(r.Path, mountPoint); err != nil {
			log.Printf("ftp: %v", err)
		}
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if active[e.Name()] {
			continue
		}
		p := filepath.Join(home, e.Name())
		_ = exec.Command("umount", "-l", p).Run()
		if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
			log.Printf("ftp cleanup %s: %v", p, err)
		}
	}
	return nil
}

func writeFTP(shares []FTPShare) error {
	if err := os.MkdirAll(config.VsftpdUserDir, 0o755); err != nil {
		return err
	}

	sync := loadSyncData()
	ftpUsers := ftpEnabledUsers(sync)
	useFilter := anyEnabledFTP(shares)
	active := make(map[string]bool)
	for username := range ftpUsers {
		roots := ftpRootsForUser(username, shares, sync, useFilter)
		if len(roots) == 0 {
			continue
		}
		if err := syncFTPHome(username, roots); err != nil {
			log.Printf("ftp home %s: %v", username, err)
		}
		active[username] = true
		home := filepath.Join("/srv", username)
		userFile := filepath.Join(config.VsftpdUserDir, username)
		cfg := fmt.Sprintf("local_root=%s\nallow_writeable_chroot=YES\n", home)
		if err := os.WriteFile(userFile, []byte(cfg), 0o644); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(config.VsftpdUserDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || active[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(config.VsftpdUserDir, e.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func anyEnabledNFS(shares []NFSShare) bool {
	for _, s := range shares {
		if s.Enabled {
			return true
		}
	}
	return false
}

func anyEnabledSamba(shares []SambaShare) bool {
	for _, s := range shares {
		if s.Enabled {
			return true
		}
	}
	return false
}

func anyEnabledFTP(shares []FTPShare) bool {
	for _, s := range shares {
		if s.Enabled {
			return true
		}
	}
	return false
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
