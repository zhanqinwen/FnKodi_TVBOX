package live

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
)

var (
	extinfRe = regexp.MustCompile(`(?i)^#EXTINF:(-?[\d.]+)\s*(.*?),\s*(.*)$`)
	attrRe   = regexp.MustCompile(`([A-Za-z0-9_-]+)="([^"]*)"`)
)

// ParsePlaylist parses M3U or genre-text live playlists.
func ParsePlaylist(body string, defaultGroup string, defaults map[string]string) []Channel {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToUpper(body), "#EXTM3U") || strings.Contains(body, "#EXTINF") {
		return parseM3U(body, defaultGroup, defaults)
	}
	return parseGenreText(body, defaultGroup, defaults)
}

func parseM3U(body, defaultGroup string, defaults map[string]string) []Channel {
	if defaultGroup == "" {
		defaultGroup = "未分组"
	}
	sc := bufio.NewScanner(strings.NewReader(body))
	// allow long lines
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	var pending *pendingCh
	var out []Channel
	group := defaultGroup
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "#EXTINF") {
			pending = parseExtinf(line, group)
			continue
		}
		if strings.HasPrefix(upper, "#EXTGRP:") {
			group = strings.TrimSpace(line[len("#EXTGRP:"):])
			if pending != nil {
				pending.group = group
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if pending == nil {
			pending = &pendingCh{name: line, group: group}
		}
		ch := pending.toChannel(line, defaults)
		out = append(out, ch)
		pending = nil
	}
	return out
}

type pendingCh struct {
	name    string
	group   string
	logo    string
	headers map[string]string
	parse   int
}

func parseExtinf(line, group string) *pendingCh {
	m := extinfRe.FindStringSubmatch(line)
	p := &pendingCh{group: group, headers: map[string]string{}}
	attrs := ""
	if len(m) >= 4 {
		attrs = m[2]
		p.name = strings.TrimSpace(m[3])
	} else if i := strings.LastIndex(line, ","); i >= 0 {
		p.name = strings.TrimSpace(line[i+1:])
		attrs = line
	}
	for _, am := range attrRe.FindAllStringSubmatch(attrs, -1) {
		key := strings.ToLower(am[1])
		val := am[2]
		switch key {
		case "group-title":
			// V1: use full group-title; encrypted groups with pass field are skipped at source load.
			if val != "" {
				p.group = val
			}
		case "tvg-logo", "logo":
			p.logo = val
		case "tvg-name":
			if p.name == "" {
				p.name = val
			}
		}
	}
	return p
}

func (p *pendingCh) toChannel(rawURL string, defaults map[string]string) Channel {
	u, hdr := player.SplitLiveURLHeaders(rawURL)
	headers := map[string]string{}
	for k, v := range defaults {
		headers[k] = v
	}
	for k, v := range p.headers {
		headers[k] = v
	}
	for k, v := range hdr {
		headers[k] = v
	}
	group := p.group
	if group == "" {
		group = "未分组"
	}
	name := p.name
	if name == "" {
		name = u
	}
	parse := p.parse
	if parse == 0 && !player.IsDirectMedia(u) {
		parse = 1
	}
	id := channelID(group, name, u)
	return Channel{
		ID:      id,
		Name:    name,
		Group:   group,
		URL:     u,
		LogoURL: p.logo,
		Headers: headers,
		Parse:   parse,
		Lines:   []any{},
	}
}

func parseGenreText(body, defaultGroup string, defaults map[string]string) []Channel {
	if defaultGroup == "" {
		defaultGroup = "未分组"
	}
	group := defaultGroup
	var out []Channel
	sc := bufio.NewScanner(strings.NewReader(body))
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	lineHeaders := map[string]string{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.Contains(line, "#genre#") {
			group = strings.TrimSpace(strings.Split(line, "#genre#")[0])
			group = strings.TrimRight(group, ",， \t")
			if group == "" {
				group = defaultGroup
			}
			// reset per-group settings
			lineHeaders = map[string]string{}
			continue
		}
		// ua=/header= setting lines without comma URL
		if !strings.Contains(line, "://") && strings.Contains(line, "=") && !strings.Contains(line, ",") {
			k, v, ok := strings.Cut(line, "=")
			if ok {
				applyLiveSetting(lineHeaders, k, v)
			}
			continue
		}
		name, rawURL, ok := strings.Cut(line, ",")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		rawURL = strings.TrimSpace(rawURL)
		if name == "" || rawURL == "" {
			continue
		}
		u, hdr := player.SplitLiveURLHeaders(rawURL)
		headers := map[string]string{}
		for k, v := range defaults {
			headers[k] = v
		}
		for k, v := range lineHeaders {
			headers[k] = v
		}
		for k, v := range hdr {
			headers[k] = v
		}
		parse := 0
		if !player.IsDirectMedia(u) {
			parse = 1
		}
		out = append(out, Channel{
			ID:      channelID(group, name, u),
			Name:    name,
			Group:   group,
			URL:     u,
			Headers: headers,
			Parse:   parse,
			Lines:   []any{},
		})
	}
	return out
}

func applyLiveSetting(dst map[string]string, key, val string) {
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	switch strings.ToLower(key) {
	case "ua", "user-agent":
		dst["User-Agent"] = val
	case "referer":
		dst["Referer"] = val
	case "origin":
		dst["Origin"] = val
	case "header":
		var m map[string]string
		if json.Unmarshal([]byte(val), &m) == nil {
			for k, v := range m {
				dst[k] = v
			}
		}
	default:
		dst[key] = val
	}
}

func channelID(group, name, rawURL string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = rawURL
	}
	id := group + "|" + base
	return url.PathEscape(id)
}

// ParseInlineLives extracts channels from subscription lives[] entries without network.
func ParseInlineLives(lives []any) []Channel {
	var out []Channel
	for _, item := range lives {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		defaults := liveDefaults(m)
		// inline channels array
		if chans, ok := m["channels"].([]any); ok {
			for _, c := range chans {
				cm, ok := asMap(c)
				if !ok {
					continue
				}
				out = append(out, channelFromMap(cm, name, defaults)...)
			}
			continue
		}
		if groups, ok := m["groups"].([]any); ok {
			for _, g := range groups {
				gm, ok := asMap(g)
				if !ok {
					continue
				}
				gname := str(gm["name"])
				if gname == "" {
					gname = name
				}
				list, _ := gm["channels"].([]any)
				if list == nil {
					list, _ = gm["channel"].([]any)
				}
				for _, c := range list {
					cm, ok := asMap(c)
					if !ok {
						continue
					}
					out = append(out, channelFromMap(cm, gname, defaults)...)
				}
			}
		}
	}
	return out
}

func channelFromMap(m map[string]any, group string, defaults map[string]string) []Channel {
	n := str(m["name"])
	u := str(m["url"])
	if u == "" {
		return nil
	}
	if group == "" {
		group = str(m["group"])
	}
	if group == "" {
		group = "未分组"
	}
	headers := map[string]string{}
	for k, v := range defaults {
		headers[k] = v
	}
	for k, v := range extractHeaders(m) {
		headers[k] = v
	}
	u2, hdr := player.SplitLiveURLHeaders(u)
	for k, v := range hdr {
		headers[k] = v
	}
	parse := toInt(m["parse"])
	if parse == 0 && !player.IsDirectMedia(u2) {
		parse = 1
	}
	return []Channel{{
		ID:      channelID(group, n, u2),
		Name:    firstNonEmpty(n, u2),
		Group:   group,
		URL:     u2,
		LogoURL: str(m["logo"]),
		Headers: headers,
		Parse:   parse,
		Lines:   []any{},
	}}
}

func liveDefaults(m map[string]any) map[string]string {
	h := extractHeaders(m)
	if h == nil {
		h = map[string]string{}
	}
	if ua := str(m["ua"]); ua != "" {
		h["User-Agent"] = ua
	}
	if ref := str(m["referer"]); ref != "" {
		h["Referer"] = ref
	}
	if origin := str(m["origin"]); origin != "" {
		h["Origin"] = origin
	}
	return h
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

func str(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
