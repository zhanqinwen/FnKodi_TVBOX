package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
)

type putSubscriptionBody struct {
	URL string `json:"url"`
}

func handleSubscriptionGet(store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sum := store.EnsureLoaded(r.Context(), false)
		writeJSON(w, http.StatusOK, sum)
	}
}

func handleSubscriptionPut(store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body putSubscriptionBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		if err := store.SetURL(body.URL); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		sum := store.EnsureLoaded(r.Context(), true)
		writeJSON(w, http.StatusOK, sum)
	}
}

func handleSubscriptionReload(store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sum := store.EnsureLoaded(r.Context(), true)
		writeJSON(w, http.StatusOK, sum)
	}
}
