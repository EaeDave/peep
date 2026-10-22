package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/auth"
	"peep/api/internal/rooms"
)

func TestDiscordLifecycle(t *testing.T) {
	var messages []string
	fail := false
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(429)
			return
		}
		var p struct {
			Content  string `json:"content"`
			Mentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if len(p.Mentions.Parse) != 0 {
			t.Error("Discord mentions enabled")
		}
		messages = append(messages, p.Content)
		w.WriteHeader(204)
	}))
	defer discord.Close()
	store := rooms.NewStore()
	cfg := Config{APIKey: "key", APISecret: "secretsecretsecretsecretsecret12", InviteSecret: "inv", DiscordWebhookURL: discord.URL}
	h := New(cfg, store, nil).Handler()
	req := httptest.NewRequest("POST", "/rooms", strings.NewReader(`{"name":"Rafa @everyone"}`))
	req.Header.Set("Authorization", "Bearer inv")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	var created struct {
		Code string `json:"code"`
	}
	json.Unmarshal(res.Body.Bytes(), &created)
	if res.Code != 201 || len(messages) != 1 || messages[0] != "🟢 Rafa @everyone criou a sala "+created.Code+"." {
		t.Fatalf("create: %d %v", res.Code, messages)
	}
	send := func(id, event string, signed bool) int {
		body, _ := json.Marshal(map[string]any{"id": id, "event": event, "room": map[string]string{"name": created.Code}, "participant": map[string]string{"name": "Rafa @everyone", "identity": "person"}})
		r := httptest.NewRequest("POST", "/webhooks/livekit", strings.NewReader(string(body)))
		if signed {
			hash := sha256.Sum256(body)
			jwt, err := auth.NewAccessToken(cfg.APIKey, cfg.APISecret).SetSha256(base64.StdEncoding.EncodeToString(hash[:])).ToJWT()
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", jwt)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := send("fake", "participant_joined", false); c != 401 || len(messages) != 1 {
		t.Fatal("unsigned event accepted")
	}
	if c := send("join", "participant_joined", true); c != 204 {
		t.Fatal(c)
	}
	if c := send("join", "participant_joined", true); c != 204 || len(messages) != 2 {
		t.Fatal("duplicate notification")
	}
	if messages[1] != "👤 Rafa @everyone entrou na sala "+created.Code+"." {
		t.Fatal(messages)
	}
	fail = true
	if c := send("left", "participant_left", true); c != 502 {
		t.Fatalf("failed delivery acknowledged: %d", c)
	}
	fail = false
	if c := send("left", "participant_left", true); c != 204 || messages[2] != "👤 Rafa @everyone saiu da sala "+created.Code+"." {
		t.Fatal("retry lost leave event")
	}
	if c := send("end", "room_finished", true); c != 204 || messages[3] != "🔴 Sala "+created.Code+" encerrada." {
		t.Fatal("missing closure")
	}
	if store.Exists(created.Code, time.Now()) {
		t.Fatal("finished room remains accessible")
	}
	// Secret-bearing transport errors must not escape to callers/logs.
	cfg.DiscordWebhookURL = "http://127.0.0.1:1/secret-token"
	s := New(cfg, store, nil)
	err := s.notifyDiscord(httptest.NewRequest("GET", "/", nil).Context(), "test")
	if err == nil || strings.Contains(err.Error(), "secret-token") {
		t.Fatal("secret URL leaked")
	}
}
