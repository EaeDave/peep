package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/livekit/protocol/livekit"

	"peep/api/internal/chat"
	"peep/api/internal/token"
)

// ChatTopic is the LiveKit data topic clients listen on for new messages.
const ChatTopic = "chat"

func (s *Server) chatHistory(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": s.chat.History(code)})
}

// chatPost stores a message and pushes it to everyone in the room (sender included).
func (s *Server) chatPost(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	var b struct{ Name, Identity, Text, File string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	name, err := token.CleanName(b.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if b.Identity == "" || len(b.Identity) > 64 || !utf8.ValidString(b.Identity) {
		writeErr(w, http.StatusBadRequest, "invalid identity")
		return
	}
	text, err := chat.CleanText(b.Text)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := s.chat.Post(code, name, b.Identity, text, b.File, time.Now())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	data, _ := json.Marshal(m)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	topic := ChatTopic
	if _, err := s.lk.SendData(ctx, &livekit.SendDataRequest{Room: code, Data: data, Kind: livekit.DataPacket_RELIABLE, Topic: &topic}); err != nil {
		// Stored anyway: clients reload the history when they (re)connect.
		slog.Warn("chat: livekit send data", "code", code, "err", err)
	}
	writeJSON(w, http.StatusCreated, m)
}

// chatUpload stores the raw body (an image or video, Content-Type required);
// the client then posts a message referencing the returned file id.
func (s *Server) chatUpload(w http.ResponseWriter, r *http.Request) {
	code, ok := s.code(w, r)
	if !ok {
		return
	}
	typ, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !chat.Allowed[typ] {
		writeErr(w, http.StatusUnsupportedMediaType, chat.ErrBadType.Error())
		return
	}
	if r.ContentLength > chat.MaxFile {
		writeErr(w, http.StatusRequestEntityTooLarge, chat.ErrTooLarge.Error())
		return
	}
	f, err := s.chat.Upload(code, r.URL.Query().Get("name"), typ, r.Body)
	switch {
	case err == nil:
		slog.Info("chat upload", "code", code, "type", typ, "size", f.Size)
		writeJSON(w, http.StatusCreated, f)
	case errors.Is(err, chat.ErrTooLarge), errors.Is(err, chat.ErrRoomFull):
		writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, chat.ErrEmpty):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		slog.Error("chat upload", "code", code, "err", err)
		writeErr(w, http.StatusInternalServerError, "upload failed")
	}
}

// chatFile serves an upload (Range supported, so videos seek without a full download).
func (s *Server) chatFile(w http.ResponseWriter, r *http.Request) {
	fh, f, err := s.chat.Open(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	h := w.Header()
	h.Set("Content-Type", f.Type)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": f.Name}))
	h.Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeContent(w, r, "", st.ModTime(), fh)
}
