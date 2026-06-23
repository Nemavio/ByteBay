package config

import (
	"os"
	"strconv"
)

const (
	DefaultSocket = "/run/bytebay/agent.sock"
	StateDir      = "/var/lib/bytebay"
)

func SocketGroup() string {
	if g := os.Getenv("BYTEBAY_SOCKET_GROUP"); g != "" {
		return g
	}
	return "bytebay"
}

func SmartIntervalSec() int {
	if v := os.Getenv("BYTEBAY_SMART_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 300
}
