package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpapi"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

func TestHealthAPIVersion(t *testing.T) {
	cfg := &config.Config{Listen: "127.0.0.1:18765", Version: "0.1.0", CacheTTL: time.Minute}
	mux := httpapi.NewMux(httpapi.Deps{
		Cfg:   cfg,
		Store: catalog.NewStore("", cfg.CacheTTL, nil, nil),
		CMS:   &cms.Client{},
		T4:    &t4.Client{},
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"apiVersion":"v1"`) {
		t.Fatal(rr.Body.String())
	}
}

func TestSubscriptionLastErrorOnBadURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)

	cfg := &config.Config{Listen: "127.0.0.1:18765", Version: "0.1.0", CacheTTL: time.Minute, HTTPTimeout: 2 * time.Second}
	client := upstream.Client()
	client.Timeout = 2 * time.Second
	store := catalog.NewStore(upstream.URL+"/missing.json", cfg.CacheTTL, client, nil)
	mux := httpapi.NewMux(httpapi.Deps{Cfg: cfg, Store: store, CMS: &cms.Client{HTTP: client}, T4: &t4.Client{HTTP: client}})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/subscription", nil))
	if rr.Code != 200 {
		t.Fatalf("status=%d", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	le, _ := body["lastError"].(map[string]any)
	if le == nil {
		t.Fatalf("expected lastError: %s", rr.Body.String())
	}
}

func TestSourcesFiltersType3(t *testing.T) {
	sub := `{
	  "sites": [
	    {"key":"cms1","name":"CMS1","type":1,"api":"https://example.com/api","searchable":1},
	    {"key":"jar1","name":"JAR","type":3,"api":"csp_X","jar":"http://x/a.jar"}
	  ],
	  "lives": [],
	  "parses": []
	}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sub))
	}))
	t.Cleanup(upstream.Close)

	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute, HTTPTimeout: 2 * time.Second}
	client := upstream.Client()
	store := catalog.NewStore(upstream.URL+"/tvbox.json", cfg.CacheTTL, client, nil)
	mux := httpapi.NewMux(httpapi.Deps{Cfg: cfg, Store: store, CMS: &cms.Client{HTTP: client}, T4: &t4.Client{HTTP: client}})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var body struct {
		Sources []struct {
			ID string `json:"id"`
		} `json:"sources"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Sources) != 1 || body.Sources[0].ID != "cms1" {
		t.Fatalf("%+v", body.Sources)
	}

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/subscription", nil))
	var sum map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &sum)
	if int(sum["skippedUnsupported"].(float64)) != 1 {
		t.Fatalf("skipped=%v", sum["skippedUnsupported"])
	}
}

func TestCMSMediaPagingAndDetail(t *testing.T) {
	muxUpstream := http.NewServeMux()
	muxUpstream.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ac := r.URL.Query().Get("ac")
		switch {
		case ac == "list" || (ac == "" && r.URL.Query().Get("ids") == "" && r.URL.Query().Get("t") == "" && r.URL.Query().Get("wd") == ""):
			_, _ = w.Write([]byte(`{"class":[{"type_id":"1","type_name":"电影"}],"list":[]}`))
		case r.URL.Query().Get("t") != "":
			_, _ = w.Write([]byte(`{"list":[{"vod_id":"100","vod_name":"片A","vod_pic":"http://p/a.jpg","vod_remarks":"高清"}],"page":1,"pagecount":3,"total":3}`))
		case r.URL.Query().Get("ids") != "":
			_, _ = w.Write([]byte(`{"list":[{"vod_id":"100","vod_name":"片A","vod_play_from":"线路1","vod_play_url":"第1集$http://a/1.m3u8#第2集$http://a/2.m3u8"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	cmsServer := httptest.NewServer(muxUpstream)
	t.Cleanup(cmsServer.Close)

	sub := `{"sites":[{"key":"testcms","name":"TestCMS","type":1,"api":"` + cmsServer.URL + `/api.php/provide/vod/"}],"lives":[],"parses":[]}`
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sub))
	}))
	t.Cleanup(subServer.Close)

	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute, HTTPTimeout: 3 * time.Second}
	client := subServer.Client()
	store := catalog.NewStore(subServer.URL+"/s.json", cfg.CacheTTL, client, nil)
	api := httpapi.NewMux(httpapi.Deps{Cfg: cfg, Store: store, CMS: &cms.Client{HTTP: client}, T4: &t4.Client{HTTP: client}})

	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sources/testcms/categories", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"电影"`) {
		t.Fatalf("categories: %s", rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	api.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/sources/testcms/media?categoryId=1&page=1", nil))
	if rr2.Code != 200 || !strings.Contains(rr2.Body.String(), `"page":1`) {
		t.Fatalf("media: %s", rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	api.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/sources/testcms/detail?mediaId=100", nil))
	if rr3.Code != 200 || !strings.Contains(rr3.Body.String(), `"id":"0:0"`) {
		t.Fatalf("detail: %s", rr3.Body.String())
	}
}

func TestPutSubscriptionInvalidURL(t *testing.T) {
	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute}
	store := catalog.NewStore("", cfg.CacheTTL, http.DefaultClient, nil)
	mux := httpapi.NewMux(httpapi.Deps{Cfg: cfg, Store: store, CMS: &cms.Client{}, T4: &t4.Client{}})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/subscription", strings.NewReader(`{"url":"not-a-url"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestNotImplementedStill501(t *testing.T) {
	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute}
	mux := httpapi.NewMux(httpapi.Deps{Cfg: cfg, Store: catalog.NewStore("", cfg.CacheTTL, nil, nil), CMS: &cms.Client{}, T4: &t4.Client{}})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/live/groups", nil))
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d", rr.Code)
	}
}
