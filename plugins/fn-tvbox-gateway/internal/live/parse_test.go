package live_test

import (
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/live"
)

func TestParseM3U(t *testing.T) {
	body := `#EXTM3U
#EXTINF:-1 tvg-logo="http://logo/1.png" group-title="央视",CCTV1
https://example/live/cctv1.m3u8
#EXTINF:-1 group-title="卫视",湖南卫视
https://example/live/hunan.m3u8|User-Agent=UA1
`
	chs := live.ParsePlaylist(body, "", nil)
	if len(chs) != 2 {
		t.Fatalf("len=%d %+v", len(chs), chs)
	}
	if chs[0].Group != "央视" || chs[0].Name != "CCTV1" {
		t.Fatalf("%+v", chs[0])
	}
	if chs[1].Headers["User-Agent"] != "UA1" {
		t.Fatalf("%+v", chs[1])
	}
	groups := live.Groups(chs)
	if len(groups) != 2 || groups[0].ChannelCount != 1 {
		t.Fatalf("%+v", groups)
	}
}

func TestParseGenreText(t *testing.T) {
	body := `央视,#genre#
CCTV1,https://example/cctv1.m3u8
卫视,#genre#
湖南卫视,https://example/hunan.m3u8
`
	chs := live.ParsePlaylist(body, "", nil)
	if len(chs) != 2 {
		t.Fatalf("%+v", chs)
	}
	filtered := live.FilterChannels(chs, "央视", "")
	if len(filtered) != 1 || filtered[0].Name != "CCTV1" {
		t.Fatalf("%+v", filtered)
	}
}
