package session

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/sysmrk/mrkvpn/core/transport"
)

func newSession(t *testing.T, policy Policy) *Session {
	t.Helper()
	s, err := New("session-1", "device-1", "gateway-1", policy)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func path(id string, rtt time.Duration, state PathState) Path {
	return Path{ID: id, Kind: transport.WireGuard, State: state, RTT: rtt}
}

func mustPath(t *testing.T, s *Session, p Path) {
	t.Helper()
	if err := s.UpsertPath(p); err != nil {
		t.Fatal(err)
	}
}

func TestManualFailsClosed(t *testing.T) {
	s := newSession(t, Policy{Mode: Manual, PreferredPathID: "manual"})
	mustPath(t, s, path("other", 1*time.Millisecond, Ready))
	if _, err := s.Select(time.Unix(100, 0)); !errors.Is(err, ErrNoAvailablePath) {
		t.Fatalf("expected fail-closed error, got %v", err)
	}
	mustPath(t, s, path("manual", 80*time.Millisecond, Ready))
	result, err := s.Select(time.Unix(101, 0))
	if err != nil || result.PathID != "manual" || !result.Changed {
		t.Fatalf("unexpected manual selection: %+v, %v", result, err)
	}
	mustPath(t, s, path("manual", 80*time.Millisecond, Down))
	if s.ActivePathID() != "" {
		t.Fatal("down path still active")
	}
	if _, err := s.Select(time.Unix(102, 0)); !errors.Is(err, ErrNoAvailablePath) {
		t.Fatal("manual selection unexpectedly fell back")
	}
}

func TestAutomaticHysteresisAndFailover(t *testing.T) {
	s := newSession(t, Policy{Mode: Automatic, MinDwell: 10 * time.Second, MinImprovementRatio: 0.2})
	start := time.Unix(100, 0)
	mustPath(t, s, path("a", 60*time.Millisecond, Ready))
	first, _ := s.Select(start)
	if first.PathID != "a" {
		t.Fatal(first)
	}
	mustPath(t, s, path("b", 10*time.Millisecond, Ready))
	second, _ := s.Select(start.Add(2 * time.Second))
	if second.PathID != "a" || second.Changed {
		t.Fatalf("switched inside dwell: %+v", second)
	}
	third, _ := s.Select(start.Add(11 * time.Second))
	if third.PathID != "b" || !third.Changed {
		t.Fatalf("did not switch on major improvement: %+v", third)
	}
	mustPath(t, s, path("b", 10*time.Millisecond, Down))
	fallback, _ := s.Select(start.Add(12 * time.Second))
	if fallback.PathID != "a" {
		t.Fatalf("failed to recover from down path: %+v", fallback)
	}
}

func TestAutomaticImprovementThreshold(t *testing.T) {
	s := newSession(t, Policy{Mode: Automatic, MinImprovementRatio: 0.25})
	start := time.Unix(100, 0)
	mustPath(t, s, path("a", 50*time.Millisecond, Ready))
	s.Select(start)
	mustPath(t, s, path("b", 48*time.Millisecond, Ready))
	choice, _ := s.Select(start.Add(time.Second))
	if choice.PathID != "a" {
		t.Fatalf("unnecessary switch: %+v", choice)
	}
}

func TestPreferredFallbackAndRecovery(t *testing.T) {
	s := newSession(t, Policy{Mode: Preferred, PreferredPathID: "preferred"})
	mustPath(t, s, path("backup", 10*time.Millisecond, Ready))
	a, err := s.Select(time.Unix(100, 0))
	if err != nil || a.PathID != "backup" {
		t.Fatalf("backup not selected: %+v, %v", a, err)
	}
	mustPath(t, s, path("preferred", 100*time.Millisecond, Ready))
	b, err := s.Select(time.Unix(101, 0))
	if err != nil || b.PathID != "preferred" {
		t.Fatalf("preferred not restored: %+v, %v", b, err)
	}
}

func TestValidationAndIdentityBinding(t *testing.T) {
	if _, err := New("", "device", "gateway", Policy{Mode: Automatic}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("missing session ID accepted")
	}
	if _, err := New("s", "d", "g", Policy{Mode: Manual}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("manual mode without target accepted")
	}
	if _, err := New("s", "d", "g", Policy{Mode: Automatic, MinImprovementRatio: math.NaN()}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("NaN policy accepted")
	}
	s := newSession(t, Policy{Mode: Automatic})
	bad := []Path{
		path("", time.Millisecond, Ready),
		path("ok", -time.Millisecond, Ready),
		{ID: "ok", Kind: transport.WireGuard, State: Ready, LossRatio: math.NaN()},
		{ID: "ok", Kind: "invalid", State: Ready},
		{ID: "ok", Kind: transport.WireGuard, State: "unrecognized"},
	}
	for _, p := range bad {
		if !errors.Is(s.UpsertPath(p), ErrInvalidInput) {
			t.Fatalf("invalid path accepted: %+v", p)
		}
	}
	mustPath(t, s, path("bound", 3*time.Millisecond, Ready))
	if err := s.UpsertPath(Path{ID: "bound", Kind: transport.QUIC, State: Ready}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("transport kind changed for existing path ID")
	}
}

func TestRemoveCloseAndConcurrentReaders(t *testing.T) {
	s := newSession(t, Policy{Mode: Automatic})
	mustPath(t, s, path("a", 4*time.Millisecond, Ready))
	s.Select(time.Unix(1, 0))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.ActivePathID()
			_ = s.Paths()
		}()
	}
	wg.Wait()
	if err := s.RemovePath("a"); err != nil || s.ActivePathID() != "" {
		t.Fatal("path removal left active recommendation")
	}
	s.Close()
	if err := s.UpsertPath(path("a", time.Millisecond, Ready)); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed session accepted mutation: %v", err)
	}
	if _, err := s.Select(time.Now()); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed session selected path: %v", err)
	}
}
