package plane

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

func sortedUsers(st State) []User {
	out := make([]User, 0, len(st.Users))
	for _, v := range st.Users {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func sortedDevices(st State) []Device {
	out := make([]Device, 0, len(st.Devices))
	for _, v := range st.Devices {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func sortedGateways(st State) []Gateway {
	out := make([]Gateway, 0, len(st.Gateways))
	for _, v := range st.Gateways {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func sortedPolicies(st State) []Policy {
	out := make([]Policy, 0, len(st.Policies))
	for _, v := range st.Policies {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

type newUserRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	MaxDevices int    `json:"max_devices"`
	ExpiresAt  string `json:"expires_at"`
}

func (a *API) users(w http.ResponseWriter, r *http.Request, actor Token) {
	if r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"items": sortedUsers(a.store.snapshot())})
		return
	}
	var input newUserRequest
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if !validName(input.Name, 80) || !validEmail(input.Email) || !validExpiry(input.ExpiresAt) || input.MaxDevices < 1 || input.MaxDevices > 20 {
		respondError(w, invalid("name, email, expiration or max_devices invalid (1..20)"))
		return
	}
	now := time.Now().UTC()
	id := randomID("usr")
	out, err := a.store.change(actor.ID, "users.create", id, func(st *State) (any, error) {
		if len(st.Users) >= 200 {
			return nil, conflict("user inventory capacity exceeded")
		}
		for _, u := range st.Users {
			if input.Email != "" && strings.EqualFold(u.Email, input.Email) {
				return nil, conflict("email already exists")
			}
		}
		u := User{ID: id, Name: input.Name, Email: input.Email, Status: "active", MaxDevices: input.MaxDevices, ExpiresAt: input.ExpiresAt, CreatedAt: now, UpdatedAt: now}
		st.Users[id] = u
		return u, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (a *API) user(w http.ResponseWriter, r *http.Request, actor Token) {
	id := r.PathValue("id")
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if input.Status != "active" && input.Status != "suspended" {
		respondError(w, invalid("status must be active or suspended"))
		return
	}
	out, err := a.store.change(actor.ID, "users.status", id, func(st *State) (any, error) {
		u, ok := st.Users[id]
		if !ok {
			return nil, missing()
		}
		u.Status = input.Status
		u.UpdatedAt = time.Now().UTC()
		st.Users[id] = u
		return u, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

type newDeviceRequest struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	PublicKey string `json:"wireguard_public_key"`
}

func (a *API) devices(w http.ResponseWriter, r *http.Request, actor Token) {
	if r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"items": sortedDevices(a.store.snapshot())})
		return
	}
	var input newDeviceRequest
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if !validName(input.Name, 80) || !validWireGuardPublicKey(input.PublicKey) {
		respondError(w, invalid("name or WireGuard public key invalid"))
		return
	}
	switch input.Platform {
	case "windows", "android", "linux", "macos", "ios":
	default:
		respondError(w, invalid("unsupported platform"))
		return
	}
	id := randomID("dev")
	now := time.Now().UTC()
	out, err := a.store.change(actor.ID, "devices.register", id, func(st *State) (any, error) {
		u, ok := st.Users[input.UserID]
		if !ok {
			return nil, missing()
		}
		if u.Status != "active" {
			return nil, conflict("user not active")
		}
		if u.ExpiresAt != "" {
			when, e := time.Parse(time.RFC3339, u.ExpiresAt)
			if e != nil || !when.After(time.Now().UTC()) {
				return nil, conflict("user expired")
			}
		}
		count := 0
		for _, d := range st.Devices {
			if d.PublicKey == input.PublicKey {
				return nil, conflict("WireGuard public key is already registered")
			}
			if d.UserID == input.UserID && d.Status == "active" {
				count++
			}
		}
		if count >= u.MaxDevices {
			return nil, conflict("device limit reached")
		}
		if len(st.Devices) >= 1000 {
			return nil, conflict("device capacity exceeded")
		}
		d := Device{ID: id, UserID: input.UserID, Name: input.Name, Platform: input.Platform, PublicKey: input.PublicKey, Status: "active", CreatedAt: now, UpdatedAt: now}
		st.Devices[id] = d
		return d, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 201, out)
}
func (a *API) device(w http.ResponseWriter, r *http.Request, actor Token) {
	id := r.PathValue("id")
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if input.Status != "revoked" {
		respondError(w, invalid("only terminal device revocation is supported"))
		return
	}
	out, err := a.store.change(actor.ID, "devices.revoke", id, func(st *State) (any, error) {
		d, ok := st.Devices[id]
		if !ok {
			return nil, missing()
		}
		if d.Status == "revoked" {
			return nil, conflict("device already revoked")
		}
		d.Status = "revoked"
		d.UpdatedAt = time.Now().UTC()
		st.Devices[id] = d
		return d, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

type newGatewayRequest struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Transport string `json:"transport"`
}

func (a *API) gateways(w http.ResponseWriter, r *http.Request, actor Token) {
	if r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"items": sortedGateways(a.store.snapshot())})
		return
	}
	var input newGatewayRequest
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if !validName(input.Name, 80) || !validEndpoint(input.Endpoint) || input.Transport != "wireguard" {
		respondError(w, invalid("gateway needs a valid name, direct-IP endpoint and supported wireguard transport"))
		return
	}
	id := randomID("gw")
	now := time.Now().UTC()
	out, err := a.store.change(actor.ID, "gateways.register", id, func(st *State) (any, error) {
		if len(st.Gateways) >= 100 {
			return nil, conflict("gateway capacity exceeded")
		}
		for _, g := range st.Gateways {
			if g.Endpoint == input.Endpoint && g.Transport == input.Transport {
				return nil, conflict("gateway endpoint already exists")
			}
		}
		g := Gateway{ID: id, Name: input.Name, Endpoint: input.Endpoint, Transport: input.Transport, Status: "enabled", CreatedAt: now, UpdatedAt: now}
		st.Gateways[id] = g
		return g, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 201, out)
}
func (a *API) gateway(w http.ResponseWriter, r *http.Request, actor Token) {
	id := r.PathValue("id")
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	if input.Status != "enabled" && input.Status != "disabled" {
		respondError(w, invalid("status must be enabled or disabled"))
		return
	}
	out, err := a.store.change(actor.ID, "gateways.status", id, func(st *State) (any, error) {
		g, ok := st.Gateways[id]
		if !ok {
			return nil, missing()
		}
		g.Status = input.Status
		g.UpdatedAt = time.Now().UTC()
		st.Gateways[id] = g
		return g, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (a *API) policies(w http.ResponseWriter, _ *http.Request, _ Token) {
	writeJSON(w, 200, map[string]any{"items": sortedPolicies(a.store.snapshot())})
}
func (a *API) policy(w http.ResponseWriter, r *http.Request, actor Token) {
	id := r.PathValue("id")
	var input struct {
		Mode               string   `json:"mode"`
		PreferredTransport string   `json:"preferred_transport"`
		AllowedTransports  []string `json:"allowed_transports"`
		DNSMode            string   `json:"dns_mode"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		respondError(w, err)
		return
	}
	p := Policy{UserID: id, Mode: input.Mode, PreferredTransport: input.PreferredTransport, AllowedTransports: input.AllowedTransports, DNSMode: input.DNSMode, UpdatedAt: time.Now().UTC()}
	if err := validatePolicy(p); err != nil {
		respondError(w, invalid(err.Error()))
		return
	}
	out, err := a.store.change(actor.ID, "policies.update", id, func(st *State) (any, error) {
		if _, ok := st.Users[id]; !ok {
			return nil, missing()
		}
		st.Policies[id] = p
		return p, nil
	})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (a *API) audit(w http.ResponseWriter, _ *http.Request, _ Token) {
	st := a.store.snapshot()
	audit := st.Audit
	if len(audit) > 200 {
		audit = audit[len(audit)-200:]
	}
	writeJSON(w, 200, map[string]any{"items": reverseAudit(audit), "retained_total": len(st.Audit), "limit": 200})
}
