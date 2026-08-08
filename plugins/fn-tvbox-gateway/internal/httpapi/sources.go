package httpapi

import (
	"net/http"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

type sourceDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Runtime     string         `json:"runtime"`
	Enabled     bool           `json:"enabled"`
	Hidden      bool           `json:"hidden"`
	ContentKind string         `json:"contentKind"`
	QuickSearch bool           `json:"quickSearch"`
	Capabilities sourceCaps    `json:"capabilities"`
}

type sourceCaps struct {
	Detail        bool `json:"detail"`
	Headers       bool `json:"headers"`
	Live          bool `json:"live"`
	Play          bool `json:"play"`
	Search        bool `json:"search"`
	RequiresProxy bool `json:"requiresProxy"`
	Audiobook     bool `json:"audiobook"`
	Music         bool `json:"music"`
}

func handleSourcesList(store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store.EnsureLoaded(r.Context(), false)
		sites := store.Sites()
		out := make([]sourceDTO, 0, len(sites))
		for _, s := range sites {
			out = append(out, toSourceDTO(s))
		}
		writeJSON(w, http.StatusOK, map[string]any{"sources": out})
	}
}

func toSourceDTO(s tvbox.SupportedSite) sourceDTO {
	return sourceDTO{
		ID:          s.Key,
		Name:        s.Name,
		Type:        s.SourceType,
		Runtime:     "http",
		Enabled:     true,
		Hidden:      s.IsHidden(),
		ContentKind: "vod",
		QuickSearch: s.IsQuickSearch(),
		Capabilities: sourceCaps{
			Detail:        true,
			Headers:       len(s.HeaderMap()) > 0,
			Live:          false,
			Play:          true,
			Search:        s.IsSearchable(),
			RequiresProxy: false,
			Audiobook:     false,
			Music:         false,
		},
	}
}
