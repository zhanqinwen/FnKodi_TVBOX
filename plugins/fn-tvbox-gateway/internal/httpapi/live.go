package httpapi

import (
	"net/http"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/live"
)

func handleLiveGroups(svc *live.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chs := svc.Channels(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{"groups": live.Groups(chs)})
	}
}

func handleLiveChannels(svc *live.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chs := svc.Channels(r.Context())
		group := r.URL.Query().Get("group")
		keyword := r.URL.Query().Get("keyword")
		writeJSON(w, http.StatusOK, map[string]any{
			"channels": live.FilterChannels(chs, group, keyword),
		})
	}
}
