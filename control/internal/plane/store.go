package plane

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	stateFile = "state.json"
	maxStateBytes = 16 << 20
	maxAuditEvents = 5000
)

type Store struct {
	mu    sync.RWMutex
	dir   string
	lock  *os.File
	state State
}

// OpenStore opens the single-writer Linux alpha store. The lock prevents
// accidentally starting two control instances on one data directory.
func OpenStore(dir string) (*Store, error) {
	if !filepath.IsAbs(dir) { return nil, errors.New("data directory must be absolute") }
	if err := os.MkdirAll(dir, 0700); err != nil { return nil, err }
	info, err := os.Lstat(dir)
	if err != nil { return nil, err }
	if !info.IsDir() || info.Mode().Perm() & 0077 != 0 {
		return nil, errors.New("data directory must be private (0700) and not a symlink")
	}
	lockPath := filepath.Join(dir, ".writer.lock")
	if _, err := os.Lstat(lockPath); err == nil {
		f, err := os.Lstat(lockPath)
		if err != nil || !f.Mode().IsRegular() || f.Mode().Perm() & 0077 != 0 {
			return nil, errors.New("lock file must be a private regular file")
		}
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil { return nil, err }
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("data directory is already in use by another control-plane process")
	}
	s := &Store{dir: dir, lock: lock}
	f, err := os.OpenFile(filepath.Join(dir, stateFile), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err == nil {
		defer f.Close()
		fi, err := f.Stat()
		if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm() & 0077 != 0 || fi.Size() > maxStateBytes {
			s.Close()
			return nil, errors.New("state file must be a private regular file <=16 MiB")
		}
		decoder := json.NewDecoder(io.LimitReader(f, maxStateBytes+1))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&s.state); err != nil {
			s.Close()
			return nil, fmt.Errorf("invalid state: %w", err)
		}
		var more any
		if err := decoder.Decode(&more); err != io.EOF || s.state.SchemaVersion != schemaVersion {
			s.Close()
			return nil, errors.New("unknown state schema or trailing content; migration required")
		}
		if s.state.Users == nil || s.state.Devices == nil || s.state.Gateways == nil || s.state.Policies == nil || s.state.Tokens == nil {
			s.Close()
			return nil, errors.New("state maps missing; refusing to reset existing data")
		}
		return s, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		s.Close()
		return nil, fmt.Errorf("cannot read state: %w", err)
	}
	// Refuse a half-initialized directory; never silently rotate credentials.
	bootstrapPath := filepath.Join(dir, "bootstrap.token")
	if _, err := os.Lstat(bootstrapPath); err == nil {
		s.Close()
		return nil, errors.New("bootstrap token exists but state does not; manual recovery required")
	} else if !errors.Is(err, os.ErrNotExist) {
		s.Close()
		return nil, err
	}
	raw, id, hash, err := newSecret()
	if err != nil { s.Close(); return nil, err }
	now := time.Now().UTC()
	s.state = newState()
	s.state.Tokens[id] = Token{ID:id, Name:"bootstrap-owner", Role:"owner", Digest:hash, CreatedAt:now}
	s.state.Audit = append(s.state.Audit, AuditEvent{ID:randomID("evt"), At:now, ActorID:"system", Action:"bootstrap", Target:id})
	bf, err := os.OpenFile(bootstrapPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil { s.Close(); return nil, err }
	if _, err = io.WriteString(bf, raw+"\n"); err == nil { err = bf.Sync() }
	if e := bf.Close(); err == nil { err = e }
	if err != nil { os.Remove(bootstrapPath); s.Close(); return nil, err }
	if err = s.save(s.state); err != nil { os.Remove(bootstrapPath); s.Close(); return nil, err }
	return s, nil
}

// Close releases the exclusive store lock.
func (s *Store) Close() error {
	if s == nil || s.lock == nil { return nil }
	err := syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	e := s.lock.Close()
	s.lock = nil
	if err != nil { return err }
	return e
}

func newSecret() (string, string, string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil { return "", "", "", err }
	token := "mrkvpn_" + base64.RawURLEncoding.EncodeToString(data)
	id := randomID("tok")
	sum := sha256.Sum256([]byte(token))
	return token, id, hex.EncodeToString(sum[:]), nil
}

func randomID(prefix string) string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil { panic("OS cryptographic RNG unavailable: " + err.Error()) }
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(data)
}

func (s *Store) authenticate(raw string) (Token, bool) {
	sum := sha256.Sum256([]byte(raw))
	candidate := hex.EncodeToString(sum[:])
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.state.Tokens {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(t.Digest)) == 1 && !t.Revoked &&
			(t.ExpiresAt.IsZero() || t.ExpiresAt.After(time.Now().UTC())) {
			return t, true
		}
	}
	return Token{}, false
}

func (s *Store) snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.state)
}

func cloneState(st State) State {
	raw, err := json.Marshal(st)
	if err != nil { panic(err) }
	var out State
	if err := json.Unmarshal(raw, &out); err != nil { panic(err) }
	return out
}

func (s *Store) change(actor, action, target string, fn func(*State) (any, error)) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	result, err := fn(&next)
	if err != nil { return nil, err }
	next.Audit = append(next.Audit, AuditEvent{ID:randomID("evt"), At:time.Now().UTC(), ActorID:actor, Action:action, Target:target})
	if len(next.Audit) > maxAuditEvents { next.Audit = next.Audit[len(next.Audit)-maxAuditEvents:] }
	if err := s.save(next); err != nil { return nil, err }
	s.state = next
	return result, nil
}

func (s *Store) save(state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil { return err }
	if len(data) > maxStateBytes { return errors.New("state capacity limit exceeded") }
	tmp, err := os.CreateTemp(s.dir, ".state-*.tmp")
	if err != nil { return err }
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil { tmp.Close(); return err }
	if _, err := tmp.Write(data); err != nil { tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(name, filepath.Join(s.dir, stateFile)); err != nil { return err }
	d, err := os.Open(s.dir)
	if err != nil { return err }
	defer d.Close()
	return d.Sync()
}
