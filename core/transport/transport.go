// Package transport defines contracts for authenticated IP-packet paths.
// No network transport or cryptographic implementation is provided here.
package transport

import "context"

// Kind identifies a transport mechanism, not a security guarantee.
type Kind string

const (
	WireGuard  Kind = "wireguard"
	QUIC       Kind = "quic"
	HTTPSRelay Kind = "https-relay"
)

func (k Kind) Valid() bool {
	switch k {
	case WireGuard, QUIC, HTTPSRelay:
		return true
	default:
		return false
	}
}

// Endpoint is a candidate entry point. Address is transport-specific and
// untrusted until the adapter validates it (including TLS identity if used).
// Direct-IP endpoints must not require a DNS lookup.
type Endpoint struct {
	ID      string
	Kind    Kind
	Address string
}

// IdentityRef identifies the device and authenticated gateway. It contains
// identifiers only; private keys and bearer credentials must not be logged or
// stored in these structs.
type IdentityRef struct {
	DeviceID  string
	GatewayID string
}

// PacketConn represents a secure, end-to-end authenticated path carrying IP
// packets. A relay is never trusted with decrypted packets. Implementations
// must handle context cancellation, concurrent Close, packet MTU validation,
// replay protection, and cryptographic authentication explicitly.
// ReadPacket returns a whole packet or an error; it must not return a partial
// packet. The caller owns the buffer and packet data.
type PacketConn interface {
	ID() string
	Kind() Kind
	ReadPacket(ctx context.Context, dst []byte) (int, error)
	WritePacket(ctx context.Context, packet []byte) error
	Close() error
}

// Adapter establishes secure paths for one protocol. Connect must not
// silently fall back to a different protocol or gateway identity.
type Adapter interface {
	Kind() Kind
	Connect(ctx context.Context, endpoint Endpoint, identity IdentityRef) (PacketConn, error)
}
