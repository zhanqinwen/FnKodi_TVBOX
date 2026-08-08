package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

const searchConcurrency = 10

type aggregateSearchItem struct {
	ID         string   `json:"id"`
	SourceID   string   `json:"sourceId"`
	SourceName string   `json:"sourceName"`
	Title      string   `json:"title"`
	CoverURL   string   `json:"coverUrl,omitempty"`
	Tags       []string `json:"tags"`
	MatchKind  string   `json:"matchKind"`
	MatchScore int      `json:"matchScore"`
}

func (a *MediaAPI) handleAggregateSearch(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "keyword is required")
		return
	}
	page := queryPage(r, 1)
	a.Store.EnsureLoaded(r.Context(), false)

	var targets []tvbox.SupportedSite
	for _, s := range a.Store.Sites() {
		if !s.IsSearchable() {
			continue
		}
		targets = append(targets, s)
	}

	type result struct {
		items []aggregateSearchItem
		fail  bool
	}
	perSourceTimeout := a.HTTPTimeout
	if perSourceTimeout <= 0 {
		perSourceTimeout = 8 * time.Second
	}

	ch := make(chan result, len(targets))
	sem := make(chan struct{}, searchConcurrency)
	var wg sync.WaitGroup
	for _, site := range targets {
		site := site
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(r.Context(), perSourceTimeout)
			defer cancel()
			var norm *cms.Normalized
			var err error
			if site.Type == 4 {
				norm, err = a.T4.Fetch(ctx, site.Site, t4.OpSearch, t4.SearchParams(keyword, page, false))
			} else {
				norm, err = a.CMS.Fetch(ctx, site.Site, cms.OpSearch, cms.SearchParams(keyword, page, false))
			}
			if err != nil || norm == nil {
				ch <- result{fail: true}
				return
			}
			items := make([]aggregateSearchItem, 0, len(norm.List))
			for _, v := range norm.List {
				items = append(items, aggregateSearchItem{
					ID:         v.VodID,
					SourceID:   site.Key,
					SourceName: site.Name,
					Title:      v.VodName,
					CoverURL:   v.VodPic,
					Tags:       splitTags(v.TypeName, v.VodClass, v.VodTag),
					MatchKind:  "contains",
					MatchScore: 80,
				})
			}
			ch <- result{items: items}
		}()
	}
	wg.Wait()
	close(ch)

	var items []aggregateSearchItem
	failed := 0
	for res := range ch {
		if res.fail {
			failed++
			continue
		}
		items = append(items, res.items...)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"keyword":             keyword,
		"page":                page,
		"searchedSourceCount": len(targets),
		"failedSourceCount":   failed,
		"items":               items,
	})
}
