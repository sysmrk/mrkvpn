package plane

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (*Store, string, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	raw, err := os.ReadFile(filepath.Join(store.dir, "bootstrap.token"))
	if err != nil {
		t.Fatal(err)
	}
	return store, strings.TrimSpace(string(raw)), NewAPI(store).Handler()
}
func call(h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, request)
	return resp
}
func requireStatus(t *testing.T, resp *httptest.ResponseRecorder, want int) {
	t.Helper()
	if resp.Code != want {
		t.Fatalf("HTTP %d wanted %d: %s", resp.Code, want, resp.Body.String())
	}
}
func parse(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(resp.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapPersistenceAndSingleWriter(t *testing.T) {
	store, raw, _ := setup(t)
	if !strings.HasPrefix(raw, "mrkvpn_") {
		t.Fatal("missing bootstrap secret")
	}
	if _, ok := store.authenticate(raw); !ok {
		t.Fatal("bootstrap token not valid")
	}
	if _, err := OpenStore(store.dir); err == nil {
		t.Fatal("second writer unexpectedly acquired lock")
	}
	for _, p := range []string{"state.json", "bootstrap.token"} {
		info, err := os.Stat(filepath.Join(store.dir, p))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("secret permissions %s: %v", p, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(store.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, ok := reopened.authenticate(raw); !ok {
		t.Fatal("bootstrap lost across restart")
	}
}
func TestAuthRBACAndHeaders(t *testing.T) {
	_, owner, h := setup(t)
	requireStatus(t, call(h, "GET", "/api/v1/users", "", nil), 401)
	requireStatus(t, call(h, "GET", "/api/v1/users", "invalid", nil), 401)
	requireStatus(t, call(h, "GET", "/api/v1/users", owner, nil), 200)
	requireStatus(t, call(h, "GET", "/healthz", "", nil), 200)
	resp := call(h, "GET", "/", "", nil)
	requireStatus(t, resp, 200)
	if !strings.Contains(resp.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("CSP missing")
	}
	if resp.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cache allowed")
	}

	issued := call(h, "POST", "/api/v1/tokens", owner, map[string]any{
		"name": "read-only-auditor", "role": "auditor", "expires_hours": 24,
	})
	requireStatus(t, issued, 201)
	var result struct {
		Token    string    `json:"token"`
		Metadata TokenView `json:"metadata"`
	}
	parse(t, issued, &result)
	if result.Token == "" || result.Metadata.Role != "auditor" {
		t.Fatal("token issue incorrect")
	}
	requireStatus(t, call(h, "GET", "/api/v1/users", result.Token, nil), 200)
	requireStatus(t, call(h, "POST", "/api/v1/users", result.Token, map[string]any{
		"name": "No Write", "email": "", "max_devices": 1, "expires_at": "",
	}), 403)
	requireStatus(t, call(h, "GET", "/api/v1/tokens", result.Token, nil), 403)
	// Raw secret must never appear in token listing or persisted JSON state.
	list := call(h, "GET", "/api/v1/tokens", owner, nil)
	requireStatus(t, list, 200)
	if strings.Contains(list.Body.String(), result.Token) {
		t.Fatal("raw token disclosed in list")
	}
}
func publicKey(value byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
}
func TestUserDevicePolicyAuditFlow(t *testing.T) {
	st, owner, h := setup(t)
	create := call(h, "POST", "/api/v1/users", owner, map[string]any{
		"name": "Alice", "email": "alice@example.test", "max_devices": 1, "expires_at": "",
	})
	requireStatus(t, create, 201)
	var user User
	parse(t, create, &user)
	if user.ID == "" || user.Status != "active" {
		t.Fatal(user)
	}
	requireStatus(t, call(h, "POST", "/api/v1/users", owner, map[string]any{
		"name": "AliceAgain", "email": "alice@example.test", "max_devices": 1, "expires_at": "",
	}), 409)
	device := func(name string, key byte) *httptest.ResponseRecorder {
		return call(h, "POST", "/api/v1/devices", owner, map[string]any{
			"user_id": user.ID, "name": name, "platform": "windows", "wireguard_public_key": publicKey(key),
		})
	}
	first := device("Laptop", 2)
	requireStatus(t, first, 201)
	var d Device
	parse(t, first, &d)
	requireStatus(t, device("Another", 3), 409) // max one active device
	requireStatus(t, device("SameKey", 2), 409)
	revoke := call(h, "PATCH", "/api/v1/devices/"+d.ID, owner, map[string]any{"status": "revoked"})
	requireStatus(t, revoke, 200)
	requireStatus(t, device("Android", 3), 201)
	requireStatus(t, device("ReusedKey", 2), 409) // revoked key remains reserved
	requireStatus(t, call(h, "PUT", "/api/v1/policies/"+user.ID, owner, map[string]any{
		"mode": "manual", "preferred_transport": "wireguard",
		"allowed_transports": []string{"wireguard"}, "dns_mode": "tunnel",
	}), 200)
	requireStatus(t, call(h, "PUT", "/api/v1/policies/"+user.ID, owner, map[string]any{
		"mode": "manual", "preferred_transport": "quic",
		"allowed_transports": []string{"wireguard"}, "dns_mode": "tunnel",
	}), 400)
	requireStatus(t, call(h, "PATCH", "/api/v1/users/"+user.ID, owner, map[string]any{"status": "suspended"}), 200)
	requireStatus(t, device("CannotJoin", 4), 409)
	audit := call(h, "GET", "/api/v1/audit", owner, nil)
	requireStatus(t, audit, 200)
	if !strings.Contains(audit.Body.String(), "devices.revoke") || !strings.Contains(audit.Body.String(), "policies.update") {
		t.Fatal("expected audit events missing")
	}
	snapshot := st.snapshot()
	if snapshot.Users[user.ID].Status != "suspended" || snapshot.Devices[d.ID].Status != "revoked" {
		t.Fatal("mutations not persisted")
	}
}
func TestGatewaysAndTokenRevocation(t *testing.T) {
	_, owner, h := setup(t)
	requireStatus(t, call(h, "POST", "/api/v1/gateways", owner, map[string]any{
		"name": "loopback", "endpoint": "127.0.0.1:51820", "transport": "wireguard",
	}), 400)
	add := call(h, "POST", "/api/v1/gateways", owner, map[string]any{
		"name": "alpha", "endpoint": "8.8.8.8:51820", "transport": "wireguard",
	})
	requireStatus(t, add, 201)
	var gw Gateway
	parse(t, add, &gw)
	requireStatus(t, call(h, "PATCH", "/api/v1/gateways/"+gw.ID, owner, map[string]any{"status": "disabled"}), 200)
	requireStatus(t, call(h, "POST", "/api/v1/tokens/"+strings.TrimPrefix(owner, "mrkvpn_")+"/revoke", owner, map[string]any{}), 404)

	list := call(h, "GET", "/api/v1/tokens", owner, nil)
	var found struct {
		Items []TokenView `json:"items"`
	}
	parse(t, list, &found)
	if len(found.Items) != 1 {
		t.Fatal(found)
	}
	boot := found.Items[0].ID
	requireStatus(t, call(h, "POST", "/api/v1/tokens/"+boot+"/revoke", owner, map[string]any{}), 409) // last owner
}
func TestInputSizeAndStrictJSON(t *testing.T) {
	_, owner, h := setup(t)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(`{"name":"Alice","unknown":"value"}`))
	req.Header.Set("Authorization", "Bearer "+owner)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(resp, req)
	requireStatus(t, resp, 400)
	big := strings.Repeat("X", 40000)
	requireStatus(t, call(h, "POST", "/api/v1/users", owner, map[string]any{
		"name": big, "email": "", "max_devices": 1,
	}), 400)
}
