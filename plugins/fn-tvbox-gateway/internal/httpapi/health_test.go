package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpapi"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/subs"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

func TestHealth(t *testing.T) {
	cfg := &config.Config{
		Listen:          "127.0.0.1:18765",
		SubscriptionURL: "https://example.com/tvbox.json",
		Version:         "0.1.0",
		CacheTTL:        time.Minute,
	}
	store := catalog.NewStoreFromURL(cfg.SubscriptionURL, cfg.CacheTTL, nil, nil)
	mux := httpapi.NewMux(httpapi.Deps{
		Cfg: cfg, Store: store,
		Subs: &subs.Service{Reg: store.Registry()},
		CMS:  &cms.Client{}, T4: &t4.Client{},
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("ok=%v", body["ok"])
	}
	if body["apiVersion"] != "v1" {
		t.Fatalf("apiVersion=%v", body["apiVersion"])
	}
	if body["version"] != "0.1.0" {
		t.Fatalf("version=%v", body["version"])
	}
	if body["subscriptionConfigured"] != true {
		t.Fatalf("subscriptionConfigured=%v", body["subscriptionConfigured"])
	}
}
