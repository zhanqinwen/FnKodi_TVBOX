package player_test

import (
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
)

func TestIsDirectMedia(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://cdn.example/a/1.m3u8", true},
		{"https://cdn.example/a/1.mp4", true},
		{"https://cdn.example/a/1.mkv", true},
		{"https://cdn.example/a/1.flv", true},
		{"https://cdn.example/page.html", false},
		{"https://cdn.example/play", false},
		{"https://cdn.example/1.m3u8|User-Agent=x", true},
	}
	for _, tc := range cases {
		if got := player.IsDirectMedia(tc.url); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.url, got, tc.want)
		}
	}
}
