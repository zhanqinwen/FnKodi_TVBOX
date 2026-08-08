package live

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// Service loads and caches live channels from subscription lives[].
type Service struct {
	HTTP   *http.Client
	Lives  func() []any
	Log    *slog.Logger
	mu     sync.Mutex
	cache  []Channel
	expiry time.Time
	ttl    time.Duration
}

// NewService creates a live service.
func NewService(httpClient *http.Client, lives func() []any, ttl time.Duration, log *slog.Logger) *Service {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Service{HTTP: httpClient, Lives: lives, ttl: ttl, Log: log}
}

// Channels returns all channels (refreshing cache as needed).
func (s *Service) Channels(ctx context.Context) []Channel {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache != nil && time.Now().Before(s.expiry) {
		return append([]Channel(nil), s.cache...)
	}
	s.cache = s.load(ctx)
	s.expiry = time.Now().Add(s.ttl)
	return append([]Channel(nil), s.cache...)
}

// Invalidate clears cache (call after subscription reload).
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.expiry = time.Time{}
	s.mu.Unlock()
}

func (s *Service) load(ctx context.Context) []Channel {
	raw := []any{}
	if s.Lives != nil {
		raw = s.Lives()
	}
	out := ParseInlineLives(raw)
	for _, item := range raw {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		// skip password-protected live sources in V1
		if pass := str(m["pass"]); pass != "" {
			if s.Log != nil {
				s.Log.Info("skip encrypted live source", "name", str(m["name"]))
			}
			continue
		}
		u := str(m["url"])
		if u == "" {
			continue
		}
		lower := strings.ToLower(u)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			continue
		}
		name := str(m["name"])
		defaults := liveDefaults(m)
		data, err := tvbox.FetchConfigGET(ctx, s.HTTP, u)
		if err != nil {
			if s.Log != nil {
				s.Log.Info("live fetch failed", "url", u, "err", err)
			}
			continue
		}
		chs := ParsePlaylist(string(data), name, defaults)
		out = append(out, chs...)
	}
	// drop empty-name channels
	filtered := out[:0]
	for _, ch := range out {
		if strings.TrimSpace(ch.Name) == "" || strings.TrimSpace(ch.URL) == "" {
			continue
		}
		filtered = append(filtered, ch)
	}
	return filtered
}

// Groups aggregates channel groups.
func Groups(channels []Channel) []Group {
	counts := map[string]int{}
	order := []string{}
	for _, ch := range channels {
		if _, ok := counts[ch.Group]; !ok {
			order = append(order, ch.Group)
		}
		counts[ch.Group]++
	}
	out := make([]Group, 0, len(order))
	for _, g := range order {
		out = append(out, Group{ID: g, Name: g, ChannelCount: counts[g]})
	}
	return out
}

// FilterChannels filters by group and keyword.
func FilterChannels(channels []Channel, group, keyword string) []Channel {
	group = strings.TrimSpace(group)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	out := make([]Channel, 0, len(channels))
	for _, ch := range channels {
		if group != "" && ch.Group != group {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(ch.Name), keyword) {
			continue
		}
		out = append(out, ch)
	}
	return out
}
