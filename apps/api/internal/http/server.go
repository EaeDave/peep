// Package http wires routes, auth, CORS and rate limiting for room-api.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"

	"peep/api/internal/rooms"
	"peep/api/internal/token"
)

type Config struct {
	APIKey, APISecret string
	InviteSecret      string
	PublicWSURL       string
	AllowedOrigins    []string
}

type Server struct {
	cfg   Config
	store *rooms.Store
	lk    *lksdk.RoomServiceClient
	rl    *limiter
}

func New(cfg Config, store *rooms.Store, lk *lksdk.RoomServiceClient) *Server {
	return &Server{cfg: cfg, store: store, lk: lk, rl: newLimiter(30, time.Minute)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("POST /rooms", s.auth(s.createRoom))
	mux.Handle("POST /rooms/{code}/join", s.auth(s.joinRoom))
	mux.Handle("GET /rooms/{code}", s.auth(s.getRoom))
	mux.Handle("POST /rooms/{code}/stats", s.auth(s.roomStats))
	return s.cors(s.rateLimit(mux))
}

// --- handlers ---

type nameBody struct {
	Name string `json:"name"`
}

func readName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var b nameBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return "", false
	}
	name, err := token.CleanName(b.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return name, true
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	if _, ok := readName(w, r); !ok {
		return
	}
	code, exp := s.store.Create(time.Now())
	slog.Info("room created", "code", code)
	writeJSON(w, http.StatusCreated, map[string]any{"code": code, "expiresAt": exp.UTC().Format(time.RFC3339)})
}

func (s *Server) code(w http.ResponseWriter, r *http.Request) (string, bool) {
	code, ok := rooms.Normalize(r.PathValue("code"))
	if !ok || !s.store.Exists(code, time.Now()) {
		writeErr(w, http.StatusNotFound, "room not found")
		return "", false
	}
	return code, true
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	name, ok := readName(w, r)
	if !ok {
		return
	}
	jwt, err := token.Issue(s.cfg.APIKey, s.cfg.APISecret, code, name)
	if err != nil {
		slog.Error("issue token", "err", err)
		writeErr(w, http.StatusInternalServerError, "token error")
		return
	}
	slog.Info("join", "code", code)
	writeJSON(w, http.StatusOK, map[string]any{"token": jwt, "url": s.cfg.PublicWSURL, "room": code})
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	n := 0
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	res, err := s.lk.ListRooms(ctx, &livekit.ListRoomsRequest{Names: []string{code}})
	if err != nil {
		slog.Warn("livekit list rooms", "err", err)
	} else if len(res.Rooms) > 0 {
		n = int(res.Rooms[0].NumParticipants)
	}
	writeJSON(w, http.StatusOK, map[string]any{"exists": true, "participants": n})
}

// roomStats logs a client's periodic WebRTC quality sample (diagnostics only;
// nothing is stored). Read them with: docker logs <room-api> | grep '"stats"'.
func (s *Server) roomStats(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	var sample map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&sample); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	slog.Info("stats", "code", code, "sample", sample)
	w.WriteHeader(http.StatusNoContent)
}

// SweepLoop expires codes every interval using LiveKit's live room list.
func (s *Server) SweepLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s.rl.prune(time.Now())
		lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res, err := s.lk.ListRooms(lctx, &livekit.ListRoomsRequest{})
		cancel()
		if err != nil {
			// Without LiveKit's view we can't tell empty rooms apart; only skip this round.
			slog.Warn("sweep: livekit list rooms", "err", err)
			continue
		}
		live := make(map[string]bool, len(res.Rooms))
		for _, rm := range res.Rooms {
			live[rm.Name] = true
		}
		if n := s.store.Sweep(time.Now(), live); n > 0 {
			slog.Info("sweep", "expired", n)
		}
	}
}

// --- middleware ---

func (s *Server) auth(h http.HandlerFunc) http.Handler {
	want := []byte("Bearer " + s.cfg.InviteSecret)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			writeErr(w, http.StatusUnauthorized, "invalid invite secret")
			return
		}
		h(w, r)
	})
}

func (s *Server) cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" {
			for _, a := range s.cfg.AllowedOrigins {
				if o == a {
					w.Header().Set("Access-Control-Allow-Origin", o)
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Max-Age", "600")
					w.Header().Add("Vary", "Origin")
					break
				}
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimit(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && !s.rl.allow(clientIP(r), time.Now()) {
			writeErr(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		h.ServeHTTP(w, r)
	})
}

// clientIP trusts the right-most X-Forwarded-For entry (appended by Traefik).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiter is a fixed-window counter per IP.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	m      map[string]*bucket
}

type bucket struct {
	start time.Time
	n     int
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, m: map[string]*bucket{}}
}

func (l *limiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.m[ip]
	if b == nil || now.Sub(b.start) >= l.window {
		b = &bucket{start: now}
		l.m[ip] = b
	}
	b.n++
	return b.n <= l.max
}

func (l *limiter) prune(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, b := range l.m {
		if now.Sub(b.start) >= l.window {
			delete(l.m, ip)
		}
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
