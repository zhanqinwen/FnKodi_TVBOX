package subs

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// DetectKind fetches URL and classifies warehouse / single / live.
func DetectKind(ctx context.Context, client *http.Client, rawURL string) (ProbeResult, error) {
	rawURL = strings.TrimSpace(rawURL)
	if err := ValidateHTTPURL(rawURL); err != nil {
		return ProbeResult{}, err
	}
	data, err := tvbox.FetchConfigGET(ctx, client, rawURL)
	if err != nil {
		return ProbeResult{OK: false, Message: err.Error()}, err
	}
	return ClassifyBody(data, rawURL), nil
}

// ClassifyBody detects kind from already-fetched body bytes.
func ClassifyBody(data []byte, rawURL string) ProbeResult {
	name := defaultNameFromURL(rawURL)
	trim := strings.TrimSpace(string(data))
	if trim == "" {
		return ProbeResult{OK: false, Message: "empty body"}
	}

	// Live playlists often start with #EXTM3U or plain URL lines.
	if looksLikeLive(trim) {
		return ProbeResult{OK: true, DetectedKind: KindLive, SourceCount: 1, Name: name}
	}

	raw, err := tvbox.ParseConfigText(data)
	if err != nil {
		if looksLikeLive(trim) {
			return ProbeResult{OK: true, DetectedKind: KindLive, SourceCount: 1, Name: name}
		}
		return ProbeResult{OK: false, Message: err.Error()}
	}

	entries := tvbox.ListWarehouseEntries(raw)
	if len(entries) > 0 && len(raw.Sites) == 0 {
		return ProbeResult{
			OK:           true,
			DetectedKind: KindWarehouse,
			SourceCount:  len(entries),
			Name:         name,
		}
	}
	if len(raw.Sites) > 0 || len(raw.Lives) > 0 || len(raw.Parses) > 0 {
		return ProbeResult{
			OK:           true,
			DetectedKind: KindSingle,
			SourceCount:  len(raw.Sites),
			Name:         name,
		}
	}
	if len(entries) > 0 {
		return ProbeResult{
			OK:           true,
			DetectedKind: KindWarehouse,
			SourceCount:  len(entries),
			Name:         name,
		}
	}
	return ProbeResult{OK: false, Message: "unrecognized subscription document"}
}

func looksLikeLive(text string) bool {
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "#extm3u") || strings.Contains(lower, "#extinf") {
		return true
	}
	// Reject JSON-looking documents.
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return false
	}
	lines := 0
	httpLines := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines++
		if strings.HasPrefix(strings.ToLower(line), "http://") || strings.HasPrefix(strings.ToLower(line), "https://") {
			httpLines++
		}
		if lines >= 8 {
			break
		}
	}
	return lines > 0 && httpLines == lines
}

// ValidateHTTPURL checks http(s) absolute URL.
func ValidateHTTPURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errBadURL("url is required")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errBadURL("invalid url")
	}
	return nil
}

type badURLError struct{ msg string }

func (e *badURLError) Error() string { return e.msg }

func errBadURL(msg string) error { return &badURLError{msg: msg} }

// IsBadURL reports validation errors suitable for HTTP 400.
func IsBadURL(err error) bool {
	_, ok := err.(*badURLError)
	return ok
}

func defaultNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "订阅"
	}
	host := u.Host
	if !utf8.ValidString(host) {
		return "订阅"
	}
	return host
}
