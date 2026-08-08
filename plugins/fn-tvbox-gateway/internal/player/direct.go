package player

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

var directMediaExt = regexp.MustCompile(`(?i)\.(?:m3u8|mp4|flv|mkv|m4v|mov|ts|webm)$`)

// IsDirectMedia reports whether url looks like a playable media URL by path extension.
func IsDirectMedia(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	// strip |header suffix used by some live sources
	if i := strings.IndexByte(raw, '|'); i >= 0 {
		raw = raw[:i]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return directMediaExt.MatchString(path.Ext(u.Path)) || directMediaExt.MatchString(u.Path)
}

// GuessFormat returns a coarse format hint.
func GuessFormat(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	p := strings.ToLower(u.Path)
	switch {
	case strings.HasSuffix(p, ".m3u8"):
		return "hls"
	case strings.HasSuffix(p, ".mp4"), strings.HasSuffix(p, ".m4v"), strings.HasSuffix(p, ".mov"):
		return "mp4"
	case strings.HasSuffix(p, ".flv"):
		return "flv"
	case strings.HasSuffix(p, ".mkv"):
		return "mkv"
	case strings.HasSuffix(p, ".ts"):
		return "ts"
	default:
		return ""
	}
}
