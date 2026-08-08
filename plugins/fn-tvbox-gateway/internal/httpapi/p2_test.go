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
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/subs"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

func testDeps(cfg *config.Config, store *catalog.Store, client *http.Client) httpapi.Deps {
	if client == nil {
		client = http.DefaultClient
	}
	return httpapi.Deps{
		Cfg:   cfg,
		Store: store,
		Subs:  &subs.Service{Reg: store.Registry(), Client: client},
		CMS:   &cms.Client{HTTP: client},
		T4:    &t4.Client{HTTP: client},
	}
}

func TestHealthAPIVersion(t *testing.T) {
	cfg := &config.Config{Listen: "127.0.0.1:18765", Version: "0.1.0", CacheTTL: time.Minute}
	store := catalog.NewStoreFromURL("", cfg.CacheTTL, nil, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, nil))
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
	store := catalog.NewStoreFromURL(upstream.URL+"/missing.json", cfg.CacheTTL, client, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, client))

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
	store := catalog.NewStoreFromURL(upstream.URL+"/tvbox.json", cfg.CacheTTL, client, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, client))

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
	if len(body.Sources) != 1 || !strings.HasSuffix(body.Sources[0].ID, "cms1") {
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
			_, _ = w.Write([]byte(`{"class":[{"type_id":"1","type_name":"Movie"}],"list":[]}`))
		case r.URL.Query().Get("t") != "":
			_, _ = w.Write([]byte(`{"list":[{"vod_id":"100","vod_name":"FilmA","vod_pic":"http://p/a.jpg","vod_remarks":"HD"}],"page":1,"pagecount":3,"total":3}`))
		case r.URL.Query().Get("ids") != "":
			_, _ = w.Write([]byte(`{"list":[{"vod_id":"100","vod_name":"FilmA","vod_play_from":"L1","vod_play_url":"E1$http://a/1.m3u8#E2$http://a/2.m3u8"}]}`))
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
	store := catalog.NewStoreFromURL(subServer.URL+"/s.json", cfg.CacheTTL, client, nil)
	api := httpapi.NewMux(testDeps(cfg, store, client))

	rr0 := httptest.NewRecorder()
	api.ServeHTTP(rr0, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
	var srcBody struct {
		Sources []struct {
			ID string `json:"id"`
		} `json:"sources"`
	}
	_ = json.Unmarshal(rr0.Body.Bytes(), &srcBody)
	if len(srcBody.Sources) != 1 {
		t.Fatalf("sources: %s", rr0.Body.String())
	}
	sid := srcBody.Sources[0].ID

	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/sources/"+sid+"/categories", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"Movie"`) {
		t.Fatalf("categories: %s", rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	api.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/sources/"+sid+"/media?categoryId=1&page=1", nil))
	if rr2.Code != 200 || !strings.Contains(rr2.Body.String(), `"page":1`) {
		t.Fatalf("media: %s", rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	api.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/sources/"+sid+"/detail?mediaId=100", nil))
	if rr3.Code != 200 || !strings.Contains(rr3.Body.String(), `"id":"0:0"`) {
		t.Fatalf("detail: %s", rr3.Body.String())
	}
}

func TestPutSubscriptionInvalidURL(t *testing.T) {
	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute}
	store := catalog.NewStoreFromURL("", cfg.CacheTTL, http.DefaultClient, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, http.DefaultClient))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/subscription", strings.NewReader(`{"url":"not-a-url"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 400 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestNotImplementedStill501(t *testing.T) {
	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute}
	store := catalog.NewStoreFromURL("", cfg.CacheTTL, nil, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, nil))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/live/groups", nil))
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestWarehouseSubscriptionsAPI(t *testing.T) {
	childA := `{"sites":[{"key":"a1","name":"A1","type":1,"api":"https://example.com/a"}],"lives":[],"parses":[]}`
	childB := `{"sites":[{"key":"b1","name":"B1","type":1,"api":"https://example.com/b"}],"lives":[],"parses":[]}`
	base := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wh.json":
			_, _ = w.Write([]byte(`{"urls":[{"name":"ChildA","url":"` + base + `/a.json"},{"name":"ChildB","url":"` + base + `/b.json"}]}`))
		case "/a.json":
			_, _ = w.Write([]byte(childA))
		case "/b.json":
			_, _ = w.Write([]byte(childB))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	base = srv.URL

	cfg := &config.Config{Version: "0.1.0", CacheTTL: time.Minute, HTTPTimeout: 3 * time.Second}
	client := srv.Client()
	store := catalog.NewStoreFromURL("", cfg.CacheTTL, client, nil)
	mux := httpapi.NewMux(testDeps(cfg, store, client))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", strings.NewReader(`{"url":"`+base+`/wh.json","name":"Warehouse"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("add: %s", rr.Body.String())
	}
	var addBody struct {
		Subscriptions []struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			ParentID string `json:"parentId"`
			Enabled  bool   `json:"enabled"`
			Name     string `json:"name"`
		} `json:"subscriptions"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &addBody)
	if len(addBody.Subscriptions) != 3 {
		t.Fatalf("want parent+2 children, got %+v body=%s", addBody.Subscriptions, rr.Body.String())
	}
	var childID string
	for _, s := range addBody.Subscriptions {
		if s.ParentID != "" && s.Name == "ChildA" {
			childID = s.ID
		}
	}
	if childID == "" {
		t.Fatal("missing child A")
	}

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
	var src struct {
		Sources []struct {
			ID string `json:"id"`
		} `json:"sources"`
	}
	_ = json.Unmarshal(rr2.Body.Bytes(), &src)
	if len(src.Sources) != 2 {
		t.Fatalf("sources before disable: %+v body=%s", src.Sources, rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	preq := httptest.NewRequest(http.MethodPatch, "/api/subscriptions/"+childID, strings.NewReader(`{"enabled":false}`))
	mux.ServeHTTP(rr3, preq)
	if rr3.Code != 200 {
		t.Fatalf("patch: %s", rr3.Body.String())
	}

	rr4 := httptest.NewRecorder()
	mux.ServeHTTP(rr4, httptest.NewRequest(http.MethodGet, "/api/sources", nil))
	_ = json.Unmarshal(rr4.Body.Bytes(), &src)
	if len(src.Sources) != 1 {
		t.Fatalf("sources after disable: %+v", src.Sources)
	}
}
