package network

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetStatus() (*Status, error) {
	cfg, err := loadNetplan()
	if err != nil {
		return nil, err
	}
	ifaces, err := listPhysicalIfaces()
	if err != nil {
		return nil, err
	}
	live := liveIPMap()
	for i := range cfg.Connections {
		if addrs, ok := live[cfg.Connections[i].Name]; ok {
			cfg.Connections[i].Addresses = addrs.Addrs
			cfg.Connections[i].OperState = addrs.OperState
		}
	}
	return &Status{
		Interfaces:  ifaces,
		Connections: cfg.Connections,
		DNS:         cfg.DNS,
		Renderer:    cfg.Renderer,
	}, nil
}

func Apply(cfg Config) error {
	if cfg.Renderer == "" {
		cfg.Renderer = "networkd"
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := writeNetplan(cfg); err != nil {
		return err
	}
	return runNetplanApply()
}

func Reapply() error {
	return runNetplanApply()
}

func listPhysicalIfaces() ([]Iface, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, err
	}
	var out []Iface
	for _, e := range entries {
		name := e.Name()
		if skipIface(name) {
			continue
		}
		iface := Iface{Name: name}
		iface.MAC = readTrim(filepath.Join("/sys/class/net", name, "address"))
		iface.State = readTrim(filepath.Join("/sys/class/net", name, "operstate"))
		iface.Speed = readTrim(filepath.Join("/sys/class/net", name, "speed"))
		if master, err := os.Readlink(filepath.Join("/sys/class/net", name, "master")); err == nil {
			iface.Master = filepath.Base(master)
		}
		out = append(out, iface)
	}
	return out, nil
}

func skipIface(name string) bool {
	if name == "lo" {
		return true
	}
	prefixes := []string{"docker", "br-", "veth", "virbr", "tun", "tap", "wg", "dummy"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return strings.Contains(name, "@")
}

type liveInfo struct {
	Addrs     []string
	OperState string
}

func liveIPMap() map[string]liveInfo {
	out := make(map[string]liveInfo)
	raw, err := exec.Command("ip", "-j", "addr", "show").Output()
	if err != nil {
		return out
	}
	var entries []struct {
		IfName   string `json:"ifname"`
		OperState string `json:"operstate"`
		AddrInfo []struct {
			Family  string `json:"family"`
			Local   string `json:"local"`
			Prefixlen int `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if json.Unmarshal(raw, &entries) != nil {
		return out
	}
	for _, e := range entries {
		info := liveInfo{OperState: e.OperState}
		for _, a := range e.AddrInfo {
			if a.Family == "inet" || a.Family == "inet6" {
				if a.Local != "" {
					info.Addrs = append(info.Addrs, fmt.Sprintf("%s/%d", a.Local, a.Prefixlen))
				}
			}
		}
		out[e.IfName] = info
	}
	return out
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func validateConfig(cfg Config) error {
	names := make(map[string]bool)
	for _, c := range cfg.Connections {
		if c.Name == "" {
			return fmt.Errorf("connection name required")
		}
		if names[c.Name] {
			return fmt.Errorf("duplicate connection %q", c.Name)
		}
		names[c.Name] = true
		switch c.Type {
		case "ethernet", "":
		case "bond":
			if len(c.Slaves) < 1 {
				return fmt.Errorf("bond %s needs at least one slave", c.Name)
			}
			if c.BondMode == "" {
				return fmt.Errorf("bond %s needs bond_mode", c.Name)
			}
		case "vlan":
			if c.VlanID < 1 || c.VlanID > 4094 {
				return fmt.Errorf("vlan %s: invalid vlan_id", c.Name)
			}
			if c.Parent == "" {
				return fmt.Errorf("vlan %s needs parent interface", c.Name)
			}
		default:
			return fmt.Errorf("unknown connection type %q", c.Type)
		}
		if c.IPv4Method == "static" && c.IPv4Address == "" {
			return fmt.Errorf("%s: ipv4_address required for static", c.Name)
		}
		if c.IPv6Method == "static" && c.IPv6Address == "" {
			return fmt.Errorf("%s: ipv6_address required for static", c.Name)
		}
	}
	return nil
}

func runNetplanApply() error {
	out, err := exec.Command("netplan", "generate").CombinedOutput()
	if err != nil {
		return fmt.Errorf("netplan generate: %s: %w", strings.TrimSpace(string(out)), err)
	}
	out, err = exec.Command("netplan", "apply").CombinedOutput()
	if err != nil {
		return fmt.Errorf("netplan apply: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
