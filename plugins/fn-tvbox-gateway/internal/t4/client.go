package t4

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

const maxBody = 8 * 1024 * 1024

// Client talks to TVBox type=4 HTTP spiders.
type Client struct {
	HTTP *http.Client
}

type Operation = cms.Operation

const (
	OpHome     = cms.OpHome
	OpCategory = cms.OpCategory
	OpDetail   = cms.OpDetail
	OpSearch   = cms.OpSearch
	OpPlay     Operation = "play"
)

// Fetch performs a T4 request.
func (c *Client) Fetch(ctx context.Context, site tvbox.Site, op Operation, params map[string]string) (*cms.Normalized, error) {
	if c.HTTP == nil {
		return nil, fmt.Errorf("nil http client")
	}
	u, err := url.Parse(strings.TrimSpace(site.API))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	if op == OpHome {
		q.Set("filter", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range site.HeaderMap() {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("t4 status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("t4 response too large")
	}
	return cms.ParseJSON(data)
}

// EncodeFiltersBase64URL encodes filter object as Base64URL JSON (no padding).
func EncodeFiltersBase64URL(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	b, err := json.Marshal(filters)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// CategoryParams builds T4 category params.
func CategoryParams(categoryID string, page int, filters map[string]string) map[string]string {
	p := map[string]string{
		"ac": "detail",
		"t":  categoryID,
		"pg": strconv.Itoa(page),
	}
	if ext := EncodeFiltersBase64URL(filters); ext != "" {
		p["ext"] = ext
	}
	return p
}

// DetailParams builds T4 detail params.
func DetailParams(mediaID string) map[string]string {
	return map[string]string{"ac": "detail", "ids": mediaID}
}

// SearchParams builds T4 search params.
func SearchParams(keyword string, page int, quick bool) map[string]string {
	p := map[string]string{
		"ac": "detail",
		"wd": keyword,
		"pg": strconv.Itoa(page),
	}
	if quick {
		p["quick"] = "1"
	}
	return p
}

// HomeParams builds T4 home params (filter=true added in Fetch).
func HomeParams() map[string]string {
	return map[string]string{}
}

// Play calls T4 ac=play and returns the raw JSON object.
func (c *Client) Play(ctx context.Context, site tvbox.Site, playURL, flag string) (map[string]any, error) {
	if c.HTTP == nil {
		return nil, fmt.Errorf("nil http client")
	}
	u, err := url.Parse(strings.TrimSpace(site.API))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("ac", "play")
	q.Set("play", playURL)
	if flag != "" {
		q.Set("flag", flag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range site.HeaderMap() {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("t4 play status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("t4 play response too large")
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("t4 play json: %w", err)
	}
	return raw, nil
}
