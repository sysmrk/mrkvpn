package transport

import "testing"

func TestKindValidation(t *testing.T) {
	for _, k := range []Kind{WireGuard, QUIC, HTTPSRelay} {
		if !k.Valid() {
			t.Fatalf("known kind %q rejected", k)
		}
	}
	for _, k := range []Kind{"", "unknown", "tcp"} {
		if k.Valid() {
			t.Fatalf("unexpected kind accepted: %q", k)
		}
	}
}
