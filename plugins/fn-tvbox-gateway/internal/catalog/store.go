package catalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// Snapshot is a successfully loaded subscription.
type Snapshot struct {
	URL                string
	Kind               string
	LoadedAt           time.Time
	Sites              []tvbox.SupportedSite
	SkippedUnsupported int
	LiveCount          int
	ParseCount         int
	Raw                *tvbox.RawConfig
}

// Store manages subscription load/cache and site lookup.
type Store struct {
	mu        sync.RWMutex
	url       string
	ttl       time.Duration
	client    *http.Client
	log       *slog.Logger
	snap      *Snapshot
	expiresAt time.Time
	lastError *tvbox.LastError
}

// NewStore creates a subscription store.
func NewStore(subscriptionURL string, ttl time.Duration, client *http.Client, log *slog.Logger) *Store {
	return &Store{
		url:    strings.TrimSpace(subscriptionURL),
		ttl:    ttl,
		client: client,
		log:    log,
	}
}

// Configured reports whether a URL is set.
func (s *Store) Configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.url != ""
}

// URL returns the current subscription URL.
func (s *Store) URL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.url
}

// SetURL validates and sets URL, clearing cache.
func (s *Store) SetURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("url is required")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid url")
	}
	s.mu.Lock()
	s.url = raw
	s.snap = nil
	s.expiresAt = time.Time{}
	s.lastError = nil
	s.mu.Unlock()
	return nil
}

// Summary returns the subscription summary (always suitable for HTTP 200).
func (s *Store) Summary() tvbox.Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sum := tvbox.Summary{
		URL:       s.url,
		Kind:      "single",
		LastError: cloneErr(s.lastError),
	}
	if s.snap != nil {
		sum.Kind = s.snap.Kind
		t := s.snap.LoadedAt.UTC()
		sum.LoadedAt = &t
		sum.SiteCount = len(s.snap.Sites)
		sum.SkippedUnsupported = s.snap.SkippedUnsupported
		sum.LiveCount = s.snap.LiveCount
		sum.ParseCount = s.snap.ParseCount
	}
	if s.url == "" && s.lastError == nil {
		sum.LastError = &tvbox.LastError{
			Code:    "bad_request",
			Message: "subscription url not configured",
			At:      time.Now().UTC(),
		}
	}
	return sum
}

// EnsureLoaded loads subscription if missing/expired. force bypasses TTL.
func (s *Store) EnsureLoaded(ctx context.Context, force bool) tvbox.Summary {
	s.mu.RLock()
	url := s.url
	fresh := s.snap != nil && !force && time.Now().Before(s.expiresAt)
	s.mu.RUnlock()
	if url == "" {
		return s.Summary()
	}
	if fresh {
		return s.Summary()
	}
	if err := s.reload(ctx); err != nil {
		s.setError(err)
	}
	return s.Summary()
}

// Sites returns supported sites from last success (may be empty).
func (s *Store) Sites() []tvbox.SupportedSite {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil {
		return nil
	}
	out := make([]tvbox.SupportedSite, len(s.snap.Sites))
	copy(out, s.snap.Sites)
	return out
}

// SiteByID finds a supported site by key/id.
func (s *Store) SiteByID(id string) (tvbox.SupportedSite, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil {
		return tvbox.SupportedSite{}, false
	}
	for _, site := range s.snap.Sites {
		if site.Key == id {
			return site, true
		}
	}
	return tvbox.SupportedSite{}, false
}

// Parses returns subscription parses[] from last success.
func (s *Store) Parses() []any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil || s.snap.Raw == nil {
		return nil
	}
	return append([]any(nil), s.snap.Raw.Parses...)
}

// Lives returns subscription lives[] from last success.
func (s *Store) Lives() []any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil || s.snap.Raw == nil {
		return nil
	}
	return append([]any(nil), s.snap.Raw.Lives...)
}

func (s *Store) reload(ctx context.Context) error {
	s.mu.RLock()
	rawURL := s.url
	client := s.client
	log := s.log
	s.mu.RUnlock()
	if rawURL == "" {
		return errors.New("subscription url not configured")
	}

	data, err := tvbox.FetchConfigGET(ctx, client, rawURL)
	if err != nil {
		return classifyFetchErr(err)
	}
	raw, err := tvbox.ParseConfigText(data)
	if err != nil {
		return &typedErr{code: "upstream_error", msg: err.Error()}
	}

	kind := "single"
	if tvbox.IsWarehouse(raw) {
		kind = "warehouse"
		raw, err = tvbox.ExpandWarehouseHTTP(ctx, func(ctx context.Context, u string) ([]byte, error) {
			return tvbox.FetchConfigGET(ctx, client, u)
		}, raw, log)
		if err != nil {
			return classifyFetchErr(err)
		}
	}

	filtered := tvbox.FilterSites(raw.Sites, log)
	snap := &Snapshot{
		URL:                rawURL,
		Kind:               kind,
		LoadedAt:           time.Now().UTC(),
		Sites:              filtered.Supported,
		SkippedUnsupported: filtered.SkippedUnsupported,
		LiveCount:          len(raw.Lives),
		ParseCount:         len(raw.Parses),
		Raw:                raw,
	}

	s.mu.Lock()
	s.snap = snap
	s.expiresAt = time.Now().Add(s.ttl)
	s.lastError = nil
	s.mu.Unlock()
	return nil
}

func (s *Store) setError(err error) {
	code, msg := "upstream_error", err.Error()
	var te *typedErr
	if errors.As(err, &te) {
		code, msg = te.code, te.msg
	}
	s.mu.Lock()
	s.lastError = &tvbox.LastError{Code: code, Message: msg, At: time.Now().UTC()}
	s.mu.Unlock()
	if s.log != nil {
		s.log.Info("subscription_load_failed", "code", code, "err", msg)
	}
}

type typedErr struct {
	code string
	msg  string
}

func (e *typedErr) Error() string { return e.msg }

func classifyFetchErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") {
		return &typedErr{code: "upstream_timeout", msg: msg}
	}
	return &typedErr{code: "upstream_error", msg: msg}
}

func cloneErr(e *tvbox.LastError) *tvbox.LastError {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}
