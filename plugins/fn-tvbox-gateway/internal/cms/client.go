package cms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

const maxBody = 8 * 1024 * 1024

// Client talks to CMS type 0/1/2 APIs.
type Client struct {
	HTTP *http.Client
}

type Operation string

const (
	OpHome     Operation = "home"
	OpCategory Operation = "category"
	OpDetail   Operation = "detail"
	OpSearch   Operation = "search"
)

// Fetch performs a CMS request and returns normalized JSON-shaped data.
func (c *Client) Fetch(ctx context.Context, site tvbox.Site, op Operation, params map[string]string) (*Normalized, error) {
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
	switch site.Type {
	case 0:
		switch op {
		case OpHome:
			q.Set("ac", "list")
		case OpCategory, OpDetail, OpSearch:
			q.Set("ac", "videolist")
		}
	default:
		// type 1/2
		if op != OpHome {
			if q.Get("ac") == "" {
				q.Set("ac", "detail")
			}
		}
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
		return nil, fmt.Errorf("cms status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("cms response too large")
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	trim := strings.TrimSpace(string(data))
	if site.Type == 0 || strings.Contains(ct, "xml") || strings.HasPrefix(trim, "<") {
		return ParseLegacyXML(data)
	}
	return ParseJSON(data)
}

// ParseJSON parses standard CMS JSON.
func ParseJSON(data []byte) (*Normalized, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("cms json: %w", err)
	}
	out := &Normalized{}
	if v, ok := raw["class"]; ok {
		_ = json.Unmarshal(v, &out.Class)
	}
	if v, ok := raw["list"]; ok {
		_ = json.Unmarshal(v, &out.List)
	}
	out.Page = jsonInt(raw, "page")
	out.PageCount = jsonInt(raw, "pagecount")
	out.Total = jsonInt(raw, "total")
	if v, ok := raw["filters"]; ok {
		var f any
		if err := json.Unmarshal(v, &f); err == nil {
			out.Filters = f
		}
	}
	return out, nil
}

func jsonInt(raw map[string]json.RawMessage, key string) int {
	v, ok := raw[key]
	if !ok {
		return 0
	}
	var n int
	if err := json.Unmarshal(v, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(s))
		return n
	}
	return 0
}

// HomeParams builds home/list params.
func HomeParams() map[string]string {
	return map[string]string{}
}

// CategoryParams builds category list params.
func CategoryParams(categoryID string, page int, filtersJSON string) map[string]string {
	p := map[string]string{
		"t":  categoryID,
		"pg": strconv.Itoa(page),
	}
	if filtersJSON != "" {
		p["f"] = filtersJSON
	}
	return p
}

// DetailParams builds detail params.
func DetailParams(mediaID string) map[string]string {
	return map[string]string{"ids": mediaID}
}

// SearchParams builds search params.
func SearchParams(keyword string, page int, quick bool) map[string]string {
	p := map[string]string{
		"wd": keyword,
		"pg": strconv.Itoa(page),
	}
	if quick {
		p["quick"] = "1"
	}
	return p
}
