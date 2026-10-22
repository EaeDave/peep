package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"peep/api/internal/rooms"
)

func adminServer(user, pass string) *Server {
	return New(Config{APISecret: "lk-secret", InviteSecret: "inv", AdminUser: user, AdminPassword: pass}, rooms.NewStore(), nil)
}

func TestAdminToken(t *testing.T) {
	s := adminServer("admin", "pw")
	now := time.Now()
	tok := s.signAdmin(now.Add(time.Hour))
	if !s.validAdmin(tok, now) {
		t.Fatal("fresh token rejected")
	}
	if s.validAdmin(tok, now.Add(2*time.Hour)) {
		t.Error("expired token accepted")
	}
	// Pushing the expiry forward must break the signature.
	e, sig, _ := strings.Cut(tok, ".")
	if s.validAdmin("9"+e+"."+sig, now) {
		t.Error("tampered expiry accepted")
	}
	if s.validAdmin("", now) || s.validAdmin("x.y", now) {
		t.Error("garbage accepted")
	}
	// Changing the password revokes existing tokens.
	if adminServer("admin", "new").validAdmin(tok, now) {
		t.Error("token survived password change")
	}
}

func TestAdminLogin(t *testing.T) {
	h := adminServer("admin", "pw").Handler()
	login := func(body string) int {
		r := httptest.NewRequest("POST", "/admin/login", strings.NewReader(body))
		r.RemoteAddr = "1.2.3.4:5"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := login(`{"user":"admin","password":"pw"}`); c != http.StatusOK {
		t.Fatalf("good login: %d", c)
	}
	for _, b := range []string{`{"user":"admin","password":"no"}`, `{"user":"x","password":"pw"}`} {
		if c := login(b); c != http.StatusUnauthorized {
			t.Errorf("bad login %s: %d", b, c)
		}
	}
	// 3 attempts used; the 6th within the minute is limited even with the right password.
	login(`{}`)
	login(`{}`)
	if c := login(`{"user":"admin","password":"pw"}`); c != http.StatusTooManyRequests {
		t.Errorf("6th attempt: %d", c)
	}

	r := httptest.NewRequest("GET", "/admin/rooms", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("rooms without token: %d", w.Code)
	}
}

func TestAdminDisabledWithoutCredentials(t *testing.T) {
	h := adminServer("admin", "").Handler()
	r := httptest.NewRequest("POST", "/admin/login", strings.NewReader(`{"user":"admin","password":""}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("admin route exists without password: %d", w.Code)
	}
}
