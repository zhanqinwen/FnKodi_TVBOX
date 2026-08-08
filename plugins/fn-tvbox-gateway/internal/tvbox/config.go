package tvbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/titanous/json5"
)

const maxConfigBytes = 8 * 1024 * 1024

// ParseConfigText parses JSON or JSON5 subscription text.
func ParseConfigText(data []byte) (*RawConfig, error) {
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("subscription too large: %d bytes", len(data))
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, fmt.Errorf("empty subscription body")
	}
	if strings.HasPrefix(text, "<") || strings.Contains(strings.ToLower(text[:min(64, len(text))]), "<html") {
		return nil, fmt.Errorf("subscription looks like HTML")
	}

	var raw RawConfig
	if err := json.Unmarshal([]byte(text), &raw); err == nil {
		return &raw, nil
	}
	if err := json5.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parse subscription: %w", err)
	}
	return &raw, nil
}

// FetchConfigGET downloads a subscription URL.
func FetchConfigGET(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxConfigBytes {
		return nil, fmt.Errorf("subscription too large: content-length %d", resp.ContentLength)
	}
	limited := io.LimitReader(resp.Body, maxConfigBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("subscription too large: > %d bytes", maxConfigBytes)
	}
	if !utf8.Valid(data) {
		// best-effort: treat as bytes; many CMS are UTF-8
	}
	return data, nil
}

// IsWarehouse reports whether the document is a warehouse index.
func IsWarehouse(raw *RawConfig) bool {
	return len(warehouseItems(raw)) > 0 && len(raw.Sites) == 0
}

func warehouseItems(raw *RawConfig) []WarehouseItem {
	for _, list := range [][]WarehouseItem{raw.Urls, raw.StoreHouse, raw.Warehouses, raw.Subscriptions} {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
