package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds gateway settings loaded from environment variables.
type Config struct {
	Listen             string
	SubscriptionURL    string
	DataDir            string
	CacheTTL           time.Duration
	MediaCacheTTL      time.Duration
	HTTPTimeout        time.Duration
	ProxyHeaderTimeout time.Duration
	UserAgent          string
	Version            string
}

// Load reads configuration from environment variables and validates it.
func Load(version string) (*Config, error) {
	cfg := &Config{
		Listen:             envOr("FNTVBOX_LISTEN", "127.0.0.1:18765"),
		SubscriptionURL:    strings.TrimSpace(os.Getenv("FNTVBOX_SUBSCRIPTION_URL")),
		DataDir:            envOr("FNTVBOX_DATA_DIR", "/var/lib/fn-tvbox"),
		UserAgent:          envOr("FNTVBOX_USER_AGENT", "FnKodiTVBox/1.0"),
		Version:            version,
		CacheTTL:           time.Duration(envIntOr("FNTVBOX_CACHE_TTL_SEC", 300)) * time.Second,
		MediaCacheTTL:      time.Duration(envIntOr("FNTVBOX_MEDIA_CACHE_TTL_SEC", 900)) * time.Second,
		HTTPTimeout:        time.Duration(envIntOr("FNTVBOX_HTTP_TIMEOUT_MS", 8000)) * time.Millisecond,
		ProxyHeaderTimeout: time.Duration(envIntOr("FNTVBOX_PROXY_HEADER_TIMEOUT_MS", 15000)) * time.Millisecond,
	}

	if cfg.HTTPTimeout <= 0 {
		return nil, fmt.Errorf("FNTVBOX_HTTP_TIMEOUT_MS must be > 0")
	}
	if cfg.ProxyHeaderTimeout <= 0 {
		return nil, fmt.Errorf("FNTVBOX_PROXY_HEADER_TIMEOUT_MS must be > 0")
	}
	if err := validateListen(cfg.Listen); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SubscriptionConfigured reports whether a subscription URL is set.
func (c *Config) SubscriptionConfigured() bool {
	return c.SubscriptionURL != ""
}

func validateListen(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid FNTVBOX_LISTEN %q: %w", addr, err)
	}
	if port == "" {
		return fmt.Errorf("invalid FNTVBOX_LISTEN %q: missing port", addr)
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		if ip := net.ParseIP(host); ip == nil {
			if _, err := net.LookupHost(host); err != nil {
				return fmt.Errorf("invalid FNTVBOX_LISTEN host %q: %w", host, err)
			}
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("invalid FNTVBOX_LISTEN port %q: %w", port, err)
	}
	return nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
