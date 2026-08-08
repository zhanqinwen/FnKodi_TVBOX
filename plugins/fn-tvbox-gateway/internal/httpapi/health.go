package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/catalog"
	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/config"
)

const apiVersion = "v1"

type healthResponse struct {
	OK                     bool   `json:"ok"`
	Version                string `json:"version"`
	APIVersion             string `json:"apiVersion"`
	SubscriptionConfigured bool   `json:"subscriptionConfigured"`
}

func handleHealth(cfg *config.Config, store *catalog.Store) http.HandlerFunc {
	// Cache marshaled bodies for the two configured states to avoid per-request Marshal.
	var once sync.Once
	var bodyTrue, bodyFalse []byte
	prepare := func() {
		once.Do(func() {
			bodyTrue, _ = json.Marshal(healthResponse{
				OK: true, Version: cfg.Version, APIVersion: apiVersion, SubscriptionConfigured: true,
			})
			bodyFalse, _ = json.Marshal(healthResponse{
				OK: true, Version: cfg.Version, APIVersion: apiVersion, SubscriptionConfigured: false,
			})
		})
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
			return
		}
		prepare()
		configured := cfg.SubscriptionConfigured()
		if store != nil {
			configured = store.Configured()
		}
		if configured {
			writeJSONBytes(w, http.StatusOK, bodyTrue)
			return
		}
		writeJSONBytes(w, http.StatusOK, bodyFalse)
	}
}
