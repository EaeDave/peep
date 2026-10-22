package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/livekit/protocol/livekit"

	"peep/api/internal/rooms"
)

// AdminTTL is how long an admin login lasts. Tokens are stateless (HMAC), so a
// restart keeps them valid; changing ADMIN_PASSWORD revokes them all.
const AdminTTL = 8 * time.Hour

// adminKey signs admin tokens. Derived from the password and the LiveKit secret
// so it never needs its own env var.
func (s *Server) adminKey() []byte {
	m := hmac.New(sha256.New, []byte(s.cfg.APISecret))
	m.Write([]byte("peep-admin\x00" + s.cfg.AdminUser + "\x00" + s.cfg.AdminPassword))
	return m.Sum(nil)
}

func (s *Server) signAdmin(exp time.Time) string {
	e := strconv.FormatInt(exp.Unix(), 10)
	m := hmac.New(sha256.New, s.adminKey())
	m.Write([]byte(e))
	return e + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *Server) validAdmin(tok string, now time.Time) bool {
	e, _, ok := strings.Cut(tok, ".")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(e, 10, 64)
	if err != nil || now.Unix() >= exp {
		return false
	}
	return hmac.Equal([]byte(tok), []byte(s.signAdmin(time.Unix(exp, 0))))
}

func (s *Server) adminEnabled() bool { return s.cfg.AdminUser != "" && s.cfg.AdminPassword != "" }

func (s *Server) adminAuth(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !s.validAdmin(tok, time.Now()) {
			writeErr(w, http.StatusUnauthorized, "admin session expired")
			return
		}
		h(w, r)
	})
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	if !s.loginRL.allow(clientIP(r), time.Now()) {
		writeErr(w, http.StatusTooManyRequests, "rate limited")
		return
	}
	var b struct{ User, Password string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	// Both compared in constant time, always both, so timing reveals nothing.
	u := subtle.ConstantTimeCompare([]byte(b.User), []byte(s.cfg.AdminUser))
	p := subtle.ConstantTimeCompare([]byte(b.Password), []byte(s.cfg.AdminPassword))
	if u&p != 1 {
		slog.Warn("admin login failed", "ip", clientIP(r))
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	exp := time.Now().Add(AdminTTL)
	slog.Info("admin login", "ip", clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"token": s.signAdmin(exp), "expiresAt": exp.UTC()})
}

type adminPerson struct {
	Identity string    `json:"identity"`
	Name     string    `json:"name"`
	JoinedAt time.Time `json:"joinedAt"`
	Mic      bool      `json:"mic"`
	Camera   bool      `json:"camera"`
	Screen   bool      `json:"screen"`
}

type adminRoom struct {
	Code      string        `json:"code"`
	CreatedAt time.Time     `json:"createdAt"`
	People    []adminPerson `json:"people"`
}

// adminRooms lists LiveKit's live rooms with their participants (no storage:
// everything comes from LiveKit, plus the code's creation time from memory).
func (s *Server) adminRooms(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	res, err := s.lk.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		slog.Warn("admin: livekit list rooms", "err", err)
		writeErr(w, http.StatusBadGateway, "livekit unavailable")
		return
	}
	out := make([]adminRoom, 0, len(res.Rooms))
	for _, rm := range res.Rooms {
		room := adminRoom{Code: rm.Name, CreatedAt: time.Unix(rm.CreationTime, 0).UTC(), People: []adminPerson{}}
		if t, ok := s.store.CreatedAt(rm.Name); ok {
			room.CreatedAt = t.UTC()
		}
		ps, err := s.lk.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: rm.Name})
		if err != nil {
			slog.Warn("admin: livekit list participants", "room", rm.Name, "err", err)
		} else {
			for _, p := range ps.Participants {
				ap := adminPerson{Identity: p.Identity, Name: p.Name, JoinedAt: time.Unix(p.JoinedAt, 0).UTC()}
				for _, t := range p.Tracks {
					on := !t.Muted
					switch t.Source {
					case livekit.TrackSource_MICROPHONE:
						ap.Mic = ap.Mic || on
					case livekit.TrackSource_CAMERA:
						ap.Camera = ap.Camera || on
					case livekit.TrackSource_SCREEN_SHARE:
						ap.Screen = true
					}
				}
				room.People = append(room.People, ap)
			}
			sort.Slice(room.People, func(i, j int) bool { return room.People[i].JoinedAt.Before(room.People[j].JoinedAt) })
		}
		out = append(out, room)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	writeJSON(w, http.StatusOK, map[string]any{"rooms": out})
}

// adminCode only normalizes: the room may be live in LiveKit even when the
// in-memory code was lost (API restart).
func adminCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	code, ok := rooms.Normalize(r.PathValue("code"))
	if !ok {
		writeErr(w, http.StatusNotFound, "room not found")
	}
	return code, ok
}

// adminClose disconnects everyone and invalidates the code.
func (s *Server) adminClose(w http.ResponseWriter, r *http.Request) {
	code, ok := adminCode(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	s.store.Delete(code)
	if _, err := s.lk.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: code}); err != nil {
		slog.Warn("admin: delete room", "code", code, "err", err)
	}
	slog.Info("admin closed room", "code", code, "ip", clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}

// adminKick removes one participant; they can rejoin while the code is valid.
func (s *Server) adminKick(w http.ResponseWriter, r *http.Request) {
	code, ok := adminCode(w, r)
	if !ok {
		return
	}
	var b struct{ Identity string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&b); err != nil || b.Identity == "" {
		writeErr(w, http.StatusBadRequest, "identity required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	if _, err := s.lk.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: code, Identity: b.Identity}); err != nil {
		slog.Warn("admin: remove participant", "code", code, "err", err)
		writeErr(w, http.StatusNotFound, "participant not found")
		return
	}
	slog.Info("admin removed participant", "code", code, "identity", b.Identity, "ip", clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
