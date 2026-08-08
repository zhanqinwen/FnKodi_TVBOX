package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cache"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cms"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/t4"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// MediaAPI serves categories/media/detail/search for a source.
type MediaAPI struct {
	Store       *catalog.Store
	CMS         *cms.Client
	T4          *t4.Client
	MediaTTL    *cache.TTL
	HTTPTimeout time.Duration
	Log         interface{ Info(string, ...any) }
}

type categoryDTO struct {
	ID       string      `json:"id"`
	SourceID string      `json:"sourceId"`
	Name     string      `json:"name"`
	Folder   bool        `json:"folder"`
	Filters  []filterDTO `json:"filters,omitempty"`
}

type filterDTO struct {
	Key     string            `json:"key"`
	Name    string            `json:"name"`
	Options []filterOptionDTO `json:"options"`
}

type filterOptionDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type mediaItemDTO struct {
	ID          string   `json:"id"`
	SourceID    string   `json:"sourceId"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle,omitempty"`
	CoverURL    string   `json:"coverUrl,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Year        string   `json:"year,omitempty"`
	Kind        string   `json:"kind"`
}

type mediaPageDTO struct {
	Items     []mediaItemDTO `json:"items"`
	Page      int            `json:"page"`
	PageCount int            `json:"pageCount,omitempty"`
	Total     int            `json:"total,omitempty"`
}

type detailDTO struct {
	ID          string          `json:"id"`
	SourceID    string          `json:"sourceId"`
	Title       string          `json:"title"`
	CoverURL    string          `json:"coverUrl,omitempty"`
	Description string          `json:"description,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Year        string          `json:"year,omitempty"`
	Actors      string          `json:"actors,omitempty"`
	Director    string          `json:"director,omitempty"`
	Area        string          `json:"area,omitempty"`
	Remarks     string          `json:"remarks,omitempty"`
	Episodes    []tvbox.Episode `json:"episodes"`
}

func (a *MediaAPI) handleCategories(w http.ResponseWriter, r *http.Request) {
	site, ok := a.site(w, r)
	if !ok {
		return
	}
	ckey := "cat:" + site.Key
	if a.MediaTTL != nil {
		if raw, hit := a.MediaTTL.Get(ckey); hit {
			writeJSONBytes(w, http.StatusOK, raw)
			return
		}
	}
	norm, err := a.fetchHome(r, site)
	if err != nil {
		a.upstreamErr(w, err)
		return
	}
	cats := make([]categoryDTO, 0, len(norm.Class))
	for _, c := range norm.Class {
		cats = append(cats, categoryDTO{
			ID:       c.TypeID,
			SourceID: site.Key,
			Name:     c.TypeName,
			Folder:   false,
			Filters:  parseFiltersForCategory(norm.Filters, c.TypeID),
		})
	}
	body := map[string]any{"categories": cats}
	if a.MediaTTL != nil {
		if b, err := json.Marshal(body); err == nil {
			a.MediaTTL.Set(ckey, b)
			writeJSONBytes(w, http.StatusOK, b)
			return
		}
	}
	writeJSON(w, http.StatusOK, body)
}

func (a *MediaAPI) handleMedia(w http.ResponseWriter, r *http.Request) {
	site, ok := a.site(w, r)
	if !ok {
		return
	}
	categoryID := strings.TrimSpace(r.URL.Query().Get("categoryId"))
	if categoryID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "categoryId is required")
		return
	}
	page := queryPage(r, 1)
	filtersRaw := strings.TrimSpace(r.URL.Query().Get("filters"))
	filters := parseFiltersQuery(filtersRaw)
	ckey := "media:" + site.Key + ":" + categoryID + ":" + strconv.Itoa(page) + ":" + filtersRaw
	if a.MediaTTL != nil {
		if raw, hit := a.MediaTTL.Get(ckey); hit {
			writeJSONBytes(w, http.StatusOK, raw)
			return
		}
	}
	norm, err := a.fetchCategory(r, site, categoryID, page, filters)
	if err != nil {
		a.upstreamErr(w, err)
		return
	}
	pageDTO := toMediaPage(site.Key, norm, page)
	if a.MediaTTL != nil {
		if b, err := json.Marshal(pageDTO); err == nil {
			a.MediaTTL.Set(ckey, b)
			writeJSONBytes(w, http.StatusOK, b)
			return
		}
	}
	writeJSON(w, http.StatusOK, pageDTO)
}

func (a *MediaAPI) handleDetail(w http.ResponseWriter, r *http.Request) {
	site, ok := a.site(w, r)
	if !ok {
		return
	}
	mediaID := strings.TrimSpace(r.URL.Query().Get("mediaId"))
	if mediaID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "mediaId is required")
		return
	}
	norm, err := a.fetchDetail(r, site, mediaID)
	if err != nil {
		a.upstreamErr(w, err)
		return
	}
	if len(norm.List) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "media not found")
		return
	}
	vod := norm.List[0]
	eps := tvbox.ParseEpisodes(vod.VodPlayFrom, vod.VodPlayURL, vod.VodID, nil)
	writeJSON(w, http.StatusOK, detailDTO{
		ID:          vod.VodID,
		SourceID:    site.Key,
		Title:       vod.VodName,
		CoverURL:    vod.VodPic,
		Description: vod.VodContent,
		Tags:        splitTags(vod.TypeName, vod.VodClass, vod.VodTag),
		Year:        vod.VodYear,
		Actors:      vod.VodActor,
		Director:    vod.VodDirector,
		Area:        vod.VodArea,
		Remarks:     vod.VodRemarks,
		Episodes:    eps,
	})
}

func (a *MediaAPI) handleSourceSearch(w http.ResponseWriter, r *http.Request) {
	site, ok := a.site(w, r)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "keyword is required")
		return
	}
	page := queryPage(r, 1)
	quick := r.URL.Query().Get("quick") == "1"
	norm, err := a.fetchSearch(r, site, keyword, page, quick)
	if err != nil {
		a.upstreamErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMediaPage(site.Key, norm, page))
}

func (a *MediaAPI) site(w http.ResponseWriter, r *http.Request) (tvbox.SupportedSite, bool) {
	a.Store.EnsureLoaded(r.Context(), false)
	id := r.PathValue("sourceId")
	site, ok := a.Store.SiteByID(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "source not found")
		return tvbox.SupportedSite{}, false
	}
	return site, true
}

func (a *MediaAPI) fetchHome(r *http.Request, site tvbox.SupportedSite) (*cms.Normalized, error) {
	ctx := r.Context()
	if site.Type == 4 {
		return a.T4.Fetch(ctx, site.Site, t4.OpHome, t4.HomeParams())
	}
	return a.CMS.Fetch(ctx, site.Site, cms.OpHome, cms.HomeParams())
}

func (a *MediaAPI) fetchCategory(r *http.Request, site tvbox.SupportedSite, categoryID string, page int, filters map[string]string) (*cms.Normalized, error) {
	ctx := r.Context()
	if site.Type == 4 {
		return a.T4.Fetch(ctx, site.Site, t4.OpCategory, t4.CategoryParams(categoryID, page, filters))
	}
	fJSON := ""
	if len(filters) > 0 {
		b, _ := json.Marshal(filters)
		fJSON = string(b)
	}
	return a.CMS.Fetch(ctx, site.Site, cms.OpCategory, cms.CategoryParams(categoryID, page, fJSON))
}

func (a *MediaAPI) fetchDetail(r *http.Request, site tvbox.SupportedSite, mediaID string) (*cms.Normalized, error) {
	ctx := r.Context()
	if site.Type == 4 {
		return a.T4.Fetch(ctx, site.Site, t4.OpDetail, t4.DetailParams(mediaID))
	}
	return a.CMS.Fetch(ctx, site.Site, cms.OpDetail, cms.DetailParams(mediaID))
}

func (a *MediaAPI) fetchSearch(r *http.Request, site tvbox.SupportedSite, keyword string, page int, quick bool) (*cms.Normalized, error) {
	ctx := r.Context()
	if site.Type == 4 {
		return a.T4.Fetch(ctx, site.Site, t4.OpSearch, t4.SearchParams(keyword, page, quick))
	}
	return a.CMS.Fetch(ctx, site.Site, cms.OpSearch, cms.SearchParams(keyword, page, quick))
}

func (a *MediaAPI) upstreamErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	code, status := "upstream_error", http.StatusBadGateway
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") {
		code, status = "upstream_timeout", http.StatusGatewayTimeout
	}
	writeError(w, status, code, msg)
}

func toMediaPage(sourceID string, norm *cms.Normalized, page int) mediaPageDTO {
	items := make([]mediaItemDTO, 0, len(norm.List))
	for _, v := range norm.List {
		items = append(items, mediaItemDTO{
			ID:          v.VodID,
			SourceID:    sourceID,
			Title:       v.VodName,
			Subtitle:    v.VodRemarks,
			CoverURL:    v.VodPic,
			Description: v.VodContent,
			Tags:        splitTags(v.TypeName, v.VodClass, v.VodTag),
			Year:        v.VodYear,
			Kind:        "media",
		})
	}
	p := page
	if norm.Page > 0 {
		p = norm.Page
	}
	return mediaPageDTO{Items: items, Page: p, PageCount: norm.PageCount, Total: norm.Total}
}

func queryPage(r *http.Request, def int) int {
	p, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || p < 1 {
		return def
	}
	return p
}

func parseFiltersQuery(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func parseFiltersForCategory(filters any, categoryID string) []filterDTO {
	if filters == nil {
		return nil
	}
	// filters may be map[categoryId][] or []
	b, err := json.Marshal(filters)
	if err != nil {
		return nil
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(b, &asMap); err == nil {
		if raw, ok := asMap[categoryID]; ok {
			return decodeFilterList(raw)
		}
	}
	return decodeFilterList(b)
}

func decodeFilterList(raw json.RawMessage) []filterDTO {
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	out := make([]filterDTO, 0, len(list))
	for _, item := range list {
		key, _ := item["key"].(string)
		name, _ := item["name"].(string)
		if key == "" {
			continue
		}
		if name == "" {
			name = key
		}
		fd := filterDTO{Key: key, Name: name}
		opts, _ := item["value"].([]any)
		if opts == nil {
			opts, _ = item["options"].([]any)
		}
		for _, o := range opts {
			om, ok := o.(map[string]any)
			if !ok {
				continue
			}
			label := firstString(om, "n", "name", "label")
			value := firstString(om, "v", "value")
			fd.Options = append(fd.Options, filterOptionDTO{Label: label, Value: value})
		}
		out = append(out, fd)
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			}
		}
	}
	return ""
}

func splitTags(parts ...string) []string {
	var tags []string
	seen := map[string]struct{}{}
	for _, p := range parts {
		for _, piece := range strings.FieldsFunc(p, func(r rune) bool {
			return r == ',' || r == '/' || r == '|'
		}) {
			t := strings.TrimSpace(piece)
			if t == "" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			tags = append(tags, t)
		}
	}
	return tags
}
