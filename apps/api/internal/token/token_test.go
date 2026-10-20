package token

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/livekit/protocol/auth"
)

func TestCleanName(t *testing.T) {
	ok := []string{"Rafa", "  Zé  ", strings.Repeat("á", 24)}
	bad := []string{"", "   ", strings.Repeat("a", 25), "a\x00b", "a\nb", "a\u0085b"}
	for _, n := range ok {
		if _, err := CleanName(n); err != nil {
			t.Errorf("CleanName(%q) rejected", n)
		}
	}
	for _, n := range bad {
		if _, err := CleanName(n); err == nil {
			t.Errorf("CleanName(%q) accepted", n)
		}
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Rafa": "rafa", "João Pé": "jo-o-p", "!!!": "user", "A  B": "a-b"} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestIssue(t *testing.T) {
	jwt, err := Issue("key", "secretsecretsecretsecretsecret12", "KJ7-29F", "Rafa")
	if err != nil {
		t.Fatal(err)
	}
	v, err := auth.ParseAPIToken(jwt)
	if err != nil {
		t.Fatal(err)
	}
	_, c, err := v.Verify("secretsecretsecretsecretsecret12")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^rafa-[a-z0-9]{4}$`).MatchString(v.Identity()) {
		t.Errorf("identity %q", v.Identity())
	}
	g := c.Video
	if c.Name != "Rafa" || !g.RoomJoin || g.Room != "KJ7-29F" || !*g.CanPublish || !*g.CanSubscribe {
		t.Errorf("grants %+v name %q", g, c.Name)
	}
	if !slices.Equal(g.CanPublishSources, []string{"camera", "microphone", "screen_share", "screen_share_audio"}) {
		t.Errorf("sources %v", g.CanPublishSources)
	}
	if _, _, err := v.Verify("wrong-secret-wrong-secret-wrong!"); err == nil {
		t.Error("verified with wrong secret")
	}
}
