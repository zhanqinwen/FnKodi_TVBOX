package catalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/subs"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// Snapshot is a successfully loaded merged catalog.
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

// Store manages multi-subscription load/cache and site lookup.
type Store struct {
	mu        sync.RWMutex
	reg       *subs.Registry
	ttl       time.Duration
	client    *http.Client
	log       *slog.Logger
	snap      *Snapshot
	expiresAt time.Time
	lastError *tvbox.LastError
}

// NewStore creates a catalog store backed by a subscription registry.
func NewStore(reg *subs.Registry, ttl time.Duration, client *http.Client, log *slog.Logger) *Store {
	if reg == nil {
		reg = subs.NewMemory()
	}
	return &Store{
		reg:    reg,
		ttl:    ttl,
		client: client,
		log:    log,
	}
}

// NewStoreFromURL creates an in-memory registry bootstrapped with one URL (tests/compat).
func NewStoreFromURL(subscriptionURL string, ttl time.Duration, client *http.Client, log *slog.Logger) *Store {
	reg := subs.NewMemory()
	subscriptionURL = strings.TrimSpace(subscriptionURL)
	if subscriptionURL != "" {
		_, _ = reg.UpsertTopLevel(subscriptionURL, "", subs.KindSingle)
	}
	return NewStore(reg, ttl, client, log)
}

// Registry returns the backing subscription registry.
func (s *Store) Registry() *subs.Registry { return s.reg }

// Configured reports whether any subscription is registered.
func (s *Store) Configured() bool {
	return s.reg != nil && s.reg.Configured()
}

// URL returns the primary top-level subscription URL.
func (s *Store) URL() string {
	if s.reg == nil {
		return ""
	}
	return s.reg.PrimaryURL()
}

// SetURL upserts a top-level URL without probing (legacy); prefer SubsService.UpsertFromURL.
func (s *Store) SetURL(raw string) error {
	if err := subs.ValidateHTTPURL(raw); err != nil {
		return err
	}
	_, err := s.reg.UpsertTopLevel(raw, "", subs.KindSingle)
	s.mu.Lock()
	s.snap = nil
	s.expiresAt = time.Time{}
	s.lastError = nil
	s.mu.Unlock()
	return err
}

// InvalidateCache clears the merged catalog cache.
func (s *Store) InvalidateCache() {
	s.mu.Lock()
	s.snap = nil
	s.expiresAt = time.Time{}
	s.mu.Unlock()
}

// Summary returns the aggregated subscription summary (HTTP 200 friendly).
func (s *Store) Summary() tvbox.Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total, children := 0, 0
	if s.reg != nil {
		total, children = s.reg.Counts()
	}
	sum := tvbox.Summary{
		URL:               s.URL(),
		Kind:              "single",
		LastError:         cloneErr(s.lastError),
		SubscriptionCount: total,
		ChildCount:        children,
	}
	if s.reg != nil && s.reg.HasWarehouse() {
		sum.Kind = "warehouse"
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
	if !s.Configured() && s.lastError == nil {
		sum.LastError = &tvbox.LastError{
			Code:    "bad_request",
			Message: "subscription url not configured",
			At:      time.Now().UTC(),
		}
	}
	return sum
}

// EnsureLoaded loads enabled non-warehouse subscriptions if missing/expired.
func (s *Store) EnsureLoaded(ctx context.Context, force bool) tvbox.Summary {
	s.mu.RLock()
	fresh := s.snap != nil && !force && time.Now().Before(s.expiresAt)
	configured := s.Configured()
	s.mu.RUnlock()
	if !configured {
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

// Sites returns supported sites from last success.
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

// SiteByID finds a supported site by prefixed source id.
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

// Parses returns merged parses[].
func (s *Store) Parses() []any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil || s.snap.Raw == nil {
		return nil
	}
	return append([]any(nil), s.snap.Raw.Parses...)
}

// Lives returns merged lives[].
func (s *Store) Lives() []any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil || s.snap.Raw == nil {
		return nil
	}
	return append([]any(nil), s.snap.Raw.Lives...)
}

func (s *Store) reload(ctx context.Context) error {
	active := s.reg.ActiveContent()
	if len(active) == 0 {
		// Still may have warehouse parents only — treat as empty success with hint.
		s.mu.Lock()
		s.snap = &Snapshot{
			URL:      s.reg.PrimaryURL(),
			Kind:     kindFromReg(s.reg),
			LoadedAt: time.Now().UTC(),
			Raw:      &tvbox.RawConfig{},
		}
		s.expiresAt = time.Now().Add(s.ttl)
		if s.reg.Configured() {
			s.lastError = nil
		}
		s.mu.Unlock()
		return nil
	}

	merged := &tvbox.RawConfig{}
	var allSites []tvbox.SupportedSite
	skipped := 0
	var firstErr error
	loadedAny := false

	for _, sub := range active {
		if sub.Kind == subs.KindLive {
			// Live-only subscriptions contribute to live service via URL list entries.
			merged.Lives = append(merged.Lives, map[string]any{"name": sub.Name, "url": sub.URL})
			loadedAny = true
			continue
		}
		data, err := tvbox.FetchConfigGET(ctx, s.client, sub.URL)
		if err != nil {
			if s.log != nil {
				s.log.Info("subscription_child_fetch_failed", "id", sub.ID, "url", sub.URL, "err", err)
			}
			if firstErr == nil {
				firstErr = classifyFetchErr(err)
			}
			continue
		}
		raw, err := tvbox.ParseConfigText(data)
		if err != nil {
			if firstErr == nil {
				firstErr = &typedErr{code: "upstream_error", msg: err.Error()}
			}
			continue
		}
		// Do not expand warehouse here — children are first-class registry entries.
		if tvbox.IsWarehouse(raw) {
			if s.log != nil {
				s.log.Info("skip_warehouse_document_in_aggregate", "id", sub.ID, "url", sub.URL)
			}
			continue
		}
		filtered := tvbox.FilterSites(raw.Sites, s.log)
		prefix := sourcePrefix(sub.ID)
		for _, site := range filtered.Supported {
			site.Key = prefix + site.Key
			allSites = append(allSites, site)
		}
		skipped += filtered.SkippedUnsupported
		merged.Sites = append(merged.Sites, raw.Sites...)
		merged.Lives = append(merged.Lives, raw.Lives...)
		merged.Parses = append(merged.Parses, raw.Parses...)
		loadedAny = true
	}

	if !loadedAny && firstErr != nil {
		return firstErr
	}

	kind := kindFromReg(s.reg)
	if len(active) == 1 && active[0].ParentID == "" {
		kind = active[0].Kind
		if kind == "" {
			kind = "single"
		}
	} else if s.reg.HasWarehouse() {
		kind = "warehouse"
	} else if len(active) > 1 {
		kind = "single"
	}

	snap := &Snapshot{
		URL:                s.reg.PrimaryURL(),
		Kind:               kind,
		LoadedAt:           time.Now().UTC(),
		Sites:              allSites,
		SkippedUnsupported: skipped,
		LiveCount:          len(merged.Lives),
		ParseCount:         len(merged.Parses),
		Raw:                merged,
	}

	s.mu.Lock()
	s.snap = snap
	s.expiresAt = time.Now().Add(s.ttl)
	if firstErr != nil && len(allSites) == 0 {
		// keep lastError via setError path
	} else {
		s.lastError = nil
	}
	s.mu.Unlock()

	if firstErr != nil && len(allSites) == 0 {
		return firstErr
	}
	return nil
}

func sourcePrefix(subID string) string {
	id := strings.TrimSpace(subID)
	if id == "" {
		return "sub_x_"
	}
	// sub_{id8}_
	short := id
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("sub_%s_", short)
}

func kindFromReg(r *subs.Registry) string {
	if r == nil {
		return "single"
	}
	if r.HasWarehouse() {
		return "warehouse"
	}
	return "single"
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
