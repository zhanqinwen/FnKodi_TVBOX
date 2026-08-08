package player

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxParseBody = 2 * 1024 * 1024

// DecodeParses converts subscription parses[] any values into typed entries.
func DecodeParses(raw []any) []ParseEntry {
	out := make([]ParseEntry, 0, len(raw))
	for _, item := range raw {
		b, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var p ParseEntry
		if err := json.Unmarshal(b, &p); err != nil {
			continue
		}
		p.Name = strings.TrimSpace(p.Name)
		p.URL = strings.TrimSpace(p.URL)
		if p.URL == "" {
			continue
		}
		// also accept headers key
		var alt struct {
			Headers map[string]string `json:"headers"`
		}
		_ = json.Unmarshal(b, &alt)
		if len(p.Headers) == 0 && len(alt.Headers) > 0 {
			p.Headers = alt.Headers
		}
		out = append(out, p)
	}
	return out
}

// TryJSONParsers sequentially tries type=1 parsers until one returns a playable URL.
// type=0 sniff parsers are skipped (no browser sniff in V1).
func TryJSONParsers(ctx context.Context, client *http.Client, parsers []ParseEntry, targetURL string) (*ResolvedPlay, error) {
	var sawType0 bool
	var lastErr error
	for _, p := range parsers {
		if p.Type == 0 {
			sawType0 = true
			continue
		}
		if p.Type != 1 {
			continue
		}
		res, err := executeJSONParser(ctx, client, p, targetURL)
		if err != nil {
			lastErr = err
			continue
		}
		if res != nil && strings.TrimSpace(res.URL) != "" {
			return res, nil
		}
	}
	if sawType0 && lastErr == nil {
		return nil, fmt.Errorf("sniff parsers (type=0) unsupported without browser; no type=1 parser succeeded")
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no json parser succeeded")
}

func executeJSONParser(ctx context.Context, client *http.Client, parser ParseEntry, targetURL string) (*ResolvedPlay, error) {
	reqURL := parser.URL
	if strings.Contains(reqURL, "{url}") {
		reqURL = strings.ReplaceAll(reqURL, "{url}", targetURL)
	} else {
		reqURL = reqURL + targetURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range parser.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("parser %s status %d", parser.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxParseBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxParseBody {
		return nil, fmt.Errorf("parser response too large")
	}
	trim := strings.TrimSpace(string(data))
	if trim == "" {
		return nil, fmt.Errorf("empty parser response")
	}
	if strings.HasPrefix(trim, "<") {
		return nil, fmt.Errorf("parser returned HTML")
	}
	// plain URL string
	if (strings.HasPrefix(trim, "http://") || strings.HasPrefix(trim, "https://")) && !strings.HasPrefix(trim, "{") && !strings.HasPrefix(trim, "[") {
		return &ResolvedPlay{
			URL:        trim,
			Parse:      0,
			Format:     GuessFormat(trim),
			ParserName: parser.Name,
			ParserURL:  parser.URL,
		}, nil
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parser json: %w", err)
	}
	play, err := ParsePlayResponse(payload)
	if err != nil {
		return nil, err
	}
	play.Parse = 0
	play.ParserName = parser.Name
	play.ParserURL = parser.URL
	if play.Format == "" {
		play.Format = GuessFormat(play.URL)
	}
	return play, nil
}

// ParsePlayResponse normalizes TVBox play JSON.
func ParsePlayResponse(payload any) (*ResolvedPlay, error) {
	m, ok := asMap(payload)
	if !ok {
		return nil, fmt.Errorf("play response not an object")
	}
	// unwrap data
	if data, ok := asMap(m["data"]); ok {
		if _, has := data["url"]; has {
			if _, hasH := data["header"]; !hasH {
				if h, ok := m["header"]; ok {
					data["header"] = h
				}
			}
			m = data
		}
	}
	playURL := firstString(m, "url", "play_url")
	if playURL == "" {
		return nil, fmt.Errorf("play response missing url")
	}
	headers := extractHeaders(m)
	parse := 0
	if v, ok := m["parse"]; ok {
		parse = toInt(v)
	}
	format, _ := m["format"].(string)
	pos := 0.0
	if v, ok := m["position"]; ok {
		// ms -> seconds if large
		ms := toFloat(v)
		if ms > 1000 {
			pos = ms / 1000
		} else {
			pos = ms
		}
	}
	return &ResolvedPlay{
		URL:             playURL,
		Headers:         headers,
		Format:          format,
		Parse:           parse,
		PositionSeconds: pos,
		Subtitles:       []any{},
		Danmaku:         []any{},
	}, nil
}

func asMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, false
		}
		return m, true
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			}
		}
	}
	return ""
}

func extractHeaders(m map[string]any) map[string]string {
	for _, key := range []string{"header", "headers"} {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case map[string]any:
			out := map[string]string{}
			for k, vv := range t {
				out[k] = fmt.Sprint(vv)
			}
			return out
		case string:
			var out map[string]string
			if json.Unmarshal([]byte(t), &out) == nil {
				return out
			}
		}
	}
	return nil
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		var n int
		fmt.Sscanf(t, "%d", &n)
		return n
	default:
		return 0
	}
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}
