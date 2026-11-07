package plane

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var webFS embed.FS

type API struct{ store *Store }

type apiError struct {
	Status int
	Code string
	Message string
}

func (e *apiError) Error() string { return e.Message }
func invalid(message string) error { return &apiError{http.StatusBadRequest, "invalid_request", message} }
func conflict(message string) error { return &apiError{http.StatusConflict, "conflict", message} }
func missing() error { return &apiError{http.StatusNotFound, "not_found", "resource not found"} }

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status":"ok","service":"mrkvpn-control"})
	})
	mux.HandleFunc("GET /", a.index)
	mux.HandleFunc("GET /app.css", a.asset("web/app.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /app.js", a.asset("web/app.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /api/v1/me", a.secure("read", a.me))
	mux.HandleFunc("GET /api/v1/overview", a.secure("read", a.overview))
	mux.HandleFunc("GET /api/v1/users", a.secure("read", a.users))
	mux.HandleFunc("POST /api/v1/users", a.secure("write", a.users))
	mux.HandleFunc("PATCH /api/v1/users/{id}", a.secure("write", a.user))
	mux.HandleFunc("GET /api/v1/devices", a.secure("read", a.devices))
	mux.HandleFunc("POST /api/v1/devices", a.secure("write", a.devices))
	mux.HandleFunc("PATCH /api/v1/devices/{id}", a.secure("write", a.device))
	mux.HandleFunc("GET /api/v1/gateways", a.secure("read", a.gateways))
	mux.HandleFunc("POST /api/v1/gateways", a.secure("write", a.gateways))
	mux.HandleFunc("PATCH /api/v1/gateways/{id}", a.secure("write", a.gateway))
	mux.HandleFunc("GET /api/v1/policies", a.secure("read", a.policies))
	mux.HandleFunc("PUT /api/v1/policies/{id}", a.secure("write", a.policy))
	mux.HandleFunc("GET /api/v1/audit", a.secure("read", a.audit))
	mux.HandleFunc("GET /api/v1/tokens", a.secure("owner", a.tokens))
	mux.HandleFunc("POST /api/v1/tokens", a.secure("owner", a.tokens))
	mux.HandleFunc("POST /api/v1/tokens/{id}/revoke", a.secure("owner", a.revokeToken))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		mux.ServeHTTP(w, r)
	})
}

func NewAPI(store *Store) *API { return &API{store:store} }
func (a *API) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" { http.NotFound(w, r); return }
	a.asset("web/index.html", "text/html; charset=utf-8")(w,r)
}
func (a *API) asset(path, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := webFS.ReadFile(path)
		if err != nil { http.Error(w, "not found", http.StatusNotFound); return }
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(content)
	}
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func respondError(w http.ResponseWriter, err error) {
	var e *apiError
	if errors.As(err, &e) {
		writeJSON(w,e.Status,map[string]any{"error":map[string]string{"code":e.Code,"message":e.Message}})
		return
	}
	writeJSON(w,http.StatusInternalServerError,map[string]any{"error":map[string]string{"code":"internal_error","message":"request could not be completed"}})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return &apiError{http.StatusUnsupportedMediaType, "content_type", "application/json required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil { return invalid("invalid JSON payload or unknown field") }
	var more any
	if err := d.Decode(&more); err != io.EOF { return invalid("JSON body must contain a single object") }
	return nil
}
type securedHandler func(http.ResponseWriter, *http.Request, Token)
func (a *API) secure(permission string, handler securedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if len(header) < 8 || len(header) > 256 || !strings.HasPrefix(header, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error":map[string]string{"code":"unauthorized","message":"Bearer token required"}})
			return
		}
		actor, ok := a.store.authenticate(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error":map[string]string{"code":"unauthorized","message":"invalid or expired token"}})
			return
		}
		if !allowed(actor.Role, permission) {
			writeJSON(w,http.StatusForbidden,map[string]any{"error":map[string]string{"code":"forbidden","message":"insufficient role permissions"}})
			return
		}
		handler(w,r,actor)
	}
}
func allowed(role, permission string) bool {
	if role == "owner" { return true }
	if permission == "read" { return role=="admin" || role=="operator" || role=="auditor" }
	if permission == "write" { return role=="admin" }
	return false
}
func (a *API) me(w http.ResponseWriter, _ *http.Request, actor Token) {
	writeJSON(w,http.StatusOK,actor.View())
}
func (a *API) overview(w http.ResponseWriter, _ *http.Request, _ Token) {
	st := a.store.snapshot()
	activeUsers, activeDevices, enabledGateways := 0,0,0
	for _, u := range st.Users { if u.Status=="active" { activeUsers++ } }
	for _, d := range st.Devices { if d.Status=="active" { activeDevices++ } }
	for _, g := range st.Gateways { if g.Status=="enabled" { enabledGateways++ } }
	last := st.Audit
	if len(last) > 8 { last=last[len(last)-8:] }
	writeJSON(w,http.StatusOK,map[string]any{
		"users_total":len(st.Users),"users_active":activeUsers,
		"devices_total":len(st.Devices),"devices_active":activeDevices,
		"gateways_total":len(st.Gateways),"gateways_enabled":enabledGateways,
		"recent_audit":reverseAudit(last),
		"data_plane_connected":false,
		"disclaimer":"Inventory and desired state only; no active gateway telemetry or VPN enforcement.",
		"generated_at":time.Now().UTC(),
	})
}
func reverseAudit(in []AuditEvent) []AuditEvent {
	out:=make([]AuditEvent,len(in))
	for i:=range in { out[i]=in[len(in)-1-i] }
	return out
}
