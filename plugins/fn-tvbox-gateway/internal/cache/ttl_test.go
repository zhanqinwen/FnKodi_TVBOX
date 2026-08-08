package cache_test

import (
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/cache"
)

func TestTTLGetSet(t *testing.T) {
	c := cache.NewTTL(50 * time.Millisecond)
	c.Set("k", []byte("v"))
	got, ok := c.Get("k")
	if !ok || string(got) != "v" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired")
	}
}
