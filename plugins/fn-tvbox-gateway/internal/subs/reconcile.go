package subs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// CreateWarehouseChildID builds a stable child id from parent id + child URL.
func CreateWarehouseChildID(parentID, childURL string) string {
	sum := sha256.Sum256([]byte(parentID + "\x00" + childURL))
	return fmt.Sprintf("%s-child-%s", parentID, hex.EncodeToString(sum[:])[:12])
}

// ReconcileWarehouse replaces children of parent with entries, preserving
// enabled/health/lastSync for unchanged URLs (bao-tv-box semantics).
func ReconcileWarehouse(items []Subscription, parent Subscription, entries []tvbox.WarehouseEntry) []Subscription {
	existingByURL := map[string]Subscription{}
	for _, s := range items {
		if s.ParentID == parent.ID {
			existingByURL[s.URL] = s
		}
	}

	children := make([]Subscription, 0, len(entries))
	for _, e := range entries {
		if existing, ok := existingByURL[e.URL]; ok {
			existing.Kind = KindSingle
			existing.Name = e.Name
			existing.ParentID = parent.ID
			existing.URL = e.URL
			children = append(children, existing)
			continue
		}
		children = append(children, Subscription{
			ID:           CreateWarehouseChildID(parent.ID, e.URL),
			Name:         e.Name,
			URL:          e.URL,
			Kind:         KindSingle,
			Enabled:      parent.Enabled,
			HealthStatus: HealthUnknown,
			ParentID:     parent.ID,
		})
	}

	out := make([]Subscription, 0, len(items)+len(children))
	inserted := false
	for _, s := range items {
		if s.ParentID == parent.ID {
			continue
		}
		out = append(out, s)
		if s.ID == parent.ID {
			out = append(out, children...)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, parent)
		out = append(out, children...)
	}
	return out
}
