package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpapi"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpclient"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/live"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/proxy"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

func TestPlayerResolveAndLiveAPI(t *testing.T) {
	liveBody := `#EXTM3U
#EXTINF:-1 group-title="央视",CCTV1
https://example/cctv1.m3u8
`
	liveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(liveBody))
	}))
	t.Cleanup(liveSrv.Close)

	sub := `{"sites":[{"key":"cms1","name":"CMS","type":1,"api":"https://example.com/api"}],"lives":[{"name":"测试直播","url":"` + liveSrv.URL + `/live.m3u"}],"parses":[]}`
	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sub))
	}))
	t.Cleanup(subSrv.Close)

	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute, HTTPTimeout: 3 * time.Second, Listen: "127.0.0.1:18765"}
	client := subSrv.Client()
	store := catalog.NewStoreFromURL(subSrv.URL+"/s.json", cfg.CacheTTL, client, nil)
	resolver := &player.Resolver{
		HTTP:     client,
		T4:       &t4.Client{HTTP: client},
		Parses:   func() []player.ParseEntry { return player.DecodeParses(store.Parses()) },
		SiteByID: store.SiteByID,
	}
	liveSvc := live.NewService(client, store.Lives, cfg.CacheTTL, nil)
	proxyStore := proxy.NewStore(httpclient.NewProxy(2*time.Second, "test"), time.Minute, cfg.Listen)

	mux := httpapi.NewMux(httpapi.Deps{
		Cfg: cfg, Store: store, CMS: &cms.Client{HTTP: client}, T4: &t4.Client{HTTP: client},
		Resolver: resolver, Live: liveSvc, Proxy: proxyStore,
	})

	store.EnsureLoaded(context.Background(), true)
	sites := store.Sites()
	if len(sites) != 1 {
		t.Fatalf("sites=%d", len(sites))
	}
	sid := sites[0].Key

	rr := httptest.NewRecorder()
	body := `{"sourceId":"` + sid + `","mediaId":"1","episodeId":"0:0","playUrl":"https://cdn.example/a.m3u8","playFrom":"L1"}`
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/player/resolve", strings.NewReader(body)))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"parse":0`) {
		t.Fatalf("resolve: %s", rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/live/groups", nil))
	if rr2.Code != 200 || !strings.Contains(rr2.Body.String(), "央视") {
		t.Fatalf("groups: %s", rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/live/channels?group="+url.QueryEscape("央视"), nil))
	if rr3.Code != 200 || !strings.Contains(rr3.Body.String(), "CCTV1") {
		t.Fatalf("channels: %s", rr3.Body.String())
	}
}

func TestProxySlowBodyNotCutByShortTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// headers returned quickly; body delayed beyond short-request default (8s)
		time.Sleep(9 * time.Second)
		_, _ = io.WriteString(w, "SLOW_BODY_OK")
	}))
	t.Cleanup(upstream.Close)

	// short client would die at 8s; proxy client must not
	short := httpclient.NewShort(8*time.Second, "test")
	proxyClient := httpclient.NewProxy(2*time.Second, "test")
	_ = short

	store := proxy.NewStore(proxyClient, time.Minute, "127.0.0.1:18765")
	cfg := &config.Config{Version: "0.1.0", Listen: "127.0.0.1:18765"}
	mux := httpapi.NewMux(httpapi.Deps{
		Cfg:   cfg,
		Store: catalog.NewStoreFromURL("", time.Minute, nil, nil),
		CMS:   &cms.Client{},
		T4:    &t4.Client{},
		Proxy: store,
	})

	rr := httptest.NewRecorder()
	reqBody, _ := json.Marshal(map[string]any{"url": upstream.URL + "/video.mp4", "headers": map[string]string{"Referer": "https://x/"}})
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/proxy/session", strings.NewReader(string(reqBody))))
	if rr.Code != 200 {
		t.Fatalf("session: %s", rr.Body.String())
	}
	var sess struct {
		PlayURL string `json:"playUrl"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &sess)
	token := sess.PlayURL[strings.LastIndex(sess.PlayURL, "/")+1:]

	start := time.Now()
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/proxy/play/"+token, nil))
	elapsed := time.Since(start)
	if rr2.Code != 200 {
		t.Fatalf("play status=%d body=%s", rr2.Code, rr2.Body.String())
	}
	if !strings.Contains(rr2.Body.String(), "SLOW_BODY_OK") {
		t.Fatalf("body=%q", rr2.Body.String())
	}
	if elapsed < 9*time.Second {
		t.Fatalf("expected slow body wait, elapsed=%s", elapsed)
	}
}
