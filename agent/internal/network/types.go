package network

type Status struct {
	Interfaces  []Iface  `json:"interfaces"`
	Connections []Conn   `json:"connections"`
	DNS         []string `json:"dns"`
	Renderer    string   `json:"renderer"`
}

type Iface struct {
	Name  string `json:"name"`
	MAC   string `json:"mac"`
	State string `json:"state"`
	Speed string `json:"speed,omitempty"`
	Master string `json:"master,omitempty"`
}

type Conn struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // ethernet, bond, vlan
	IPv4Method  string   `json:"ipv4_method"` // dhcp, static, disabled
	IPv4Address string   `json:"ipv4_address,omitempty"`
	IPv4Gateway string   `json:"ipv4_gateway,omitempty"`
	IPv6Method  string   `json:"ipv6_method"` // dhcp, static, auto, disabled
	IPv6Address string   `json:"ipv6_address,omitempty"`
	IPv6Gateway string   `json:"ipv6_gateway,omitempty"`
	DNS         []string `json:"dns,omitempty"`
	MTU         int      `json:"mtu,omitempty"`
	// bond
	BondMode string   `json:"bond_mode,omitempty"`
	Slaves   []string `json:"slaves,omitempty"`
	// vlan
	VlanID int    `json:"vlan_id,omitempty"`
	Parent string `json:"parent,omitempty"`
	// live
	Addresses []string `json:"addresses,omitempty"`
	OperState string   `json:"oper_state,omitempty"`
}

type Config struct {
	Renderer    string `json:"renderer"`
	DNS         []string `json:"dns"`
	Connections []Conn `json:"connections"`
}
