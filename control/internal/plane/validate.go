package plane

import (
	"encoding/base64"
	"errors"
	"net"
	"net/mail"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func validName(v string, max int) bool {
	return len(v) > 0 && len(v) <= max && utf8.ValidString(v) && v == strings.TrimSpace(v) && !strings.ContainsAny(v, "\r\n\t")
}
func validEmail(v string) bool {
	if v == "" {
		return true
	}
	if len(v) > 254 || strings.ContainsAny(v, " \r\n\t") {
		return false
	}
	addr, err := mail.ParseAddress(v)
	return err == nil && addr.Address == v
}
func validExpiry(v string) bool {
	if v == "" {
		return true
	}
	expiry, err := time.Parse(time.RFC3339, v)
	return err == nil && expiry.After(time.Now().UTC())
}
func validWireGuardPublicKey(raw string) bool {
	bytes, err := base64.StdEncoding.Strict().DecodeString(raw)
	if err != nil || len(bytes) != 32 {
		return false
	}
	nonzero := false
	for _, b := range bytes {
		if b != 0 {
			nonzero = true
		}
	}
	return nonzero
}
func validEndpoint(v string) bool {
	if len(v) > 120 || strings.ContainsAny(v, " /?@\r\n\t") {
		return false
	}
	host, port, err := net.SplitHostPort(v)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.IsGlobalUnicast() {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
func validatePolicy(p Policy) error {
	switch p.Mode {
	case "automatic", "preferred", "manual":
	default:
		return errors.New("mode must be automatic, preferred or manual")
	}
	if p.DNSMode != "tunnel" {
		return errors.New("only tunnel DNS mode is accepted as desired configuration")
	}
	if len(p.AllowedTransports) < 1 || len(p.AllowedTransports) > 3 {
		return errors.New("allowed transports required")
	}
	seen := map[string]bool{}
	hasPreferred := false
	for _, tr := range p.AllowedTransports {
		if tr != "wireguard" && tr != "quic" && tr != "https-relay" {
			return errors.New("unknown transport")
		}
		if seen[tr] {
			return errors.New("duplicate transport")
		}
		seen[tr] = true
		if tr == p.PreferredTransport {
			hasPreferred = true
		}
	}
	if p.Mode != "automatic" && (!hasPreferred || p.PreferredTransport == "") {
		return errors.New("preferred transport must belong to allowed transports")
	}
	return nil
}
