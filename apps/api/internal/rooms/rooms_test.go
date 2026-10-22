package rooms

import (
	"regexp"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"KJ7-29F":  "KJ7-29F",
		"kj729f":   "KJ7-29F",
		" kj7 29f": "KJ7-29F",
		"KJ7-29":   "", // short
		"KJ7-29FF": "", // long
		"KJ0-29F":  "", // ambiguous 0
		"KJ1-29F":  "", // ambiguous 1
		"KJI-29F":  "", // ambiguous I
		"KJ7_29F":  "", // junk
	}
	for in, want := range cases {
		got, ok := Normalize(in)
		if ok != (want != "") || got != want {
			t.Errorf("Normalize(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
}

func TestCreateFormatAndRoundTrip(t *testing.T) {
	re := regexp.MustCompile(`^[` + Alphabet + `]{3}-[` + Alphabet + `]{3}$`)
	s := NewStore()
	now := time.Now()
	for range 1000 {
		c, exp := s.Create(now)
		if !re.MatchString(c) {
			t.Fatalf("bad code %q", c)
		}
		if n, _ := Normalize(c); n != c {
			t.Fatalf("code %q not stable under Normalize", c)
		}
		if !exp.Equal(now.Add(MaxAge)) {
			t.Fatalf("expiry %v", exp)
		}
	}
	if len(s.m) != 1000 {
		t.Fatalf("collisions not avoided: %d", len(s.m))
	}
}

func TestSweep(t *testing.T) {
	s := NewStore()
	t0 := time.Now()
	live, _ := s.Create(t0)
	idle, _ := s.Create(t0)

	// within grace: both kept
	if removed := s.Sweep(t0.Add(EmptyGrace-time.Second), nil); len(removed) != 0 {
		t.Fatalf("removed %v within grace", removed)
	}
	// past grace: only the room LiveKit reports as live survives
	if removed := s.Sweep(t0.Add(EmptyGrace+time.Second), map[string]bool{live: true}); len(removed) != 1 || removed[0] != idle {
		t.Fatalf("removed %v, want idle code %s", removed, idle)
	}
	if s.Exists(idle, t0) || !s.Exists(live, t0) {
		t.Fatal("wrong code swept")
	}
	// hard 12h cap even if still live
	if removed := s.Sweep(t0.Add(MaxAge+time.Second), map[string]bool{live: true}); len(removed) != 1 || removed[0] != live {
		t.Fatalf("max age not enforced")
	}
}
