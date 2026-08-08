package subs

import (
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

func TestReconcileWarehousePreservesEnabled(t *testing.T) {
	parent := Subscription{
		ID: "subscription-abc", Name: "多仓", URL: "http://x/wh.json",
		Kind: KindWarehouse, Enabled: true, HealthStatus: HealthHealthy,
	}
	child := Subscription{
		ID: CreateWarehouseChildID(parent.ID, "http://a/1.json"),
		Name: "旧名", URL: "http://a/1.json", Kind: KindSingle,
		Enabled: false, HealthStatus: HealthHealthy, ParentID: parent.ID,
		LastSyncAt: "2026-01-01T00:00:00Z",
	}
	items := []Subscription{parent, child}
	entries := []tvbox.WarehouseEntry{
		{Name: "新名A", URL: "http://a/1.json"},
		{Name: "B", URL: "http://b/2.json"},
	}
	out := ReconcileWarehouse(items, parent, entries)
	if len(out) != 3 {
		t.Fatalf("len=%d", len(out))
	}
	var a, b Subscription
	for _, s := range out {
		switch s.URL {
		case "http://a/1.json":
			a = s
		case "http://b/2.json":
			b = s
		}
	}
	if a.Name != "新名A" || a.Enabled != false || a.HealthStatus != HealthHealthy {
		t.Fatalf("preserved child: %+v", a)
	}
	if a.ID != child.ID {
		t.Fatalf("id changed: %s", a.ID)
	}
	if b.Name != "B" || !b.Enabled || b.ParentID != parent.ID {
		t.Fatalf("new child: %+v", b)
	}
}

func TestClassifyWarehouse(t *testing.T) {
	body := []byte(`{"urls":[{"name":"A","url":"http://a/1.json"},{"name":"B","url":"http://b/2.json"}]}`)
	p := ClassifyBody(body, "http://x/wh.json")
	if !p.OK || p.DetectedKind != KindWarehouse || p.SourceCount != 2 {
		t.Fatalf("%+v", p)
	}
}

func TestClassifySingle(t *testing.T) {
	body := []byte(`{"sites":[{"key":"k","name":"N","type":1,"api":"http://x/api"}],"lives":[],"parses":[]}`)
	p := ClassifyBody(body, "http://x/s.json")
	if !p.OK || p.DetectedKind != KindSingle || p.SourceCount != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestClassifyLive(t *testing.T) {
	body := []byte("#EXTM3U\n#EXTINF:-1,CCTV\nhttp://a/1.m3u8\n")
	p := ClassifyBody(body, "http://x/live.m3u")
	if !p.OK || p.DetectedKind != KindLive {
		t.Fatalf("%+v", p)
	}
}

func TestRegistryUpsertAndCascade(t *testing.T) {
	dir := t.TempDir()
	r, err := Load(dir, "http://example.com/a.json")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Configured() {
		t.Fatal("expected configured")
	}
	parent, err := r.UpsertTopLevel("http://example.com/wh.json", "WH", KindWarehouse)
	if err != nil {
		t.Fatal(err)
	}
	items := ReconcileWarehouse(r.List(), parent, []tvbox.WarehouseEntry{
		{Name: "C1", URL: "http://c/1.json"},
	})
	if err := r.SetList(items); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetEnabled(parent.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, s := range r.List() {
		if s.ParentID == parent.ID && s.Enabled {
			t.Fatalf("child should be disabled: %+v", s)
		}
	}
	active := r.ActiveContent()
	for _, s := range active {
		if s.URL == "http://c/1.json" {
			t.Fatal("disabled child should not be active")
		}
	}
}
