// Package subscription turns a user + node into the share links that VPN
// clients import (the "subscription" you paste into v2rayN, Streisand, etc.).
package subscription

import (
	"encoding/base64"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
)

// VLESSLink builds a vless:// share link for a user connecting to node.
//
// Shape: vless://<uuid>@<host>:<port>?<params>#<label>
// The params encode VLESS over TCP with Reality, matching what the Xray
// inbound on the node expects.
func VLESSLink(u model.User, node config.NodeConfig) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("security", "reality")
	q.Set("type", "tcp")
	q.Set("sni", node.RealitySNI)
	q.Set("fp", node.Fingerprint)
	q.Set("pbk", node.RealityPBK)
	if node.RealitySID != "" {
		q.Set("sid", node.RealitySID)
	}
	if node.Flow != "" {
		q.Set("flow", node.Flow)
	}

	link := url.URL{
		Scheme:   "vless",
		User:     url.User(u.UUID),
		Host:     net.JoinHostPort(node.Host, strconv.Itoa(node.Port)),
		RawQuery: q.Encode(),
		Fragment: u.Email, // shown as the connection name in the client
	}
	return link.String()
}

// Encode renders a list of share links as subscription content: the links
// joined by newlines and base64-encoded, which is the de-facto format every
// major client understands.
func Encode(links []string) string {
	joined := strings.Join(links, "\n")
	return base64.StdEncoding.EncodeToString([]byte(joined))
}
