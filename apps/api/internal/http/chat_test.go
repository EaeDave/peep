package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"

	"peep/api/internal/chat"
	"peep/api/internal/rooms"
)

type sentLK struct {
	fakeLK
	sent []*livekit.SendDataRequest
}

func (f *sentLK) SendData(_ context.Context, req *livekit.SendDataRequest) (*livekit.SendDataResponse, error) {
	f.sent = append(f.sent, req)
	return &livekit.SendDataResponse{}, nil
}

// Upload → message → broadcast → history → file served inline, safely, with Range.
func TestChatFlow(t *testing.T) {
	store := rooms.NewStore()
	code, _ := store.Create(time.Now())
	chats, err := chat.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lk := &sentLK{}
	h := New(Config{InviteSecret: "inv"}, store, lk, chats).Handler()
	do := func(method, path, ctype, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer inv")
		if ctype != "" {
			r.Header.Set("Content-Type", ctype)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	if w := do("POST", "/rooms/"+code+"/files?name=a.html", "text/html", "<script>"); w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("html upload: %d", w.Code)
	}
	w := do("POST", "/rooms/"+code+"/files?name=clip.mp4", "video/mp4", "0123456789")
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	var f chat.File
	json.Unmarshal(w.Body.Bytes(), &f)

	w = do("POST", "/rooms/"+code+"/messages", "application/json", `{"name":"Rafa","identity":"rafa-1","text":"olha","file":"`+f.ID+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("post: %d %s", w.Code, w.Body)
	}
	if len(lk.sent) != 1 || lk.sent[0].Room != code || lk.sent[0].GetTopic() != ChatTopic || lk.sent[0].Kind != livekit.DataPacket_RELIABLE {
		t.Fatalf("broadcast: %+v", lk.sent)
	}
	if w := do("POST", "/rooms/"+code+"/messages", "application/json", `{"name":"Rafa","identity":"rafa-1","text":"  "}`); w.Code != http.StatusBadRequest {
		t.Fatalf("empty message: %d", w.Code)
	}

	var hist struct{ Messages []chat.Message }
	json.Unmarshal(do("GET", "/rooms/"+code+"/messages", "", "").Body.Bytes(), &hist)
	if len(hist.Messages) != 1 || hist.Messages[0].Text != "olha" || hist.Messages[0].File.ID != f.ID {
		t.Fatalf("history: %+v", hist)
	}

	r := httptest.NewRequest("GET", "/files/"+f.ID, nil) // no Authorization: <video src>
	r.Header.Set("Range", "bytes=2-4")
	fw := httptest.NewRecorder()
	h.ServeHTTP(fw, r)
	if fw.Code != http.StatusPartialContent || fw.Body.String() != "234" ||
		fw.Header().Get("Content-Type") != "video/mp4" || fw.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("file: %d %q %v", fw.Code, fw.Body, fw.Header())
	}
	if w := do("GET", "/files/deadbeef", "", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown file: %d", w.Code)
	}
}
