package chat

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUploadPostDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "chat")
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	if _, err := s.Upload("AAA-AAA", "x.svg", "image/svg+xml", strings.NewReader("<svg/>")); !errors.Is(err, ErrBadType) {
		t.Fatalf("svg accepted: %v", err)
	}
	f, err := s.Upload("AAA-AAA", "../../etc/foto.png", "image/png", strings.NewReader("png"))
	if err != nil || f.Size != 3 || f.Name != "....etcfoto.png" {
		t.Fatalf("upload: %+v %v", f, err)
	}
	// Uploaded but not posted yet: history is an empty list, not JSON null.
	if h := s.History("AAA-AAA"); h == nil || len(h) != 0 {
		t.Fatalf("history before any post: %#v", h)
	}
	// A file id from another room can't be attached.
	if _, err := s.Post("BBB-BBB", "Rafa", "rafa-1", "", f.ID, now); !errors.Is(err, ErrNoFile) {
		t.Fatalf("cross-room attach: %v", err)
	}
	m, err := s.Post("AAA-AAA", "Rafa", "rafa-1", "olha", f.ID, now)
	if err != nil || m.ID != 1 || m.File == nil {
		t.Fatalf("post: %+v %v", m, err)
	}
	if h := s.History("AAA-AAA"); len(h) != 1 || len(s.History("BBB-BBB")) != 0 {
		t.Fatalf("history: %+v", h)
	}

	s.Delete("AAA-AAA")
	if len(s.History("AAA-AAA")) != 0 {
		t.Fatal("history survived delete")
	}
	if _, _, err := s.Open(f.ID); err == nil {
		t.Fatal("file survived delete")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Fatalf("files left on disk: %d", len(left))
	}
}

func TestLimits(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CleanText(strings.Repeat("é", MaxText+1)); err == nil {
		t.Fatal("long text accepted")
	}
	if _, err := s.Upload("AAA-AAA", "v.mp4", "video/mp4", strings.NewReader(strings.Repeat("x", MaxFile+1))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized upload: %v", err)
	}

	// Room quota: freed again when old messages roll off the history.
	r := s.room("AAA-AAA")
	r.bytes = MaxRoomBytes - 2
	if _, err := s.Upload("AAA-AAA", "a.png", "image/png", strings.NewReader("abc")); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("quota: %v", err)
	}
	r.bytes = 0
	f, _ := s.Upload("AAA-AAA", "a.png", "image/png", strings.NewReader("abc"))
	s.Post("AAA-AAA", "Rafa", "rafa-1", "", f.ID, time.Now())
	for range MaxMessages {
		s.Post("AAA-AAA", "Rafa", "rafa-1", "oi", "", time.Now())
	}
	if h := s.History("AAA-AAA"); len(h) != MaxMessages || h[0].ID != 2 {
		t.Fatalf("history not trimmed: %d first=%d", len(h), h[0].ID)
	}
	if r.bytes != 0 {
		t.Fatalf("rolled-off file still counted: %d", r.bytes)
	}
}
