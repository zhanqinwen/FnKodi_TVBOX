package tvbox_test

import (
	"testing"

	"github.com/zhanqinwen/FnKodi_TVBOX/fn-tvbox-gateway/internal/tvbox"
)

func TestFilterSitesSkipsType3AndJar(t *testing.T) {
	sites := []tvbox.Site{
		{Key: "ok", Name: "CMS", Type: 1, API: "https://example.com/api"},
		{Key: "jar", Name: "JAR", Type: 3, API: "csp_Foo", Jar: "http://x/a.jar"},
		{Key: "drpy", Name: "DRPY", Type: 3, API: "https://x/drpy2.min.js"},
		{Key: "t4", Name: "T4", Type: 4, API: "https://example.com/t4"},
		{Key: "bad", Name: "Bad", Type: 1, API: "ftp://nope"},
	}
	res := tvbox.FilterSites(sites, nil)
	if len(res.Supported) != 2 {
		t.Fatalf("supported=%d %+v", len(res.Supported), res.Supported)
	}
	if res.SkippedUnsupported != 3 {
		t.Fatalf("skipped=%d", res.SkippedUnsupported)
	}
	if res.Supported[0].SourceType != "cms" || res.Supported[1].SourceType != "t4" {
		t.Fatalf("%+v", res.Supported)
	}
}
