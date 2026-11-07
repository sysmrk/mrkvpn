package plane

import "time"

// This schema is deliberately small and versioned. It is NOT WireGuard
// configuration, a live device enrollment protocol or a gateway API.
const schemaVersion = 1

type User struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	MaxDevices int       `json:"max_devices"`
	ExpiresAt  string    `json:"expires_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Device struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	PublicKey string    `json:"wireguard_public_key"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Gateway struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Transport string    `json:"transport"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Policy struct {
	UserID             string    `json:"user_id"`
	Mode               string    `json:"mode"`
	PreferredTransport string    `json:"preferred_transport"`
	AllowedTransports  []string  `json:"allowed_transports"`
	DNSMode            string    `json:"dns_mode"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Token digests are stored in the state file; raw tokens are never stored
// there or returned again. Treat the generated bootstrap.token as a secret.
type Token struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Digest    string    `json:"digest"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Revoked   bool      `json:"revoked"`
}

type TokenView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Revoked   bool      `json:"revoked"`
}

func (t Token) View() TokenView {
	return TokenView{ID: t.ID, Name: t.Name, Role: t.Role, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, Revoked: t.Revoked}
}

type AuditEvent struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
}

type State struct {
	SchemaVersion int                `json:"schema_version"`
	Users         map[string]User    `json:"users"`
	Devices       map[string]Device  `json:"devices"`
	Gateways      map[string]Gateway `json:"gateways"`
	Policies      map[string]Policy  `json:"policies"`
	Tokens        map[string]Token   `json:"tokens"`
	Audit         []AuditEvent       `json:"audit"`
}

func newState() State {
	return State{
		SchemaVersion: schemaVersion,
		Users: map[string]User{}, Devices: map[string]Device{},
		Gateways: map[string]Gateway{}, Policies: map[string]Policy{},
		Tokens: map[string]Token{}, Audit: []AuditEvent{},
	}
}
