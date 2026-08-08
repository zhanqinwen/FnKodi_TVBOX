package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpapi"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpclient"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/live"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/logging"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/proxy"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
)

// version is injected via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	log := logging.New()

	cfg, err := config.Load(version)
	if err != nil {
		log.Error("config_invalid", "err", err)
		os.Exit(1)
	}

	short := httpclient.NewShort(cfg.HTTPTimeout, cfg.UserAgent)
	proxyClient := httpclient.NewProxy(cfg.ProxyHeaderTimeout, cfg.UserAgent)
	store := catalog.NewStore(cfg.SubscriptionURL, cfg.CacheTTL, short, log)
	cmsClient := &cms.Client{HTTP: short}
	t4Client := &t4.Client{HTTP: short}

	resolver := &player.Resolver{
		HTTP: short,
		T4:   t4Client,
		Parses: func() []player.ParseEntry {
			return player.DecodeParses(store.Parses())
		},
		SiteByID: store.SiteByID,
		Log:      log,
	}
	liveSvc := live.NewService(short, store.Lives, cfg.CacheTTL, log)
	proxyStore := proxy.NewStore(proxyClient, 10*time.Minute, cfg.Listen)

	mux := httpapi.NewMux(httpapi.Deps{
		Cfg:      cfg,
		Store:    store,
		CMS:      cmsClient,
		T4:       t4Client,
		Resolver: resolver,
		Live:     liveSvc,
		Proxy:    proxyStore,
		Log:      log,
	})
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if cfg.SubscriptionURL != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout+2*time.Second)
			defer cancel()
			_ = store.EnsureLoaded(ctx, true)
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("gateway_listen", "addr", cfg.Listen, "version", cfg.Version, "apiVersion", "v1")
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("gateway_shutdown", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("gateway_listen_failed", "err", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("gateway_shutdown_failed", "err", err)
		os.Exit(1)
	}
}
