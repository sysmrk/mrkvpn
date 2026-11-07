package wgconfig

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKey(v byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{v}, 32))
}

func validConfig(t *testing.T) Gateway {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.key")
	if err := os.WriteFile(path, []byte(testKey(1)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return Gateway{
		InterfaceName:  "wg0",
		Address:        "10.77.0.1/24",
		ListenPort:     51820,
		PrivateKeyFile: path,
		Peers:          []Peer{{Name: "laptop", PublicKey: testKey(2), Address: "10.77.0.2/32"}},
	}
}

func TestRenderIsStrictAndDeterministic(t *testing.T) {
	c := validConfig(t)
	b, err := Render(c)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, expected := range []string{
		"[Interface]", "Address = 10.77.0.1/24", "ListenPort = 51820",
		"PrivateKey = " + testKey(1), "SaveConfig = false",
		"# Device: laptop", "[Peer]", "PublicKey = " + testKey(2),
		"AllowedIPs = 10.77.0.2/32",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("render missing expected content %q", expected)
		}
	}
	if strings.Contains(got, "PostUp") || strings.Contains(got, "0.0.0.0/0") {
		t.Fatal("unexpected network mutations or wildcard peer route")
	}
	b2, err := Render(c)
	if err != nil || !bytes.Equal(b, b2) {
		t.Fatal("render not deterministic")
	}
}

func TestRejectsInvalidGatewayAndPeers(t *testing.T) {
	base := validConfig(t)
	tests := []struct {
		name string
		mod  func(*Gateway)
	}{
		{"bad interface", func(c *Gateway) { c.InterfaceName = "wg0\nPostUp=evil" }},
		{"too long interface", func(c *Gateway) { c.InterfaceName = strings.Repeat("a", 16) }},
		{"missing port", func(c *Gateway) { c.ListenPort = 0 }},
		{"invalid port", func(c *Gateway) { c.ListenPort = 65536 }},
		{"ipv6", func(c *Gateway) { c.Address = "fd00::1/64" }},
		{"public address", func(c *Gateway) { c.Address = "8.8.8.1/24" }},
		{"server network address", func(c *Gateway) { c.Address = "10.77.0.0/24" }},
		{"server broadcast address", func(c *Gateway) { c.Address = "10.77.0.255/24" }},
		{"oversized network", func(c *Gateway) { c.Address = "10.0.0.1/8" }},
		{"peer name injection", func(c *Gateway) { c.Peers[0].Name = "bad\n[Interface]" }},
		{"bad peer key", func(c *Gateway) { c.Peers[0].PublicKey = "not-a-key" }},
		{"ipv6 peer", func(c *Gateway) { c.Peers[0].Address = "fd00::2/128" }},
		{"peer outside range", func(c *Gateway) { c.Peers[0].Address = "10.78.0.2/32" }},
		{"peer wrong mask", func(c *Gateway) { c.Peers[0].Address = "10.77.0.2/24" }},
		{"peer network address", func(c *Gateway) { c.Peers[0].Address = "10.77.0.0/32" }},
		{"peer broadcast address", func(c *Gateway) { c.Peers[0].Address = "10.77.0.255/32" }},
		{"peer same as server", func(c *Gateway) { c.Peers[0].Address = "10.77.0.1/32" }},
		{"duplicate peer IP", func(c *Gateway) { c.Peers = append(c.Peers, Peer{Name: "phone", PublicKey: testKey(3), Address: "10.77.0.2/32"}) }},
		{"duplicate public key", func(c *Gateway) { c.Peers = append(c.Peers, Peer{Name: "phone", PublicKey: testKey(2), Address: "10.77.0.3/32"}) }},
		{"duplicate peer name", func(c *Gateway) { c.Peers = append(c.Peers, Peer{Name: "laptop", PublicKey: testKey(3), Address: "10.77.0.3/32"}) }},
		{"relative private key path", func(c *Gateway) { c.PrivateKeyFile = "gateway.key" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			c.Peers = append([]Peer(nil), base.Peers...)
			tc.mod(&c)
			if err := Validate(c); err == nil {
				t.Fatalf("accepted invalid configuration: %+v", c)
			}
		})
	}
}

func TestRejectsInsecurePrivateKeyFiles(t *testing.T) {
	c := validConfig(t)
	if err := os.Chmod(c.PrivateKeyFile, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(c); err == nil {
		t.Fatal("world-readable private key accepted")
	}
	if err := os.Chmod(c.PrivateKeyFile, 0600); err != nil {
		t.Fatal(err)
	}
	sym := filepath.Join(t.TempDir(), "key-link")
	if err := os.Symlink(c.PrivateKeyFile, sym); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	c.PrivateKeyFile = sym
	if err := Validate(c); err == nil {
		t.Fatal("symlink private key accepted")
	}
}

func TestRejectsMalformedPrivateKeyWithoutLeakingIt(t *testing.T) {
	c := validConfig(t)
	const secret = "do-not-print-my-broken-key"
	if err := os.WriteFile(c.PrivateKeyFile, []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	err := Validate(c)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("private key validation error leaked or missing: %v", err)
	}
}

func TestWriteNewNeverOverwritesAndUses0600(t *testing.T) {
	p := filepath.Join(t.TempDir(), "wg0.conf")
	if err := WriteNew(p, []byte("first")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("output not mode 0600: %v %v", info, err)
	}
	if err := WriteNew(p, []byte("second")); err == nil {
		t.Fatal("overwrote existing config")
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "first" {
		t.Fatalf("file changed after rejected overwrite: %q, %v", b, err)
	}
	if err := WriteNew("", []byte("x")); err == nil {
		t.Fatal("missing output path accepted")
	}
}
