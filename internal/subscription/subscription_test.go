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
