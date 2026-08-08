package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/proxy"
)

type proxySessionRequest struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func handleProxySession(store *proxy.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req proxySessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		playURL, exp, err := store.Create(req.URL, req.Headers)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"playUrl":    playURL,
			"expiresAt":  exp,
		})
	}
}

func handleProxyPlay(store *proxy.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("token")
		store.ServePlay(w, r, token)
	}
}
