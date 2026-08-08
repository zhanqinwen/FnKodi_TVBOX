package tvbox

import (
	"log/slog"
	"strings"
)

// FilterResult holds supported sites and skip counts.
type FilterResult struct {
	Supported          []SupportedSite
	SkippedUnsupported int
}

// FilterSites keeps type 0/1/2/4 with http(s) API; skips Android/DRPY/jar.
func FilterSites(sites []Site, log *slog.Logger) FilterResult {
	out := FilterResult{}
	for _, s := range sites {
		key := strings.TrimSpace(s.Key)
		name := strings.TrimSpace(s.Name)
		if key == "" || name == "" {
			out.SkippedUnsupported++
			if log != nil {
				log.Info("skip unsupported site", "key", key, "reason", "missing_key_or_name")
			}
			continue
		}
		api := strings.TrimSpace(s.API)
		apiLower := strings.ToLower(api)
		reason := ""
		switch {
		case s.Type == 3:
			reason = "android_or_drpy"
		case strings.Contains(apiLower, "csp_"):
			reason = "android_or_drpy"
		case strings.TrimSpace(s.Jar) != "":
			reason = "android_or_drpy"
		case s.Type != 0 && s.Type != 1 && s.Type != 2 && s.Type != 4:
			reason = "android_or_drpy"
		case !strings.HasPrefix(apiLower, "http://") && !strings.HasPrefix(apiLower, "https://"):
			reason = "android_or_drpy"
		}
		if reason != "" {
			out.SkippedUnsupported++
			if log != nil {
				log.Info("skip unsupported site", "key", key, "reason", reason)
			}
			continue
		}
		st := "cms"
		if s.Type == 4 {
			st = "t4"
		}
		out.Supported = append(out.Supported, SupportedSite{Site: s, SourceType: st})
	}
	return out
}
