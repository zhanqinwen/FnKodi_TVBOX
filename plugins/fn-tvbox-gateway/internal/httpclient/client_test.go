package httpclient_test

import (
	"testing"
	"time"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/httpclient"
)

func TestNewShortHasTimeout(t *testing.T) {
	c := httpclient.NewShort(2*time.Second, "test")
	if c.Timeout != 2*time.Second {
		t.Fatalf("Timeout=%v", c.Timeout)
	}
	if c.Transport == nil {
		t.Fatal("missing Transport")
	}
}

func TestNewProxyNoOverallTimeout(t *testing.T) {
	// P3.5 / P10.2 regression: media proxy must not set Client.Timeout.
	c := httpclient.NewProxy(5*time.Second, "test")
	if c.Timeout != 0 {
		t.Fatalf("proxy Client.Timeout must be 0, got %v", c.Timeout)
	}
	if c.Transport == nil {
		t.Fatal("missing Transport")
	}
}
