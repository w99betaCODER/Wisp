package model

import "time"

// Node is a VPN server running a Wisp node agent that the panel controls.
//
// The panel talks to the agent at Address over mTLS; the agent in turn drives
// the local Xray instance. Users are pushed to every enabled node.
type Node struct {
	ID         string    `json:"id"`          // panel-internal primary key
	Name       string    `json:"name"`        // human label, e.g. "nl-amsterdam-1"
	Address    string    `json:"address"`     // agent host:port (mTLS control), e.g. "1.2.3.4:8443"
	Protocol   string    `json:"protocol"`    // vless | vmess | trojan
	PublicHost string    `json:"public_host"` // host clients dial for VPN traffic
	PublicPort int       `json:"public_port"` // port clients dial (usually 443)
	Enabled    bool      `json:"enabled"`     // false = users are not pushed here
	CreatedAt  time.Time `json:"created_at"`  // when the node was registered
}
