package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cache"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/live"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/proxy"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/subs"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

// Deps wires HTTP handlers.
type Deps struct {
	Cfg      *config.Config
	Store    *catalog.Store
	Subs     *subs.Service
	CMS      *cms.Client
	T4       *t4.Client
	Resolver *player.Resolver
	Live     *live.Service
	Proxy    *proxy.Store
	Log      *slog.Logger
}

// NewMux builds the HTTP mux for the gateway.
func NewMux(d Deps) *http.ServeMux {
	mux := http.NewServeMux()
	mediaTTL := time.Duration(0)
	httpTimeout := time.Duration(0)
	if d.Cfg != nil {
		mediaTTL = d.Cfg.MediaCacheTTL
		httpTimeout = d.Cfg.HTTPTimeout
	}
	media := &MediaAPI{
		Store:       d.Store,
		CMS:         d.CMS,
		T4:          d.T4,
		MediaTTL:    cache.NewTTL(mediaTTL),
		HTTPTimeout: httpTimeout,
		Log:         d.Log,
	}

	mux.HandleFunc("GET /health", handleHealth(d.Cfg, d.Store))
	mux.HandleFunc("GET /api/subscription", handleSubscriptionGet(d.Store))
	mux.HandleFunc("PUT /api/subscription", handleSubscriptionPut(d.Store, d.Subs))
	mux.HandleFunc("POST /api/subscription/reload", handleSubscriptionReload(d.Store))

	if d.Subs != nil && d.Subs.Reg != nil {
		mux.HandleFunc("GET /api/subscriptions", handleSubscriptionsList(d.Subs.Reg))
		mux.HandleFunc("POST /api/subscriptions", handleSubscriptionsAdd(d.Subs, d.Store))
		mux.HandleFunc("POST /api/subscriptions/probe", handleSubscriptionsProbe(d.Subs))
		mux.HandleFunc("PATCH /api/subscriptions/{id}", handleSubscriptionPatch(d.Subs, d.Store))
		mux.HandleFunc("DELETE /api/subscriptions/{id}", handleSubscriptionDelete(d.Subs, d.Store))
		mux.HandleFunc("POST /api/subscriptions/{id}/sync", handleSubscriptionSync(d.Subs, d.Store))
		mux.HandleFunc("POST /api/subscriptions/{id}/test", handleSubscriptionTest(d.Subs))
	}

	mux.HandleFunc("GET /api/sources", handleSourcesList(d.Store))
	mux.HandleFunc("GET /api/sources/{sourceId}/categories", media.handleCategories)
	mux.HandleFunc("GET /api/sources/{sourceId}/media", media.handleMedia)
	mux.HandleFunc("GET /api/sources/{sourceId}/detail", media.handleDetail)
	mux.HandleFunc("GET /api/sources/{sourceId}/search", media.handleSourceSearch)
	mux.HandleFunc("GET /api/search", media.handleAggregateSearch)

	if d.Resolver != nil {
		mux.HandleFunc("POST /api/player/resolve", handlePlayerResolve(d.Resolver))
	}
	if d.Live != nil {
		mux.HandleFunc("GET /api/live/groups", handleLiveGroups(d.Live))
		mux.HandleFunc("GET /api/live/channels", handleLiveChannels(d.Live))
	}
	if d.Proxy != nil {
		mux.HandleFunc("POST /api/proxy/session", handleProxySession(d.Proxy))
		mux.HandleFunc("GET /api/proxy/play/{token}", handleProxyPlay(d.Proxy))
	}

	mux.HandleFunc("/api/", handleNotImplemented)
	mux.HandleFunc("/api", handleNotImplemented)
	return mux
}

// NewMuxFromConfig is a convenience for tests that only need health/not-implemented.
func NewMuxFromConfig(cfg *config.Config) *http.ServeMux {
	store := catalog.NewStoreFromURL(cfg.SubscriptionURL, cfg.CacheTTL, nil, nil)
	svc := &subs.Service{Reg: store.Registry(), Client: nil}
	return NewMux(Deps{Cfg: cfg, Store: store, Subs: svc})
}

func handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "endpoint not implemented yet")
}
