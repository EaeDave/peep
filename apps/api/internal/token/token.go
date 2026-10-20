// Package token issues LiveKit access tokens and validates display names.
package token

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/livekit/protocol/auth"
)

const TTL = 6 * time.Hour

var ErrBadName = errors.New("name must be 1-24 characters without control characters")

// CleanName trims and validates a display name.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 24 || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", ErrBadName
	}
	return name, nil
}

// Slug turns a display name into a lowercase [a-z0-9-] identifier prefix.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimSuffix(b.String(), "-")
	if s == "" {
		return "user"
	}
	return s
}

func suffix() string {
	const chars = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// Issue builds a JWT for name to join room. name must already be cleaned.
func Issue(apiKey, apiSecret, room, name string) (string, error) {
	yes := true
	return auth.NewAccessToken(apiKey, apiSecret).
		SetIdentity(Slug(name) + "-" + suffix()).
		SetName(name).
		SetValidFor(TTL).
		SetVideoGrant(&auth.VideoGrant{
			RoomJoin:          true,
			Room:              room,
			CanPublish:        &yes,
			CanSubscribe:      &yes,
			CanPublishSources: []string{"camera", "microphone", "screen_share", "screen_share_audio"},
		}).
		ToJWT()
}
