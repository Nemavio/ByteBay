package shares

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	if !strings.HasPrefix(p, config.DataRoot) {
		return fmt.Errorf("path must be under %s: %s", config.DataRoot, p)
	}
	return os.MkdirAll(p, 0o755)
}

func writeNFS(shares []NFSShare) error {
	var lines string
	useFilter := anyEnabledNFS(shares)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Path == "" {
			continue
		}
		opts := s.Options
		if opts == "" {
			opts = "rw,sync,no_subtree_check,no_root_squash,fsid=0"
		}
		clients := s.Clients
		if clients == "" {
			clients = "*"
		}
		lines += fmt.Sprintf("%s %s(%s)\n", s.Path, clients, opts)
	}
	if err := os.MkdirAll(filepath.Dir(config.NFSExportsPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.NFSExportsPath, []byte(lines), 0o644)
}

func writeSamba(shares []SambaShare) error {
	var b strings.Builder
	useFilter := anyEnabledSamba(shares)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Name == "" || s.Path == "" {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\n  path = %s\n  browseable = %s\n  read only = %s\n  guest ok = %s\n",
			s.Name, s.Path, yesNo(s.Browseable), yesNo(s.ReadOnly), yesNo(s.GuestOK))
	}
	if err := os.MkdirAll(filepath.Dir(config.SambaIncludePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.SambaIncludePath, []byte(b.String()), 0o644)
}

func writeFTP(shares []FTPShare) error {
	if err := os.MkdirAll(config.VsftpdUserDir, 0o755); err != nil {
		return err
	}
	var mainCfg strings.Builder
	mainCfg.WriteString("# ByteBay engine\n")
	mainCfg.WriteString("user_config_dir=" + config.VsftpdUserDir + "\n")

	useFilter := anyEnabledFTP(shares)
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Name == "" || s.Path == "" {
			continue
		}
		userFile := filepath.Join(config.VsftpdUserDir, s.Name)
		cfg := fmt.Sprintf("local_root=%s\nallow_writeable_chroot=YES\n", s.Path)
		if err := os.WriteFile(userFile, []byte(cfg), 0o644); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(config.VsftpdConfigPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.VsftpdConfigPath, []byte(mainCfg.String()), 0o644)
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
