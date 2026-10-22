package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/webhook"
	"peep/api/internal/rooms"
)

// Event IDs suppress LiveKit retries within this API process. No durable queue:
// Discord outages are retried by LiveKit; API-created notices are best effort.
type discordEvents struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

var discordClient = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (s *Server) notifyDiscord(ctx context.Context, text string) error {
	if s.cfg.DiscordWebhookURL == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]any{"content": text, "allowed_mentions": map[string]any{"parse": []string{}}})
	if err != nil {
		return errors.New("discord payload invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.DiscordWebhookURL, bytes.NewReader(payload))
	if err != nil {
		return errors.New("discord URL invalid")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := discordClient.Do(req)
	// Transport errors include the secret URL: never propagate or log them.
	if err != nil {
		return errors.New("discord delivery failed")
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("discord returned HTTP %d", res.StatusCode)
	}
	return nil
}

func (s *Server) livekitWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	event, err := webhook.ReceiveWebhookEvent(r, auth.NewSimpleKeyProvider(s.cfg.APIKey, s.cfg.APISecret))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}
	code, valid := rooms.Normalize(event.GetRoom().GetName())
	if !valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var text string
	switch event.Event {
	case "participant_joined", "participant_left":
		p := event.GetParticipant()
		if p == nil {
			writeErr(w, http.StatusBadRequest, "missing participant")
			return
		}
		name := p.Name
		if name == "" {
			name = p.Identity
		}
		verb := "entrou na"
		if event.Event == "participant_left" {
			verb = "saiu da"
		}
		text = fmt.Sprintf("👤 %s %s sala %s.", name, verb, code)
	case "room_finished":
		text = fmt.Sprintf("🔴 Sala %s encerrada.", code)
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.discord.mu.Lock()
	// ponytail: serialize webhook deliveries; per-room locks if event volume grows.
	defer s.discord.mu.Unlock()
	now := time.Now()
	for id, at := range s.discord.seen {
		if now.Sub(at) > 24*time.Hour {
			delete(s.discord.seen, id)
		}
	}
	if _, ok := s.discord.seen[event.Id]; event.Id != "" && ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.notifyDiscord(r.Context(), text); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if event.Event == "room_finished" {
		s.store.Delete(code)
	}
	if s.discord.seen == nil {
		s.discord.seen = make(map[string]time.Time)
	}
	if event.Id != "" {
		s.discord.seen[event.Id] = now
	}
	slog.Info("discord notified", "event", event.Event, "code", code)
	w.WriteHeader(http.StatusNoContent)
}
