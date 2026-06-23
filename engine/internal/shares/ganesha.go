package shares

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytebay/bytebay/engine/internal/config"
)

func writeGanesha(shares []NFSShare) error {
	useFilter := anyEnabledNFS(shares)
	if err := syncNFSMounts(shares, useFilter); err != nil {
		return err
	}

	var exports strings.Builder
	id := 1
	for _, s := range shares {
		if useFilter && !s.Enabled {
			continue
		}
		if s.Path == "" {
			continue
		}
		exportPath := resolvedNFSExport(s)
		clients := strings.TrimSpace(s.Clients)
		if clients == "" {
			clients = "*"
		}
		squash := ganeshaSquash(s.Options)
		pseudo := ganeshaPseudo(s, exportPath)
		exports.WriteString(fmt.Sprintf(`
EXPORT {
    Export_Id = %d;
    Path = "%s";
    Pseudo = "%s";
    Protocols = 3, 4;
    Access_Type = RW;
    Squash = %s;

    FSAL {
        Name = VFS;
    }

    CLIENT {
        Clients = %s;
        Access_Type = RW;
        Squash = %s;
        PrivilegedPort = false;
    }
}
`, id, exportPath, pseudo, squash, clients, squash))
		id++
	}

	conf := ganeshaBaseConfig() + exports.String()
	if err := os.MkdirAll(filepath.Dir(config.GaneshaConfigPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.GaneshaConfigPath, []byte(conf), 0o644)
}

func ganeshaPluginsDir() string {
	candidates := []string{
		"/usr/lib/aarch64-linux-gnu/ganesha",
		"/usr/lib/x86_64-linux-gnu/ganesha",
		"/usr/lib/ganesha",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "libfsalvfs.so")); err == nil {
			return dir
		}
	}
	return "/usr/lib/ganesha"
}

func ganeshaBaseConfig() string {
	return fmt.Sprintf(`NFS_CORE_PARAM {
    Plugins_Dir = "%s";
    NFS_Port = 2049;
    MNT_Port = 20048;
    mount_path_pseudo = true;
    Protocols = 3, 4;
}

NFSV4 {
    Grace_Period = 90;
    Lease_Lifetime = 60;
    RecoveryBackend = "fs";
}

EXPORT_DEFAULTS {
    Access_Type = RW;
    Squash = no_root_squash;
    Attr_Expiration_Time = 60;
}

`, ganeshaPluginsDir())
}

func ganeshaPseudo(s NFSShare, exportPath string) string {
	exp := strings.TrimSpace(s.Export)
	if exp != "" && !strings.Contains(exp, "/") {
		return "/" + exp
	}
	if strings.HasPrefix(exportPath, config.NFSExportRoot+"/") {
		return "/" + strings.TrimPrefix(exportPath, config.NFSExportRoot+"/")
	}
	base := filepath.Base(exportPath)
	if base == "" || base == "." {
		base = "export"
	}
	return "/" + base
}

func ganeshaSquash(opts string) string {
	o := strings.ToLower(opts)
	switch {
	case strings.Contains(o, "all_squash"):
		return "all_anonymous"
	case strings.Contains(o, "no_root_squash"):
		return "no_root_squash"
	case strings.Contains(o, "root_squash"):
		return "root_squash"
	default:
		return "no_root_squash"
	}
}
