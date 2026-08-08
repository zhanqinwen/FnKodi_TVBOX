package tvbox_test

import (
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

func TestEpisodesE01_SingleLine(t *testing.T) {
	eps := tvbox.ParseEpisodes("", "第1集$http://a/1.m3u8#第2集$http://a/2.m3u8", "m1", nil)
	if len(eps) != 2 {
		t.Fatalf("len=%d", len(eps))
	}
	if eps[0].ID != "0:0" || eps[1].ID != "0:1" {
		t.Fatalf("ids=%s,%s", eps[0].ID, eps[1].ID)
	}
	if eps[0].Title != "第1集" || eps[0].PlayURL != "http://a/1.m3u8" {
		t.Fatalf("ep0=%+v", eps[0])
	}
	if eps[0].PlayFrom != "线路1" {
		t.Fatalf("playFrom=%s", eps[0].PlayFrom)
	}
}

func TestEpisodesE02_MultiLine(t *testing.T) {
	eps := tvbox.ParseEpisodes(
		"线路A$$$线路B",
		"第1集$http://a/1.m3u8$$$第1集$http://b/1.m3u8",
		"m1", nil,
	)
	if len(eps) != 2 {
		t.Fatalf("len=%d %+v", len(eps), eps)
	}
	if eps[0].PlayFrom != "线路A" || eps[0].ID != "0:0" {
		t.Fatalf("ep0=%+v", eps[0])
	}
	if eps[1].PlayFrom != "线路B" || eps[1].ID != "1:0" {
		t.Fatalf("ep1=%+v", eps[1])
	}
}

func TestEpisodesE03_PlayFromPipe(t *testing.T) {
	eps := tvbox.ParseEpisodes("线路|A", "第1集$http://a/1.m3u8", "m1", nil)
	if len(eps) != 1 || eps[0].ID != "0:0" {
		t.Fatalf("%+v", eps)
	}
	if eps[0].PlayFrom != "线路|A" {
		t.Fatalf("playFrom=%s", eps[0].PlayFrom)
	}
}

func TestEpisodesE04_TitleContainsHash(t *testing.T) {
	eps := tvbox.ParseEpisodes("", "第1#集$http://a/1.m3u8#第2集$http://a/2.m3u8", "m1", nil)
	if len(eps) != 2 {
		t.Fatalf("len=%d %+v", len(eps), eps)
	}
	if eps[0].Title != "第1#集" || eps[0].PlayURL != "http://a/1.m3u8" {
		t.Fatalf("ep0=%+v", eps[0])
	}
}

func TestEpisodesE05_TripleDollarOnlyGroupSep(t *testing.T) {
	// $$$ is only a group separator; two groups map to playFrom A/B.
	eps := tvbox.ParseEpisodes(
		"线路A$$$线路B",
		"标题含普通字$http://a/1.m3u8$$$另一组$http://b/1.m3u8",
		"m1", nil,
	)
	if len(eps) != 2 {
		t.Fatalf("len=%d", len(eps))
	}
	if eps[0].PlayFrom != "线路A" || eps[1].PlayFrom != "线路B" {
		t.Fatalf("%+v", eps)
	}
	if eps[0].ID != "0:0" || eps[1].ID != "1:0" {
		t.Fatalf("ids broken: %+v", eps)
	}
}

func TestEpisodesE06_MissingDollarSkipped(t *testing.T) {
	eps := tvbox.ParseEpisodes("", "脏数据无分隔#第1集$http://a/1.m3u8", "m1", nil)
	if len(eps) != 1 || eps[0].PlayURL != "http://a/1.m3u8" {
		t.Fatalf("%+v", eps)
	}
}

func TestEpisodesE07_MismatchedGroups(t *testing.T) {
	// Locked: from[g] || from[0] || 线路{n}
	eps := tvbox.ParseEpisodes(
		"仅一线",
		"a$http://a/1.m3u8$$$b$http://b/1.m3u8",
		"m1", nil,
	)
	if len(eps) != 2 {
		t.Fatalf("%+v", eps)
	}
	if eps[0].PlayFrom != "仅一线" || eps[1].PlayFrom != "仅一线" {
		t.Fatalf("fallback to first playFrom: %+v", eps)
	}
	eps2 := tvbox.ParseEpisodes(
		"",
		"a$http://a/1.m3u8$$$b$http://b/1.m3u8",
		"m1", nil,
	)
	if eps2[0].PlayFrom != "线路1" || eps2[1].PlayFrom != "线路2" {
		t.Fatalf("fallback 线路n: %+v", eps2)
	}
}

func TestEpisodesE08_EmptyTitle(t *testing.T) {
	eps := tvbox.ParseEpisodes("", "$http://a/1.m3u8#$http://a/2.m3u8", "m1", nil)
	if len(eps) != 2 {
		t.Fatalf("%+v", eps)
	}
	if eps[0].Title != "第1集" || eps[1].Title != "第2集" {
		t.Fatalf("%+v", eps)
	}
}

func TestEpisodesE09_XMLPlayGroups(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rss>
  <list page="1" pagecount="1" recordcount="1">
    <video>
      <id>9</id>
      <name>示例剧</name>
      <dl>
        <dd flag="线路A">第1集$http://a/1.m3u8#第2集$http://a/2.m3u8</dd>
        <dd flag="线路B">第1集$http://b/1.m3u8</dd>
      </dl>
    </video>
  </list>
</rss>`
	norm, err := cms.ParseLegacyXML([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(norm.List) != 1 {
		t.Fatalf("list=%d", len(norm.List))
	}
	vod := norm.List[0]
	eps := tvbox.ParseEpisodes(vod.VodPlayFrom, vod.VodPlayURL, vod.VodID, nil)
	if len(eps) != 3 {
		t.Fatalf("len=%d %+v", len(eps), eps)
	}
	if eps[0].ID != "0:0" || eps[0].PlayFrom != "线路A" {
		t.Fatalf("%+v", eps[0])
	}
	if eps[2].ID != "1:0" || eps[2].PlayFrom != "线路B" {
		t.Fatalf("%+v", eps[2])
	}
}
