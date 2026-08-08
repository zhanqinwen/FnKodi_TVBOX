package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/player"
)

func handlePlayerResolve(resolver *player.Resolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req player.ResolveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		req.PlayURL = strings.TrimSpace(req.PlayURL)
		if req.PlayURL == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "playUrl is required")
			return
		}
		if req.SourceID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "sourceId is required")
			return
		}
		res, err := resolver.Resolve(r.Context(), req)
		if err != nil {
			msg := err.Error()
			code := "resolve_failed"
			status := http.StatusBadGateway
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") {
				code, status = "upstream_timeout", http.StatusGatewayTimeout
			}
			writeError(w, status, code, msg)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
