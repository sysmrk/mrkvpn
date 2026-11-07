// Package session models path availability and deterministic path selection.
// It does NOT forward traffic, configure an OS kill switch, or authenticate a
// transport. Selection decisions are advisory to a separately enforced policy.
package session

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sysmrk/mrkvpn/core/transport"
)

var (
	ErrClosed          = errors.New("session is closed")
	ErrNoAvailablePath = errors.New("no eligible path; fail closed")
	ErrInvalidInput    = errors.New("invalid session input")
)

type Mode string

const (
	Automatic Mode = "automatic"
	Preferred Mode = "preferred"
	Manual    Mode = "manual"
)

type PathState string

const (
	Ready    PathState = "ready"
	Degraded PathState = "degraded"
	Down     PathState = "down"
)

// Path is a telemetry snapshot for a previously authenticated connection.
// A Ready or Degraded report alone does not authenticate the connection.
type Path struct {
	ID        string
	Kind      transport.Kind
	State     PathState
	RTT       time.Duration
	LossRatio float64 // [0, 1]
}

// Policy controls selection. PreferredPathID is mandatory for Manual and
// Preferred modes. MinDwell prevents unnecessary switches between live paths.
// MinImprovementRatio applies to Automatic only, with values from 0 to 1.
type Policy struct {
	Mode                Mode
	PreferredPathID     string
	MinDwell            time.Duration
	MinImprovementRatio float64
}

type Decision struct {
	PathID  string
	Kind    transport.Kind
	Changed bool
}

// Session owns one logical identity and the current recommended primary path.
// Multiple paths may remain connected simultaneously. Packet striping and
// per-flow active-active scheduling are separate, future capabilities.
type Session struct {
	mu         sync.RWMutex
	id         string
	deviceID   string
	gatewayID  string
	policy     Policy
	paths      map[string]Path
	activeID   string
	lastSwitch time.Time
	closed     bool
}

func New(id, deviceID, gatewayID string, policy Policy) (*Session, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(deviceID) == "" || strings.TrimSpace(gatewayID) == "" || !validPolicy(policy) {
		return nil, ErrInvalidInput
	}
	return &Session{
		id: id, deviceID: deviceID, gatewayID: gatewayID,
		policy: policy, paths: make(map[string]Path),
	}, nil
}

func validPolicy(p Policy) bool {
	if p.MinDwell < 0 || math.IsNaN(p.MinImprovementRatio) || math.IsInf(p.MinImprovementRatio, 0) || p.MinImprovementRatio < 0 || p.MinImprovementRatio > 1 {
		return false
	}
	switch p.Mode {
	case Automatic:
		return true
	case Preferred, Manual:
		return strings.TrimSpace(p.PreferredPathID) != ""
	default:
		return false
	}
}

func validPath(p Path) bool {
	if strings.TrimSpace(p.ID) == "" || !p.Kind.Valid() || p.RTT < 0 || math.IsNaN(p.LossRatio) || math.IsInf(p.LossRatio, 0) || p.LossRatio < 0 || p.LossRatio > 1 {
		return false
	}
	return p.State == Ready || p.State == Degraded || p.State == Down
}

func eligible(p Path) bool { return p.State == Ready || p.State == Degraded }

func (s *Session) UpsertPath(p Path) error {
	if !validPath(p) {
		return ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	// A path identity is bound to its transport kind for the session lifetime.
	if previous, ok := s.paths[p.ID]; ok && previous.Kind != p.Kind {
		return ErrInvalidInput
	}
	s.paths[p.ID] = p
	if s.activeID == p.ID && !eligible(p) {
		s.activeID = "" // never recommend a known-down path
	}
	return nil
}

func (s *Session) RemovePath(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	delete(s.paths, id)
	if s.activeID == id {
		s.activeID = ""
	}
	return nil
}

// Select chooses a recommended path at a caller-provided time, so hysteresis
// behavior is deterministic and unit-testable. If none is eligible, it clears
// the active recommendation and returns ErrNoAvailablePath. In Manual mode it
// does not use alternative paths, even when those paths are healthy.
func (s *Session) Select(now time.Time) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Decision{}, ErrClosed
	}

	candidates := make([]Path, 0, len(s.paths))
	for _, p := range s.paths {
		if eligible(p) {
			candidates = append(candidates, p)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := score(candidates[i]), score(candidates[j])
		if a == b {
			return candidates[i].ID < candidates[j].ID
		}
		return a < b
	})

	var choice Path
	switch s.policy.Mode {
	case Manual:
		p, ok := s.paths[s.policy.PreferredPathID]
		if !ok || !eligible(p) {
			s.activeID = ""
			return Decision{}, ErrNoAvailablePath
		}
		choice = p
	case Preferred:
		if p, ok := s.paths[s.policy.PreferredPathID]; ok && eligible(p) {
			choice = p
		} else if len(candidates) > 0 {
			choice = candidates[0]
		}
	case Automatic:
		if len(candidates) > 0 {
			choice = candidates[0]
		}
	}
	if choice.ID == "" {
		s.activeID = ""
		return Decision{}, ErrNoAvailablePath
	}

	previous, activeHealthy := s.paths[s.activeID]
	activeHealthy = activeHealthy && eligible(previous)
	if s.activeID != "" && activeHealthy && s.activeID != choice.ID {
		// Manual mode is an explicit user command, not a suggestion.
		if s.policy.Mode != Manual && now.Sub(s.lastSwitch) < s.policy.MinDwell {
			choice = previous
		} else if s.policy.Mode == Automatic && score(choice) >= score(previous)*(1-s.policy.MinImprovementRatio) {
			choice = previous
		}
	}

	changed := choice.ID != s.activeID
	if changed {
		s.lastSwitch = now
		s.activeID = choice.ID
	}
	return Decision{PathID: choice.ID, Kind: choice.Kind, Changed: changed}, nil
}

// score is only a relative heuristic. It is not an end-to-end latency, DPI
// resistance, throughput or security estimate.
func score(p Path) float64 {
	ms := float64(p.RTT) / float64(time.Millisecond)
	result := (ms + 1) * (1 + 4*p.LossRatio)
	if p.State == Degraded {
		result *= 2
	}
	return result
}

func (s *Session) ActivePathID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeID
}

func (s *Session) Paths() []Path {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Path, 0, len(s.paths))
	for _, p := range s.paths {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.activeID = ""
	clear(s.paths)
}
