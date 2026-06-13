// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
)

// Config holds all runtime settings for the panel.
type Config struct {
	// Addr is the host:port the HTTP API listens on, e.g. ":8080".
	Addr string

	// DBPath is the SQLite database file. Use ":memory:" for an
	// ephemeral in-process database (handy for tests).
	DBPath string

	// XrayAPIAddr is the host:port of the local Xray gRPC API. When empty
	// the panel uses a no-op Xray client and only stores users in the DB —
	// useful for development on a machine without Xray running.
	XrayAPIAddr string

	// InboundTag is the tag of the Xray inbound that users are added to.
	InboundTag string

	// Node describes how clients reach the VPN node; used to build the
	// vless:// subscription links.
	Node NodeConfig
}

// NodeConfig holds the public connection parameters of a VPN node.
type NodeConfig struct {
	Host        string // public host/IP that clients dial
	Port        int    // public port (usually 443)
	Flow        string // VLESS flow, e.g. "xtls-rprx-vision"
	RealityPBK  string // Reality public key (from `xray x25519`)
	RealitySNI  string // Reality serverName / SNI, e.g. "www.microsoft.com"
	RealitySID  string // Reality short id
	Fingerprint string // uTLS fingerprint, e.g. "chrome"
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Addr:        env("WISP_ADDR", ":8080"),
		DBPath:      env("WISP_DB", "wisp.db"),
		XrayAPIAddr: env("WISP_XRAY_API", ""),
		InboundTag:  env("WISP_INBOUND_TAG", "vless-reality"),
		Node: NodeConfig{
			Host:        env("WISP_NODE_HOST", "127.0.0.1"),
			Port:        envInt("WISP_NODE_PORT", 443),
			Flow:        env("WISP_NODE_FLOW", "xtls-rprx-vision"),
			RealityPBK:  env("WISP_REALITY_PBK", ""),
			RealitySNI:  env("WISP_REALITY_SNI", "www.microsoft.com"),
			RealitySID:  env("WISP_REALITY_SID", ""),
			Fingerprint: env("WISP_REALITY_FP", "chrome"),
		},
	}
}

// env returns the value of key, or def if the variable is unset or empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envInt returns the integer value of key, or def if unset or unparsable.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
