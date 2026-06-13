// Package subscription turns a user + node into the share links that VPN
// clients import (the "subscription" you paste into v2rayN, Streisand, etc.).
package subscription

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
)

// Link builds the share link for a user on a node, choosing the format from the
// node's protocol. reality supplies the shared Reality parameters used by VLESS
// nodes (per-node Reality keys are a future enhancement).
func Link(u model.User, node model.Node, reality config.NodeConfig) string {
	host, port := node.PublicHost, node.PublicPort
	switch node.Protocol {
	case "vmess":
		return vmessLink(u, host, port)
	case "trojan":
		return trojanLink(u, host, port)
	default: // vless
		return vlessURL(u.UUID, host, port, u.Email, reality)
	}
}

// VLESSLink builds a vless:// (Reality) link from the global node config. Used
// as the single-server fallback when no nodes are registered.
func VLESSLink(u model.User, node config.NodeConfig) string {
	return vlessURL(u.UUID, node.Host, node.Port, u.Email, node)
}

// vlessURL renders vless://<uuid>@<host>:<port>?<reality params>#<label>.
func vlessURL(uuid, host string, port int, label string, r config.NodeConfig) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("security", "reality")
	q.Set("type", "tcp")
	q.Set("sni", r.RealitySNI)
	q.Set("fp", r.Fingerprint)
	q.Set("pbk", r.RealityPBK)
	if r.RealitySID != "" {
		q.Set("sid", r.RealitySID)
	}
	if r.Flow != "" {
		q.Set("flow", r.Flow)
	}
	link := url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: q.Encode(),
		Fragment: label,
	}
	return link.String()
}

// vmessLink renders a vmess:// link as base64-encoded JSON (the v2rayN format).
func vmessLink(u model.User, host string, port int) string {
	cfg := map[string]string{
		"v": "2", "ps": u.Email, "add": host, "port": strconv.Itoa(port),
		"id": u.UUID, "aid": "0", "scy": "auto", "net": "tcp", "type": "none",
		"host": "", "path": "", "tls": "tls", "sni": host,
	}
	b, _ := json.Marshal(cfg)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

// trojanLink renders trojan://<password>@<host>:<port>?security=tls#<label>.
func trojanLink(u model.User, host string, port int) string {
	q := url.Values{}
	q.Set("security", "tls")
	q.Set("type", "tcp")
	q.Set("sni", host)
	link := url.URL{
		Scheme:   "trojan",
		User:     url.User(u.UUID), // the UUID doubles as the Trojan password
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		RawQuery: q.Encode(),
		Fragment: u.Email,
	}
	return link.String()
}

// Encode renders a list of share links as subscription content: the links
// joined by newlines and base64-encoded, which every major client understands.
func Encode(links []string) string {
	joined := strings.Join(links, "\n")
	return base64.StdEncoding.EncodeToString([]byte(joined))
}
