package subscription

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
)

func testNode() config.NodeConfig {
	return config.NodeConfig{
		Host:        "vpn.example.com",
		Port:        443,
		Flow:        "xtls-rprx-vision",
		RealityPBK:  "PUBLICKEY",
		RealitySNI:  "www.microsoft.com",
		RealitySID:  "abcd",
		Fingerprint: "chrome",
	}
}

func TestVLESSLink(t *testing.T) {
	u := model.User{UUID: "uuid-1234", Email: "alice"}
	link := VLESSLink(u, testNode())

	if !strings.HasPrefix(link, "vless://uuid-1234@vpn.example.com:443?") {
		t.Fatalf("unexpected prefix: %s", link)
	}

	// Parse it back and assert the security-relevant params survived.
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("link is not a valid URL: %v", err)
	}
	q := parsed.Query()
	for k, want := range map[string]string{
		"security": "reality",
		"pbk":      "PUBLICKEY",
		"sni":      "www.microsoft.com",
		"sid":      "abcd",
		"flow":     "xtls-rprx-vision",
		"fp":       "chrome",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("param %q = %q, want %q", k, got, want)
		}
	}
	if parsed.Fragment != "alice" {
		t.Errorf("fragment = %q, want alice", parsed.Fragment)
	}
}

func TestLink_PerProtocol(t *testing.T) {
	u := model.User{UUID: "uuid-1", Email: "alice"}
	node := func(proto string) model.Node {
		return model.Node{Protocol: proto, PublicHost: "vpn.example.com", PublicPort: 443}
	}
	r := testNode()

	if l := Link(u, node("vless"), r); !strings.HasPrefix(l, "vless://uuid-1@vpn.example.com:443?") {
		t.Fatalf("vless link: %s", l)
	}
	if l := Link(u, node("trojan"), r); !strings.HasPrefix(l, "trojan://uuid-1@vpn.example.com:443?") {
		t.Fatalf("trojan link: %s", l)
	}

	vmess := Link(u, node("vmess"), r)
	if !strings.HasPrefix(vmess, "vmess://") {
		t.Fatalf("vmess prefix: %s", vmess)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(vmess, "vmess://"))
	if err != nil {
		t.Fatalf("vmess base64: %v", err)
	}
	if !strings.Contains(string(raw), `"id":"uuid-1"`) {
		t.Fatalf("vmess json missing id: %s", raw)
	}
}

func TestEncode(t *testing.T) {
	links := []string{"vless://a", "vless://b"}
	enc := Encode(links)

	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("output is not valid base64: %v", err)
	}
	if string(raw) != "vless://a\nvless://b" {
		t.Fatalf("decoded = %q", raw)
	}
}
