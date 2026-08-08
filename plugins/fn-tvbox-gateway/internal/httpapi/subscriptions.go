package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/subs"
)

type addSubscriptionBody struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type probeBody struct {
	URL string `json:"url"`
}

type patchSubscriptionBody struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
}

func handleSubscriptionsList(reg *subs.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"subscriptions": reg.List()})
	}
}

func handleSubscriptionsAdd(svc *subs.Service, store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body addSubscriptionBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		sub, err := svc.AddFromURL(r.Context(), body.URL, body.Name)
		if err != nil {
			if subs.IsBadURL(err) {
				writeError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}
			if strings.Contains(err.Error(), "already exists") {
				writeError(w, http.StatusConflict, "conflict", err.Error())
				return
			}
			writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		store.InvalidateCache()
		_ = store.EnsureLoaded(r.Context(), true)
		writeJSON(w, http.StatusOK, map[string]any{"subscription": sub, "subscriptions": svc.Reg.List()})
	}
}

func handleSubscriptionsProbe(svc *subs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body probeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		probe, err := subs.DetectKind(r.Context(), svc.Client, body.URL)
		if err != nil && subs.IsBadURL(err) {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err != nil {
			writeJSON(w, http.StatusOK, probe)
			return
		}
		writeJSON(w, http.StatusOK, probe)
	}
}

func handleSubscriptionPatch(svc *subs.Service, store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body patchSubscriptionBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid json body")
			return
		}
		sub, err := svc.Reg.Patch(id, body.Name, body.Enabled)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		store.InvalidateCache()
		_ = store.EnsureLoaded(r.Context(), true)
		writeJSON(w, http.StatusOK, map[string]any{"subscription": sub, "subscriptions": svc.Reg.List()})
	}
}

func handleSubscriptionDelete(svc *subs.Service, store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.Reg.Delete(id); err != nil {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		store.InvalidateCache()
		_ = store.EnsureLoaded(r.Context(), true)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "subscriptions": svc.Reg.List()})
	}
}

func handleSubscriptionSync(svc *subs.Service, store *catalog.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := svc.Sync(r.Context(), id); err != nil {
			if err.Error() == "not found" {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			sub, _ := svc.Reg.Get(id)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":           false,
				"subscription": sub,
				"subscriptions": svc.Reg.List(),
				"error":        err.Error(),
			})
			return
		}
		store.InvalidateCache()
		_ = store.EnsureLoaded(r.Context(), true)
		sub, _ := svc.Reg.Get(id)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"subscription":  sub,
			"subscriptions": svc.Reg.List(),
		})
	}
}

func handleSubscriptionTest(svc *subs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		probe, err := svc.TestConnectivity(r.Context(), id)
		if err != nil && err.Error() == "not found" {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		sub, _ := svc.Reg.Get(id)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           probe.OK,
			"probe":        probe,
			"subscription": sub,
		})
	}
}
