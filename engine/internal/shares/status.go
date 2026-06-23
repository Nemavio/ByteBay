package shares

import (
	"os/exec"
	"strings"
)

type ShareService struct {
	Running bool `json:"running"`
	Enabled bool `json:"enabled"`
	Shares  int  `json:"shares"`
}

type ServicesSnapshot struct {
	Samba ShareService `json:"samba"`
	NFS   ShareService `json:"nfs"`
	FTP   ShareService `json:"ftp"`
}

func ServiceStatus() (*ServicesSnapshot, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	sambaEnabled, nfsEnabled, ftpEnabled := 0, 0, 0
	for _, s := range cfg.Samba {
		if s.Enabled {
			sambaEnabled++
		}
	}
	for _, s := range cfg.NFS {
		if s.Enabled {
			nfsEnabled++
		}
	}
	for _, s := range cfg.FTP {
		if s.Enabled {
			ftpEnabled++
		}
	}
	return &ServicesSnapshot{
		Samba: ShareService{
			Running: supervisorRunning("smbd"),
			Enabled: sambaEnabled > 0,
			Shares:  sambaEnabled,
		},
		NFS: ShareService{
			Running: supervisorRunning("ganesha"),
			Enabled: nfsEnabled > 0,
			Shares:  nfsEnabled,
		},
		FTP: ShareService{
			Running: supervisorRunning("vsftpd") || supervisorRunning("vsftpd-ipv6"),
			Enabled: ftpEnabled > 0,
			Shares:  ftpEnabled,
		},
	}, nil
}

func supervisorRunning(program string) bool {
	out, err := exec.Command("supervisorctl", "status", program).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "RUNNING")
}
