// Package chat keeps each room's messages in memory and its uploads on disk.
// Everything is ephemeral: it is dropped when the room's code dies.
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxText      = 2000
	MaxFile      = 100 << 20 // per upload
	MaxRoomBytes = 1 << 30   // all uploads of one room
	MaxMessages  = 1000      // older messages (and their files) are dropped
)

var (
	ErrBadText   = errors.New("message must be up to 2000 characters")
	ErrEmpty     = errors.New("message needs text or a file")
	ErrNoFile    = errors.New("file not found")
	ErrTooLarge  = errors.New("file larger than 100 MB")
	ErrRoomFull  = errors.New("room upload quota exceeded")
	ErrBadType   = errors.New("only images and videos are allowed")
	errNotExists = errors.New("not found")
)

// Allowed are the upload types. No SVG/HTML: files are served from the API origin.
var Allowed = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"video/mp4": true, "video/webm": true, "video/quicktime": true,
}

type File struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type Message struct {
	ID       int64     `json:"id"`
	Author   string    `json:"author"`
	Identity string    `json:"identity"`
	Text     string    `json:"text,omitempty"`
	File     *File     `json:"file,omitempty"`
	At       time.Time `json:"at"`
}

type room struct {
	msgs  []Message
	next  int64
	bytes int64
}

type stored struct {
	File
	room string
	path string
}

// Store is safe for concurrent use.
type Store struct {
	dir   string
	mu    sync.Mutex
	rooms map[string]*room
	files map[string]stored
}

// New empties dir (uploads of a previous process are unreachable) and uses it.
func New(dir string) (*Store, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, rooms: map[string]*room{}, files: map[string]stored{}}, nil
}

func (s *Store) room(code string) *room {
	r := s.rooms[code]
	if r == nil {
		r = &room{next: 1}
		s.rooms[code] = r
	}
	return r
}

// CleanText trims and validates a message body (newlines and tabs allowed).
func CleanText(t string) (string, error) {
	t = strings.TrimSpace(t)
	if !utf8.ValidString(t) || utf8.RuneCountInString(t) > MaxText ||
		strings.IndexFunc(t, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return "", ErrBadText
	}
	return t, nil
}

// CleanFileName keeps a display-safe base name.
func CleanFileName(n string) string {
	n = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, n)
	n = strings.TrimSpace(n)
	if r := []rune(n); len(r) > 120 {
		n = string(r[len(r)-120:])
	}
	if n == "" || !utf8.ValidString(n) {
		return "arquivo"
	}
	return n
}

// Post appends a message; fileID (optional) must be an upload of the same room.
func (s *Store) Post(code, author, identity, text, fileID string, now time.Time) (Message, error) {
	if text == "" && fileID == "" {
		return Message{}, ErrEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := Message{Author: author, Identity: identity, Text: text, At: now.UTC()}
	if fileID != "" {
		f, ok := s.files[fileID]
		if !ok || f.room != code {
			return Message{}, ErrNoFile
		}
		m.File = &f.File
	}
	r := s.room(code)
	m.ID = r.next
	r.next++
	r.msgs = append(r.msgs, m)
	if over := len(r.msgs) - MaxMessages; over > 0 {
		for _, old := range r.msgs[:over] {
			if old.File != nil {
				s.removeFile(r, old.File.ID)
			}
		}
		r.msgs = append([]Message(nil), r.msgs[over:]...)
	}
	return m, nil
}

// History returns a copy of the room's messages, oldest first.
func (s *Store) History(code string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[code]
	if r == nil {
		return []Message{}
	}
	return append([]Message(nil), r.msgs...)
}

// Upload streams body (at most MaxFile bytes) to disk for the room.
func (s *Store) Upload(code, name, typ string, body io.Reader) (File, error) {
	if !Allowed[typ] {
		return File{}, ErrBadType
	}
	id := newID()
	path := filepath.Join(s.dir, id)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return File{}, err
	}
	n, err := io.Copy(f, io.LimitReader(body, MaxFile+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > MaxFile {
		err = ErrTooLarge
	}
	if err == nil && n == 0 {
		err = ErrEmpty
	}
	if err != nil {
		os.Remove(path)
		return File{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.room(code)
	if r.bytes+n > MaxRoomBytes {
		os.Remove(path)
		return File{}, ErrRoomFull
	}
	r.bytes += n
	file := File{ID: id, Name: CleanFileName(name), Type: typ, Size: n}
	s.files[id] = stored{File: file, room: code, path: path}
	return file, nil
}

// Open returns an upload for serving; the caller closes it.
func (s *Store) Open(id string) (*os.File, File, error) {
	s.mu.Lock()
	f, ok := s.files[id]
	s.mu.Unlock()
	if !ok {
		return nil, File{}, errNotExists
	}
	fh, err := os.Open(f.path)
	return fh, f.File, err
}

// Delete drops a room's messages and uploads.
func (s *Store) Delete(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[code]
	if r == nil {
		return
	}
	for id, f := range s.files {
		if f.room == code {
			s.removeFile(r, id)
		}
	}
	delete(s.rooms, code)
}

func (s *Store) removeFile(r *room, id string) {
	f, ok := s.files[id]
	if !ok {
		return
	}
	os.Remove(f.path)
	r.bytes -= f.Size
	delete(s.files, id)
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
