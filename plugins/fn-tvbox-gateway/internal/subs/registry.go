package subs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fileName = "subscriptions.json"

// Registry persists and mutates the subscription list.
type Registry struct {
	mu    sync.RWMutex
	path  string
	items []Subscription
}

type filePayload struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// Load opens or bootstraps a registry under dataDir.
// bootstrapURL is used only when no file exists yet.
func Load(dataDir, bootstrapURL string) (*Registry, error) {
	r := &Registry{}
	if strings.TrimSpace(dataDir) != "" {
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			return nil, fmt.Errorf("data dir: %w", err)
		}
		r.path = filepath.Join(dataDir, fileName)
	}
	if r.path != "" {
		if raw, err := os.ReadFile(r.path); err == nil && len(raw) > 0 {
			var p filePayload
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, fmt.Errorf("parse subscriptions.json: %w", err)
			}
			r.items = normalizeList(p.Subscriptions)
			return r, nil
		} else if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	bootstrapURL = strings.TrimSpace(bootstrapURL)
	if bootstrapURL != "" {
		if err := ValidateHTTPURL(bootstrapURL); err != nil {
			return nil, err
		}
		r.items = []Subscription{{
			ID:           newID("subscription"),
			Name:         defaultNameFromURL(bootstrapURL),
			URL:          bootstrapURL,
			Kind:         KindSingle, // refined on first probe/sync
			Enabled:      true,
			HealthStatus: HealthUnknown,
		}}
		if err := r.persistLocked(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// NewMemory creates an empty in-memory registry (tests).
func NewMemory() *Registry {
	return &Registry{}
}

// Configured reports whether any subscription URL exists.
func (r *Registry) Configured() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items) > 0
}

// List returns a copy of all subscriptions.
func (r *Registry) List() []Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Subscription, len(r.items))
	copy(out, r.items)
	return out
}

// Get returns a subscription by id.
func (r *Registry) Get(id string) (Subscription, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.items {
		if s.ID == id {
			return s, true
		}
	}
	return Subscription{}, false
}

// ReplaceAll sets the full list (validated).
func (r *Registry) ReplaceAll(items []Subscription) error {
	items = normalizeList(items)
	if err := validateParents(items); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = items
	return r.persistLocked()
}

// UpsertTopLevel adds or updates a non-child subscription by URL.
func (r *Registry) UpsertTopLevel(url, name, kind string) (Subscription, error) {
	if err := ValidateHTTPURL(url); err != nil {
		return Subscription{}, err
	}
	if kind == "" {
		kind = KindSingle
	}
	if name == "" {
		name = defaultNameFromURL(url)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, s := range r.items {
		if s.ParentID == "" && s.URL == url {
			s.Name = name
			s.Kind = kind
			r.items[i] = s
			if err := r.persistLocked(); err != nil {
				return Subscription{}, err
			}
			return s, nil
		}
	}
	sub := Subscription{
		ID:           newID("subscription"),
		Name:         name,
		URL:          url,
		Kind:         kind,
		Enabled:      true,
		HealthStatus: HealthUnknown,
	}
	r.items = append(r.items, sub)
	if err := r.persistLocked(); err != nil {
		return Subscription{}, err
	}
	return sub, nil
}

// Add inserts a new top-level subscription.
func (r *Registry) Add(url, name, kind string) (Subscription, error) {
	if err := ValidateHTTPURL(url); err != nil {
		return Subscription{}, err
	}
	if kind == "" {
		kind = KindSingle
	}
	if name == "" {
		name = defaultNameFromURL(url)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.items {
		if s.ParentID == "" && s.URL == url {
			return Subscription{}, fmt.Errorf("subscription already exists")
		}
	}
	sub := Subscription{
		ID:           newID("subscription"),
		Name:         name,
		URL:          url,
		Kind:         kind,
		Enabled:      true,
		HealthStatus: HealthUnknown,
	}
	r.items = append(r.items, sub)
	if err := r.persistLocked(); err != nil {
		return Subscription{}, err
	}
	return sub, nil
}

// SetEnabled updates enabled; warehouse parents cascade to children.
func (r *Registry) SetEnabled(id string, enabled bool) (Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := -1
	for i, s := range r.items {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Subscription{}, fmt.Errorf("not found")
	}
	r.items[idx].Enabled = enabled
	if r.items[idx].Kind == KindWarehouse {
		for i, s := range r.items {
			if s.ParentID == id {
				r.items[i].Enabled = enabled
			}
		}
	}
	if err := r.persistLocked(); err != nil {
		return Subscription{}, err
	}
	return r.items[idx], nil
}

// Patch updates name and/or enabled.
func (r *Registry) Patch(id string, name *string, enabled *bool) (Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := -1
	for i, s := range r.items {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Subscription{}, fmt.Errorf("not found")
	}
	if name != nil && strings.TrimSpace(*name) != "" {
		r.items[idx].Name = strings.TrimSpace(*name)
	}
	if enabled != nil {
		r.items[idx].Enabled = *enabled
		if r.items[idx].Kind == KindWarehouse {
			for i, s := range r.items {
				if s.ParentID == id {
					r.items[i].Enabled = *enabled
				}
			}
		}
	}
	if err := r.persistLocked(); err != nil {
		return Subscription{}, err
	}
	return r.items[idx], nil
}

// Delete removes a subscription; deleting a warehouse removes its children.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	out := r.items[:0]
	for _, s := range r.items {
		if s.ID == id || s.ParentID == id {
			found = true
			continue
		}
		out = append(out, s)
	}
	if !found {
		return fmt.Errorf("not found")
	}
	r.items = out
	return r.persistLocked()
}

// SetList replaces items without validation beyond normalize (used after reconcile).
func (r *Registry) SetList(items []Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = normalizeList(items)
	return r.persistLocked()
}

// UpdateMeta updates health / sync / error fields for one id.
func (r *Registry) UpdateMeta(id string, health, lastSync string, lastErr *LastError) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, s := range r.items {
		if s.ID != id {
			continue
		}
		if health != "" {
			r.items[i].HealthStatus = health
		}
		if lastSync != "" {
			r.items[i].LastSyncAt = lastSync
		}
		r.items[i].LastError = lastErr
		return r.persistLocked()
	}
	return fmt.Errorf("not found")
}

// ActiveContent returns enabled non-warehouse subscriptions for catalog load.
func (r *Registry) ActiveContent() []Subscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Subscription
	for _, s := range r.items {
		if !s.Enabled || s.Kind == KindWarehouse {
			continue
		}
		out = append(out, s)
	}
	return out
}

// PrimaryURL returns the first top-level URL (for legacy summary).
func (r *Registry) PrimaryURL() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.items {
		if s.ParentID == "" {
			return s.URL
		}
	}
	return ""
}

// Counts returns total and warehouse-child counts.
func (r *Registry) Counts() (total, children int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total = len(r.items)
	for _, s := range r.items {
		if s.ParentID != "" {
			children++
		}
	}
	return total, children
}

// HasWarehouse reports whether any top-level warehouse exists.
func (r *Registry) HasWarehouse() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.items {
		if s.Kind == KindWarehouse {
			return true
		}
	}
	return false
}

func (r *Registry) persistLocked() error {
	if r.path == "" {
		return nil
	}
	p := filePayload{Subscriptions: r.items}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func normalizeList(items []Subscription) []Subscription {
	out := make([]Subscription, 0, len(items))
	for _, s := range items {
		s.ID = strings.TrimSpace(s.ID)
		s.URL = strings.TrimSpace(s.URL)
		s.Name = strings.TrimSpace(s.Name)
		s.Kind = strings.TrimSpace(s.Kind)
		s.ParentID = strings.TrimSpace(s.ParentID)
		if s.ID == "" || s.URL == "" {
			continue
		}
		if s.Kind == "" {
			s.Kind = KindSingle
		}
		if s.HealthStatus == "" {
			s.HealthStatus = HealthUnknown
		}
		if s.Name == "" {
			s.Name = defaultNameFromURL(s.URL)
		}
		out = append(out, s)
	}
	return out
}

func validateParents(items []Subscription) error {
	byID := map[string]Subscription{}
	for _, s := range items {
		byID[s.ID] = s
	}
	for _, s := range items {
		if s.ParentID == "" {
			continue
		}
		if s.Kind != KindSingle {
			return fmt.Errorf("child %s must be kind=single", s.ID)
		}
		p, ok := byID[s.ParentID]
		if !ok || p.Kind != KindWarehouse {
			return fmt.Errorf("child %s parent must be warehouse", s.ID)
		}
	}
	return nil
}

func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}

// NowUTC returns RFC3339 UTC timestamp.
func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
