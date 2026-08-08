package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := config.Load("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:18765" {
		t.Fatalf("listen=%s", cfg.Listen)
	}
	if cfg.HTTPTimeout.Milliseconds() != 8000 {
		t.Fatalf("httpTimeout=%v", cfg.HTTPTimeout)
	}
	if cfg.ProxyHeaderTimeout.Milliseconds() != 15000 {
		t.Fatalf("proxyHeaderTimeout=%v", cfg.ProxyHeaderTimeout)
	}
	if cfg.MediaCacheTTL != 900*time.Second {
		t.Fatalf("mediaCacheTTL=%v", cfg.MediaCacheTTL)
	}
	if cfg.SubscriptionConfigured() {
		t.Fatal("expected subscriptionConfigured=false")
	}
}

func TestLoadRejectsZeroTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv("FNTVBOX_HTTP_TIMEOUT_MS", "0")
	if _, err := config.Load("0.1.0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsInvalidListen(t *testing.T) {
	clearEnv(t)
	t.Setenv("FNTVBOX_LISTEN", "not-an-addr")
	if _, err := config.Load("0.1.0"); err == nil {
		t.Fatal("expected error")
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"FNTVBOX_LISTEN",
		"FNTVBOX_SUBSCRIPTION_URL",
		"FNTVBOX_DATA_DIR",
		"FNTVBOX_CACHE_TTL_SEC",
		"FNTVBOX_MEDIA_CACHE_TTL_SEC",
		"FNTVBOX_HTTP_TIMEOUT_MS",
		"FNTVBOX_PROXY_HEADER_TIMEOUT_MS",
		"FNTVBOX_USER_AGENT",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
