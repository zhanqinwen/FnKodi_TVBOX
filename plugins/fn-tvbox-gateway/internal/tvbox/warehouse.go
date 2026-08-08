package tvbox

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
)

const (
	maxWarehouseChildren = 100
	warehouseConcurrency = 4
)

// WarehouseEntry is a normalized multi-warehouse child.
type WarehouseEntry struct {
	Name string
	URL  string
}

// ListWarehouseEntries extracts and de-duplicates HTTP child entries (max 100).
func ListWarehouseEntries(raw *RawConfig) []WarehouseEntry {
	items := warehouseItems(raw)
	if len(items) == 0 {
		return nil
	}
	var out []WarehouseEntry
	seen := map[string]struct{}{}
	for _, it := range items {
		u := firstNonEmpty(it.URL, it.SourceURL, it.API)
		u = strings.TrimSpace(u)
		lower := strings.ToLower(u)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		name := firstNonEmpty(it.Name, it.SourceName, it.Title)
		if name == "" {
			if pu, err := url.Parse(u); err == nil && pu.Host != "" {
				name = pu.Host
			} else {
				name = "未命名子仓"
			}
		}
		out = append(out, WarehouseEntry{Name: name, URL: u})
		if len(out) >= maxWarehouseChildren {
			break
		}
	}
	return out
}

// ExpandWarehouseHTTP fetches child subscriptions and merges sites/lives/parses.
func ExpandWarehouseHTTP(ctx context.Context, doFetch func(ctx context.Context, rawURL string) ([]byte, error), raw *RawConfig, log *slog.Logger) (*RawConfig, error) {
	entries := ListWarehouseEntries(raw)
	if len(entries) == 0 {
		return raw, nil
	}

	type childJob struct {
		name string
		url  string
	}
	jobs := make([]childJob, 0, len(entries))
	for _, e := range entries {
		jobs = append(jobs, childJob{name: e.Name, url: e.URL})
	}

	merged := &RawConfig{
		Sites:  append([]Site{}, raw.Sites...),
		Lives:  append([]any{}, raw.Lives...),
		Parses: append([]any{}, raw.Parses...),
		Spider: raw.Spider,
	}

	sem := make(chan struct{}, warehouseConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := doFetch(ctx, job.url)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if log != nil {
					log.Info("warehouse child fetch failed", "name", job.name, "url", job.url, "err", err)
				}
				if firstErr == nil {
					firstErr = fmt.Errorf("warehouse child %s: %w", job.url, err)
				}
				return
			}
			child, err := ParseConfigText(data)
			if err != nil {
				if log != nil {
					log.Info("warehouse child parse failed", "url", job.url, "err", err)
				}
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			merged.Sites = append(merged.Sites, child.Sites...)
			merged.Lives = append(merged.Lives, child.Lives...)
			merged.Parses = append(merged.Parses, child.Parses...)
		}()
	}
	wg.Wait()

	if len(merged.Sites) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return merged, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
