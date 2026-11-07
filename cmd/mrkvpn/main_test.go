package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sysmrk/mrkvpn/internal/wgconfig"
)

func testKey(v byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{v}, 32))
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "gateway.key")
	if err := os.WriteFile(keyFile, []byte(testKey(7)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := wgconfig.Gateway{
		InterfaceName: "wg0",
		Address: "10.77.0.1/24",
		ListenPort: 51820,
		PrivateKeyFile: keyFile,
		Peers: []wgconfig.Peer{
			{Name: "first-device", PublicKey: testKey(8), Address: "10.77.0.2/32"},
		},
	}
	path := filepath.Join(dir, "gateway.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGatewayCheckAndRender(t *testing.T) {
	config := fixture(t)
	var output bytes.Buffer
	if err := run([]string{"gateway", "check", "-config", config}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "no system changes") || strings.Contains(output.String(), testKey(7)) {
		t.Fatalf("unsafe check output: %q", output.String())
	}
	out := filepath.Join(t.TempDir(), "wg0.conf")
	output.Reset()
	if err := run([]string{"gateway", "render", "-config", config, "-out", out}, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), testKey(7)) {
		t.Fatal("render printed private key")
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), "PrivateKey = "+testKey(7)) {
		t.Fatalf("rendered config missing or invalid: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("output not restricted: %v", err)
	}
	if err := run([]string{"gateway", "render", "-config", config, "-out", out}, &output); err == nil {
		t.Fatal("overwrote existing config file")
	}
}

func TestGatewayRejectsMalformedInput(t *testing.T) {
	c := fixture(t)
	for _, args := range [][]string{
		nil,
		{"gateway"},
		{"gateway", "render", "-config", c},
		{"gateway", "check"},
		{"gateway", "check", "-config", c, "-out", "x"},
		{"gateway", "check", "-config", c, "extra"},
		{"gateway", "unknown"},
	} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted invalid command: %v", args)
		}
	}

	dir := t.TempDir()
	for _, raw := range []string{
		`{"interface_name":"wg0","unexpected_field":1}`,
		`{"address":"10.77.0.1/24"}{"address":"10.77.0.2/24"}`,
		`{"address":"10.77.0.1/24"`,
	} {
		p := filepath.Join(dir, "invalid.json")
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readConfig(p); err == nil {
			t.Fatalf("accepted malformed JSON: %s", raw)
		}
	}
}
