package player_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

func TestResolveDirectCMS(t *testing.T) {
	r := &player.Resolver{
		HTTP: http.DefaultClient,
		SiteByID: func(id string) (tvbox.SupportedSite, bool) {
			return tvbox.SupportedSite{
				Site:       tvbox.Site{Key: id, Name: "CMS", Type: 1, API: "https://example.com/", Headers: map[string]string{"Referer": "https://example.com/"}},
				SourceType: "cms",
			}, true
		},
	}
	res, err := r.Resolve(context.Background(), player.ResolveRequest{
		SourceID: "cms1",
		PlayURL:  "https://cdn.example/v/1.m3u8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Parse != 0 || res.URL != "https://cdn.example/v/1.m3u8" {
		t.Fatalf("%+v", res)
	}
	if res.Headers["Referer"] != "https://example.com/" {
		t.Fatalf("headers=%v", res.Headers)
	}
}

func TestResolveViaJSONParser(t *testing.T) {
	parserSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"url":    "https://cdn.example/final.m3u8",
			"header": map[string]string{"User-Agent": "ParserUA"},
		})
	}))
	t.Cleanup(parserSrv.Close)

	r := &player.Resolver{
		HTTP: parserSrv.Client(),
		Parses: func() []player.ParseEntry {
			return []player.ParseEntry{{
				Name: "json1",
				Type: 1,
				URL:  parserSrv.URL + "/parse?url=",
			}}
		},
		SiteByID: func(id string) (tvbox.SupportedSite, bool) {
			return tvbox.SupportedSite{Site: tvbox.Site{Key: id, Type: 1, API: "https://x/"}, SourceType: "cms"}, true
		},
	}
	res, err := r.Resolve(context.Background(), player.ResolveRequest{
		SourceID: "cms1",
		PlayURL:  "https://page.example/watch?id=1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "https://cdn.example/final.m3u8" || res.Parse != 0 {
		t.Fatalf("%+v", res)
	}
	if res.Headers["User-Agent"] != "ParserUA" {
		t.Fatalf("headers=%v", res.Headers)
	}
}

func TestResolveType0ParserUnsupported(t *testing.T) {
	r := &player.Resolver{
		HTTP: http.DefaultClient,
		Parses: func() []player.ParseEntry {
			return []player.ParseEntry{{Name: "sniff", Type: 0, URL: "https://sniff/"}}
		},
		SiteByID: func(id string) (tvbox.SupportedSite, bool) {
			return tvbox.SupportedSite{Site: tvbox.Site{Key: id, Type: 1, API: "https://x/"}, SourceType: "cms"}, true
		},
	}
	_, err := r.Resolve(context.Background(), player.ResolveRequest{
		SourceID: "cms1",
		PlayURL:  "https://page.example/watch",
	})
	if err == nil || !strings.Contains(err.Error(), "resolve_failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveT4Play(t *testing.T) {
	t4srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ac") != "play" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"url": "https://cdn.example/t4.m3u8", "parse": 0})
	}))
	t.Cleanup(t4srv.Close)

	r := &player.Resolver{
		HTTP: t4srv.Client(),
		T4:   &t4.Client{HTTP: t4srv.Client()},
		SiteByID: func(id string) (tvbox.SupportedSite, bool) {
			return tvbox.SupportedSite{
				Site:       tvbox.Site{Key: id, Type: 4, API: t4srv.URL + "/"},
				SourceType: "t4",
			}, true
		},
	}
	res, err := r.Resolve(context.Background(), player.ResolveRequest{
		SourceID: "t4",
		PlayURL:  "flag-token-not-direct",
		PlayFrom: "线路1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "https://cdn.example/t4.m3u8" {
		t.Fatalf("%+v", res)
	}
}

func TestResolveLive(t *testing.T) {
	r := &player.Resolver{HTTP: http.DefaultClient}
	res, err := r.Resolve(context.Background(), player.ResolveRequest{
		SourceID: player.LiveSourceID,
		PlayURL:  "https://live.example/cctv1.m3u8|User-Agent=LiveUA&Referer=https://live.example/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "https://live.example/cctv1.m3u8" {
		t.Fatalf("%+v", res)
	}
	if res.Headers["User-Agent"] != "LiveUA" || res.Headers["Referer"] != "https://live.example/" {
		t.Fatalf("headers=%v", res.Headers)
	}
}
