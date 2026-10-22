// Package rooms generates short room codes and keeps them in memory with a TTL.
package rooms

import (
	"crypto/rand"
	"math/big"
	"strings"
	"sync"
	"time"
)

const (
	Alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	MaxAge   = 12 * time.Hour
	// EmptyGrace mirrors LiveKit's room.empty_timeout: a code whose room is not
	// live in LiveKit for this long is dropped (also covers codes never joined).
	EmptyGrace = 5 * time.Minute
)

// Normalize uppercases, strips spaces/hyphens and returns "XXX-XXX", or false.
func Normalize(in string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(in) {
		switch {
		case r == '-' || r == ' ':
			continue
		case strings.ContainsRune(Alphabet, r):
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	s := b.String()
	if len(s) != 6 {
		return "", false
	}
	return s[:3] + "-" + s[3:], true
}

func newCode() string {
	b := make([]byte, 6)
	n := big.NewInt(int64(len(Alphabet)))
	for i := range b {
		v, err := rand.Int(rand.Reader, n)
		if err != nil {
			panic(err) // crypto/rand never fails on supported platforms
		}
		b[i] = Alphabet[v.Int64()]
	}
	return string(b[:3]) + "-" + string(b[3:])
}

type entry struct{ createdAt, lastSeen time.Time }

// Store maps code → timestamps. Safe for concurrent use.
type Store struct {
	mu sync.Mutex
	m  map[string]entry
}

func NewStore() *Store { return &Store{m: map[string]entry{}} }

// Create registers a fresh unique code and returns it with its expiry.
func (s *Store) Create(now time.Time) (string, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		c := newCode()
		if _, taken := s.m[c]; !taken {
			s.m[c] = entry{now, now}
			return c, now.Add(MaxAge)
		}
	}
}

// Exists reports whether code is registered and touches lastSeen.
func (s *Store) Exists(code string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[code]
	if ok {
		e.lastSeen = now
		s.m[code] = e
	}
	return ok
}

// CreatedAt returns when code was created, if it is registered.
func (s *Store) CreatedAt(code string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[code]
	return e.createdAt, ok
}

// Delete invalidates code immediately (admin closed the room).
func (s *Store) Delete(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, code)
}

// Adopt registers a code that is live in LiveKit but unknown here (lost on an
// API restart). createdAt comes from LiveKit, so MaxAge still applies.
func (s *Store) Adopt(code string, createdAt, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[code]; !ok {
		s.m[code] = entry{createdAt, now}
	}
}

// Sweep drops codes older than MaxAge or absent from live for longer than
// EmptyGrace. live holds room names currently present in LiveKit.
func (s *Store) Sweep(now time.Time, live map[string]bool) (removed []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c, e := range s.m {
		if live[c] {
			e.lastSeen = now
			s.m[c] = e
		}
		if now.Sub(e.createdAt) > MaxAge || now.Sub(e.lastSeen) > EmptyGrace {
			delete(s.m, c)
			removed = append(removed, c)
		}
	}
	return removed
}
