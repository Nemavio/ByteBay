package network

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	netplanDir  = "/etc/netplan"
	bytebayFile = "90-bytebay.yaml"
)

type netplanDoc struct {
	Network netplanNetwork `yaml:"network"`
}

type netplanNetwork struct {
	Version     int                       `yaml:"version"`
	Renderer    string                    `yaml:"renderer,omitempty"`
	Ethernets   map[string]netplanLink    `yaml:"ethernets,omitempty"`
	Bonds       map[string]netplanBond    `yaml:"bonds,omitempty"`
	Vlans       map[string]netplanVlan    `yaml:"vlans,omitempty"`
	Nameservers *netplanNS                `yaml:"nameservers,omitempty"`
}

type netplanNS struct {
	Addresses []string `yaml:"addresses,omitempty"`
}

type netplanLink struct {
	DHCP4       yamlValue    `yaml:"dhcp4,omitempty"`
	DHCP6       yamlValue    `yaml:"dhcp6,omitempty"`
	Addresses   []string     `yaml:"addresses,omitempty"`
	Routes      []netplanRoute `yaml:"routes,omitempty"`
	Nameservers *netplanNS   `yaml:"nameservers,omitempty"`
	MTU         int          `yaml:"mtu,omitempty"`
	Optional    bool         `yaml:"optional,omitempty"`
}

type netplanBond struct {
	Interfaces []string         `yaml:"interfaces"`
	Parameters netplanBondParams `yaml:"parameters,omitempty"`
	DHCP4      yamlValue        `yaml:"dhcp4,omitempty"`
	DHCP6      yamlValue        `yaml:"dhcp6,omitempty"`
	Addresses  []string         `yaml:"addresses,omitempty"`
	Routes     []netplanRoute   `yaml:"routes,omitempty"`
	Nameservers *netplanNS      `yaml:"nameservers,omitempty"`
	MTU        int              `yaml:"mtu,omitempty"`
}

type netplanBondParams struct {
	Mode               string `yaml:"mode,omitempty"`
	LacpRate           string `yaml:"lacp-rate,omitempty"`
	MiiMonitorInterval int    `yaml:"mii-monitor-interval,omitempty"`
}

type netplanVlan struct {
	ID        int          `yaml:"id"`
	Link      string       `yaml:"link"`
	DHCP4     yamlValue    `yaml:"dhcp4,omitempty"`
	DHCP6     yamlValue    `yaml:"dhcp6,omitempty"`
	Addresses []string     `yaml:"addresses,omitempty"`
	Routes    []netplanRoute `yaml:"routes,omitempty"`
	Nameservers *netplanNS `yaml:"nameservers,omitempty"`
	MTU       int          `yaml:"mtu,omitempty"`
}

type netplanRoute struct {
	To  string `yaml:"to"`
	Via string `yaml:"via,omitempty"`
}

// yamlValue accepts bool or "yes"/"no" in netplan files.
type yamlValue struct {
	Set   bool
	Value bool
}

func (v *yamlValue) UnmarshalYAML(n *yaml.Node) error {
	var b bool
	if n.Decode(&b) == nil {
		v.Set = true
		v.Value = b
		return nil
	}
	var s string
	if n.Decode(&s) == nil {
		v.Set = true
		v.Value = strings.EqualFold(s, "yes") || strings.EqualFold(s, "true")
		return nil
	}
	return nil
}

func (v yamlValue) MarshalYAML() (interface{}, error) {
	if !v.Set {
		return nil, nil
	}
	if v.Value {
		return true, nil
	}
	return false, nil
}

func loadNetplan() (Config, error) {
	cfg := Config{Renderer: "networkd", Connections: []Conn{}}
	entries, err := os.ReadDir(netplanDir)
	if err != nil {
		return cfg, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(netplanDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc netplanDoc
		if err := yaml.Unmarshal(b, &doc); err != nil {
			continue
		}
		mergeDoc(&cfg, doc)
	}
	if len(cfg.Connections) == 0 {
		ifaces, _ := listPhysicalIfaces()
		for _, iface := range ifaces {
			cfg.Connections = append(cfg.Connections, Conn{
				Name:       iface.Name,
				Type:       "ethernet",
				IPv4Method: "dhcp",
				IPv6Method: "dhcp",
			})
		}
	}
	return cfg, nil
}

func mergeDoc(cfg *Config, doc netplanDoc) {
	n := doc.Network
	if n.Renderer != "" {
		cfg.Renderer = n.Renderer
	}
	if n.Nameservers != nil && len(n.Nameservers.Addresses) > 0 {
		cfg.DNS = n.Nameservers.Addresses
	}
	for name, link := range n.Ethernets {
		cfg.Connections = appendOrReplace(cfg.Connections, linkToConn(name, "ethernet", link))
	}
	for name, bond := range n.Bonds {
		c := bondToConn(name, bond)
		cfg.Connections = appendOrReplace(cfg.Connections, c)
	}
	for name, vlan := range n.Vlans {
		c := vlanToConn(name, vlan)
		cfg.Connections = appendOrReplace(cfg.Connections, c)
	}
}

func appendOrReplace(list []Conn, c Conn) []Conn {
	for i, x := range list {
		if x.Name == c.Name {
			list[i] = c
			return list
		}
	}
	return append(list, c)
}

func linkToConn(name, typ string, l netplanLink) Conn {
	c := Conn{
		Name: name,
		Type: typ,
		MTU:  l.MTU,
		DNS:  nsAddrs(l.Nameservers),
	}
	setIPFromLink(&c, l.DHCP4, l.DHCP6, l.Addresses, l.Routes)
	return c
}

func bondToConn(name string, b netplanBond) Conn {
	c := Conn{
		Name:     name,
		Type:     "bond",
		Slaves:   b.Interfaces,
		BondMode: b.Parameters.Mode,
		MTU:      b.MTU,
		DNS:      nsAddrs(b.Nameservers),
	}
	if c.BondMode == "" {
		c.BondMode = "802.3ad"
	}
	setIPFromLink(&c, b.DHCP4, b.DHCP6, b.Addresses, b.Routes)
	return c
}

func vlanToConn(name string, v netplanVlan) Conn {
	c := Conn{
		Name:   name,
		Type:   "vlan",
		VlanID: v.ID,
		Parent: v.Link,
		MTU:    v.MTU,
		DNS:    nsAddrs(v.Nameservers),
	}
	setIPFromLink(&c, v.DHCP4, v.DHCP6, v.Addresses, v.Routes)
	return c
}

func setIPFromLink(c *Conn, dhcp4, dhcp6 yamlValue, addrs []string, routes []netplanRoute) {
	if dhcp4.Set && dhcp4.Value {
		c.IPv4Method = "dhcp"
	} else if len(addrs) > 0 {
		for _, a := range addrs {
			if strings.Contains(a, ":") {
				if c.IPv6Method != "dhcp" {
					c.IPv6Method = "static"
					c.IPv6Address = a
				}
			} else {
				c.IPv4Method = "static"
				c.IPv4Address = a
			}
		}
	} else {
		c.IPv4Method = "disabled"
	}
	if dhcp6.Set && dhcp6.Value {
		c.IPv6Method = "dhcp"
	} else if c.IPv6Method == "" {
		c.IPv6Method = "disabled"
	}
	for _, r := range routes {
		if r.To == "default" || r.To == "0.0.0.0/0" {
			if strings.Contains(r.Via, ":") {
				c.IPv6Gateway = r.Via
			} else {
				c.IPv4Gateway = r.Via
			}
		}
	}
}

func nsAddrs(ns *netplanNS) []string {
	if ns == nil {
		return nil
	}
	return ns.Addresses
}

func writeNetplan(cfg Config) error {
	doc := netplanDoc{Network: netplanNetwork{
		Version:  2,
		Renderer: cfg.Renderer,
		Ethernets: map[string]netplanLink{},
		Bonds:     map[string]netplanBond{},
		Vlans:     map[string]netplanVlan{},
	}}
	if len(cfg.DNS) > 0 {
		doc.Network.Nameservers = &netplanNS{Addresses: cfg.DNS}
	}
	for _, c := range cfg.Connections {
		typ := c.Type
		if typ == "" {
			typ = "ethernet"
		}
		switch typ {
		case "ethernet":
			doc.Network.Ethernets[c.Name] = connToLink(c)
		case "bond":
			for _, slave := range c.Slaves {
				if _, ok := doc.Network.Ethernets[slave]; !ok {
					doc.Network.Ethernets[slave] = netplanLink{Optional: true}
				}
			}
			doc.Network.Bonds[c.Name] = connToBond(c)
		case "vlan":
			doc.Network.Vlans[c.Name] = connToVlan(c)
		}
	}
	if len(doc.Network.Ethernets) == 0 {
		doc.Network.Ethernets = nil
	}
	if len(doc.Network.Bonds) == 0 {
		doc.Network.Bonds = nil
	}
	if len(doc.Network.Vlans) == 0 {
		doc.Network.Vlans = nil
	}
	b, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	header := "# Managed by ByteBay — do not edit manually\n"
	if err := os.MkdirAll(netplanDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(netplanDir, bytebayFile)
	return os.WriteFile(path, append([]byte(header), b...), 0o600)
}

func connToLink(c Conn) netplanLink {
	l := netplanLink{MTU: c.MTU}
	applyIP(&l.DHCP4, &l.DHCP6, &l.Addresses, &l.Routes, c)
	if len(c.DNS) > 0 {
		l.Nameservers = &netplanNS{Addresses: c.DNS}
	}
	return l
}

func connToBond(c Conn) netplanBond {
	b := netplanBond{
		Interfaces: c.Slaves,
		MTU:        c.MTU,
		Parameters: netplanBondParams{
			Mode:               c.BondMode,
			LacpRate:           "fast",
			MiiMonitorInterval: 100,
		},
	}
	applyIP(&b.DHCP4, &b.DHCP6, &b.Addresses, &b.Routes, c)
	if len(c.DNS) > 0 {
		b.Nameservers = &netplanNS{Addresses: c.DNS}
	}
	return b
}

func connToVlan(c Conn) netplanVlan {
	v := netplanVlan{ID: c.VlanID, Link: c.Parent, MTU: c.MTU}
	applyIP(&v.DHCP4, &v.DHCP6, &v.Addresses, &v.Routes, c)
	if len(c.DNS) > 0 {
		v.Nameservers = &netplanNS{Addresses: c.DNS}
	}
	return v
}

func applyIP(dhcp4, dhcp6 *yamlValue, addrs *[]string, routes *[]netplanRoute, c Conn) {
	switch c.IPv4Method {
	case "dhcp":
		*dhcp4 = yamlValue{Set: true, Value: true}
	case "static":
		*dhcp4 = yamlValue{Set: true, Value: false}
		if c.IPv4Address != "" {
			*addrs = append(*addrs, c.IPv4Address)
		}
		if c.IPv4Gateway != "" {
			*routes = append(*routes, netplanRoute{To: "default", Via: c.IPv4Gateway})
		}
	default:
		*dhcp4 = yamlValue{Set: true, Value: false}
	}
	switch c.IPv6Method {
	case "dhcp", "auto":
		*dhcp6 = yamlValue{Set: true, Value: true}
	case "static":
		*dhcp6 = yamlValue{Set: true, Value: false}
		if c.IPv6Address != "" {
			*addrs = append(*addrs, c.IPv6Address)
		}
		if c.IPv6Gateway != "" {
			*routes = append(*routes, netplanRoute{To: "default", Via: c.IPv6Gateway})
		}
	default:
		*dhcp6 = yamlValue{Set: true, Value: false}
	}
}
