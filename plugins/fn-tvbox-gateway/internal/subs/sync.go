package subs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

// Service performs probe/sync against upstream using a registry.
type Service struct {
	Reg    *Registry
	Client *http.Client
	Log    *slog.Logger
}

// AddFromURL probes, inserts, and syncs warehouses.
func (s *Service) AddFromURL(ctx context.Context, rawURL, name string) (Subscription, error) {
	probe, err := DetectKind(ctx, s.Client, rawURL)
	if err != nil && !probe.OK {
		if IsBadURL(err) {
			return Subscription{}, err
		}
		return Subscription{}, fmt.Errorf("probe: %w", err)
	}
	if !probe.OK {
		return Subscription{}, fmt.Errorf("probe failed: %s", probe.Message)
	}
	if name == "" {
		name = probe.Name
	}
	sub, err := s.Reg.Add(rawURL, name, probe.DetectedKind)
	if err != nil {
		return Subscription{}, err
	}
	if probe.DetectedKind == KindWarehouse {
		if err := s.Sync(ctx, sub.ID); err != nil {
			return sub, err
		}
		sub, _ = s.Reg.Get(sub.ID)
	} else {
		_ = s.Reg.UpdateMeta(sub.ID, HealthHealthy, NowUTC(), nil)
		sub, _ = s.Reg.Get(sub.ID)
	}
	return sub, nil
}

// UpsertFromURL is legacy PUT /api/subscription behavior (does not remove others).
func (s *Service) UpsertFromURL(ctx context.Context, rawURL string) (Subscription, error) {
	probe, err := DetectKind(ctx, s.Client, rawURL)
	kind := KindSingle
	name := defaultNameFromURL(rawURL)
	if err == nil && probe.OK {
		kind = probe.DetectedKind
		if probe.Name != "" {
			name = probe.Name
		}
	} else if IsBadURL(err) {
		return Subscription{}, err
	}
	sub, err := s.Reg.UpsertTopLevel(rawURL, name, kind)
	if err != nil {
		return Subscription{}, err
	}
	if kind == KindWarehouse {
		if syncErr := s.Sync(ctx, sub.ID); syncErr != nil {
			_ = s.Reg.UpdateMeta(sub.ID, HealthError, "", &LastError{
				Code: "upstream_error", Message: syncErr.Error(), At: time.Now().UTC(),
			})
			return sub, syncErr
		}
	} else if err == nil && probe.OK {
		_ = s.Reg.UpdateMeta(sub.ID, HealthHealthy, NowUTC(), nil)
	}
	sub, _ = s.Reg.Get(sub.ID)
	return sub, nil
}

// Sync reconciles a warehouse or refreshes a single/live health check.
func (s *Service) Sync(ctx context.Context, id string) error {
	sub, ok := s.Reg.Get(id)
	if !ok {
		return fmt.Errorf("not found")
	}
	data, err := tvbox.FetchConfigGET(ctx, s.Client, sub.URL)
	if err != nil {
		_ = s.Reg.UpdateMeta(id, HealthError, "", &LastError{
			Code: classifyCode(err), Message: err.Error(), At: time.Now().UTC(),
		})
		return err
	}

	if sub.Kind == KindWarehouse || (sub.ParentID == "" && tvbox.IsWarehouse(mustParse(data))) {
		raw, perr := tvbox.ParseConfigText(data)
		if perr != nil {
			_ = s.Reg.UpdateMeta(id, HealthError, "", &LastError{
				Code: "upstream_error", Message: perr.Error(), At: time.Now().UTC(),
			})
			return perr
		}
		entries := tvbox.ListWarehouseEntries(raw)
		if len(entries) == 0 {
			_ = s.Reg.UpdateMeta(id, HealthError, "", &LastError{
				Code: "upstream_error", Message: "warehouse has no children", At: time.Now().UTC(),
			})
			return fmt.Errorf("warehouse has no children")
		}
		parent := sub
		parent.Kind = KindWarehouse
		items := ReconcileWarehouse(s.Reg.List(), parent, entries)
		// ensure parent kind persisted
		for i := range items {
			if items[i].ID == parent.ID {
				items[i].Kind = KindWarehouse
				items[i].HealthStatus = HealthHealthy
				items[i].LastSyncAt = NowUTC()
				items[i].LastError = nil
			}
		}
		if err := s.Reg.SetList(items); err != nil {
			return err
		}
		return nil
	}

	// single / live / child
	probe := ClassifyBody(data, sub.URL)
	health := HealthHealthy
	var lastErr *LastError
	if !probe.OK {
		health = HealthError
		lastErr = &LastError{Code: "upstream_error", Message: probe.Message, At: time.Now().UTC()}
	}
	return s.Reg.UpdateMeta(id, health, NowUTC(), lastErr)
}

// TestConnectivity probes without reconciling warehouse children.
func (s *Service) TestConnectivity(ctx context.Context, id string) (ProbeResult, error) {
	sub, ok := s.Reg.Get(id)
	if !ok {
		return ProbeResult{}, fmt.Errorf("not found")
	}
	probe, err := DetectKind(ctx, s.Client, sub.URL)
	health := HealthHealthy
	var lastErr *LastError
	if err != nil || !probe.OK {
		health = HealthError
		msg := probe.Message
		if err != nil {
			msg = err.Error()
		}
		lastErr = &LastError{Code: classifyCode(err), Message: msg, At: time.Now().UTC()}
		_ = s.Reg.UpdateMeta(id, health, "", lastErr)
		return probe, err
	}
	_ = s.Reg.UpdateMeta(id, health, "", nil)
	return probe, nil
}

func mustParse(data []byte) *tvbox.RawConfig {
	raw, err := tvbox.ParseConfigText(data)
	if err != nil {
		return &tvbox.RawConfig{}
	}
	return raw
}

func classifyCode(err error) string {
	if err == nil {
		return "upstream_error"
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") {
		return "upstream_timeout"
	}
	return "upstream_error"
}
