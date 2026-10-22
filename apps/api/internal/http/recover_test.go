package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"

	"peep/api/internal/rooms"
)

// fakeLK knows a fixed set of live rooms; down simulates LiveKit being unreachable.
type fakeLK struct {
	LiveKit
	live  map[string]int64 // name -> creation unix time
	down  bool
	calls int
}

func (f *fakeLK) ListRooms(_ context.Context, req *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error) {
	f.calls++
	if f.down {
		return nil, errors.New("connection refused")
	}
	out := &livekit.ListRoomsResponse{}
	for _, n := range req.Names {
		if t, ok := f.live[n]; ok {
			out.Rooms = append(out.Rooms, &livekit.Room{Name: n, CreationTime: t})
		}
	}
	return out, nil
}

func join(h http.Handler, code string) int {
	r := httptest.NewRequest("POST", "/rooms/"+code+"/join", strings.NewReader(`{"name":"Rafa"}`))
	r.Header.Set("Authorization", "Bearer inv")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// After an API restart the store is empty but the room still runs in LiveKit:
// newcomers must still get in; unknown rooms stay 404; LiveKit outages are 503.
func TestJoinRecoversRoomAfterRestart(t *testing.T) {
	lk := &fakeLK{live: map[string]int64{"GDN-K22": time.Now().Add(-time.Hour).Unix()}}
	store := rooms.NewStore() // fresh process: no codes in memory
	h := New(Config{APIKey: "k", APISecret: "secretsecretsecretsecretsecret12", InviteSecret: "inv"}, store, lk).Handler()

	if c := join(h, "gdn-k22"); c != http.StatusOK {
		t.Fatalf("live room after restart: %d", c)
	}
	if !store.Exists("GDN-K22", time.Now()) {
		t.Fatal("recovered room not adopted")
	}
	before := lk.calls
	join(h, "GDN-K22")
	if lk.calls != before {
		t.Error("adopted room still queried LiveKit")
	}
	if c := join(h, "ZZZ-ZZZ"); c != http.StatusNotFound {
		t.Errorf("unknown room: %d", c)
	}
	lk.down = true
	if c := join(h, "AAA-AAA"); c != http.StatusServiceUnavailable {
		t.Errorf("livekit down: %d, want 503 (not 'room not found')", c)
	}
}

// Adopted rooms keep LiveKit's creation time, so the 12 h cap still applies.
func TestAdoptKeepsCreationTime(t *testing.T) {
	s := rooms.NewStore()
	now := time.Now()
	s.Adopt("GDN-K22", now.Add(-rooms.MaxAge), now)
	if removed := s.Sweep(now.Add(time.Second), map[string]bool{"GDN-K22": true}); len(removed) != 1 {
		t.Error("adopted room exceeded MaxAge but was kept")
	}
}
