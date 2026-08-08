package player

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

const LiveSourceID = "__live__"

// Resolver resolves play URLs for CMS/T4/live.
type Resolver struct {
	HTTP    *http.Client
	T4      *t4.Client
	Parses  func() []ParseEntry
	SiteByID func(id string) (tvbox.SupportedSite, bool)
	Log     *slog.Logger
}

// Resolve runs the V1 play resolve pipeline.
func (r *Resolver) Resolve(ctx context.Context, req ResolveRequest) (*ResolvedPlay, error) {
	playURL := strings.TrimSpace(req.PlayURL)
	if playURL == "" {
		return nil, fmt.Errorf("playUrl is required")
	}
	headers := cloneHeaders(req.Headers)

	if req.SourceID == LiveSourceID {
		urlOnly, extra := SplitLiveURLHeaders(playURL)
		playURL = urlOnly
		headers = mergeHeaders(headers, extra)
		return r.finalize(ctx, &ResolvedPlay{
			URL:     playURL,
			Headers: headers,
			Parse:   boolToParse(!IsDirectMedia(playURL)),
			Format:  GuessFormat(playURL),
		})
	}

	site, ok := r.SiteByID(req.SourceID)
	if ok {
		headers = mergeHeaders(headers, site.HeaderMap())
	}

	var resolved *ResolvedPlay
	if ok && site.Type == 4 && !IsDirectMedia(playURL) {
		raw, err := r.T4.Play(ctx, site.Site, playURL, req.PlayFrom)
		if err != nil {
			return nil, fmt.Errorf("t4 play: %w", err)
		}
		resolved, err = ParsePlayResponse(raw)
		if err != nil {
			return nil, err
		}
		resolved.Headers = mergeHeaders(headers, resolved.Headers)
	} else {
		direct := IsDirectMedia(playURL)
		resolved = &ResolvedPlay{
			URL:     playURL,
			Headers: headers,
			Parse:   boolToParse(!direct),
			Format:  GuessFormat(playURL),
		}
	}

	return r.finalize(ctx, resolved)
}

func (r *Resolver) finalize(ctx context.Context, resolved *ResolvedPlay) (*ResolvedPlay, error) {
	if resolved == nil || strings.TrimSpace(resolved.URL) == "" {
		return nil, fmt.Errorf("resolve_failed: empty url")
	}
	if resolved.Parse == 0 || IsDirectMedia(resolved.URL) {
		resolved.Parse = 0
		if resolved.Format == "" {
			resolved.Format = GuessFormat(resolved.URL)
		}
		if resolved.Subtitles == nil {
			resolved.Subtitles = []any{}
		}
		if resolved.Danmaku == nil {
			resolved.Danmaku = []any{}
		}
		return resolved, nil
	}

	parsers := []ParseEntry{}
	if r.Parses != nil {
		parsers = r.Parses()
	}
	parsed, err := TryJSONParsers(ctx, r.HTTP, parsers, resolved.URL)
	if err != nil {
		return nil, fmt.Errorf("resolve_failed: %w", err)
	}
	parsed.Headers = mergeHeaders(resolved.Headers, parsed.Headers)
	if parsed.Subtitles == nil {
		parsed.Subtitles = []any{}
	}
	if parsed.Danmaku == nil {
		parsed.Danmaku = []any{}
	}
	return parsed, nil
}

func boolToParse(need bool) int {
	if need {
		return 1
	}
	return 0
}

func cloneHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeHeaders(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// SplitLiveURLHeaders parses `url|User-Agent=...&Referer=...` style suffixes.
func SplitLiveURLHeaders(raw string) (string, map[string]string) {
	raw = strings.TrimSpace(raw)
	i := strings.IndexByte(raw, '|')
	if i < 0 {
		return raw, nil
	}
	base := strings.TrimSpace(raw[:i])
	q := strings.TrimSpace(raw[i+1:])
	headers := map[string]string{}
	for _, part := range strings.Split(q, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		switch strings.ToLower(k) {
		case "ua", "user-agent", "user_agent":
			headers["User-Agent"] = v
		case "referer", "referrer":
			headers["Referer"] = v
		case "origin":
			headers["Origin"] = v
		case "cookie":
			headers["Cookie"] = v
		default:
			headers[k] = v
		}
	}
	return base, headers
}
