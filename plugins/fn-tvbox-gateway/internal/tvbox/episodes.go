package tvbox

import (
	"fmt"
	"log/slog"
	"strings"
)

// ParseEpisodes splits vod_play_from / vod_play_url with $$$ / # / $.
//
// Locked strategies:
//   - episodeId = "{groupIndex}:{episodeIndex}" (0-based)
//   - playFrom fallback: from[g] || from[0] || "线路{g+1}"
//   - missing "$": skip entry (log)
//   - empty title with non-empty url: title = "第{n}集" (1-based in group)
//   - "#" inside title: merge fragments until a segment contains "$"
func ParseEpisodes(playFrom, playURL, mediaID string, log *slog.Logger) []Episode {
	urlGroups := splitGroups(playURL)
	fromGroups := splitGroups(playFrom)
	if len(urlGroups) == 1 && strings.TrimSpace(urlGroups[0]) == "" {
		return nil
	}

	var out []Episode
	for g, group := range urlGroups {
		playFromName := pickPlayFrom(fromGroups, g)
		entries := splitEpisodeEntries(group)
		epIdx := 0
		for _, entry := range entries {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			sep := strings.IndexByte(entry, '$')
			if sep < 0 {
				if log != nil {
					log.Info("skip episode entry missing $", "mediaId", mediaID, "group", g, "entry", truncate(entry, 80))
				}
				continue
			}
			title := strings.TrimSpace(entry[:sep])
			url := strings.TrimSpace(entry[sep+1:])
			if url == "" {
				if log != nil {
					log.Info("skip episode entry empty url", "mediaId", mediaID, "group", g)
				}
				continue
			}
			if title == "" {
				title = fmt.Sprintf("第%d集", epIdx+1)
			}
			out = append(out, Episode{
				ID:       fmt.Sprintf("%d:%d", g, epIdx),
				MediaID:  mediaID,
				Title:    title,
				PlayFrom: playFromName,
				PlayURL:  url,
			})
			epIdx++
		}
	}
	return out
}

func splitGroups(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "$$$")
}

func pickPlayFrom(fromGroups []string, g int) string {
	if g < len(fromGroups) {
		if name := strings.TrimSpace(fromGroups[g]); name != "" {
			return name
		}
	}
	if len(fromGroups) > 0 {
		if name := strings.TrimSpace(fromGroups[0]); name != "" {
			return name
		}
	}
	return fmt.Sprintf("线路%d", g+1)
}

// splitEpisodeEntries splits on '#' but keeps merging until an entry is complete
// (contains '$') and the next fragment also looks like a new episode (contains '$'),
// so titles may contain '#' (E04) and play URLs may contain '#fragment'.
func splitEpisodeEntries(group string) []string {
	parts := strings.Split(group, "#")
	if len(parts) == 0 {
		return nil
	}
	var entries []string
	var pending string
	for i, part := range parts {
		if pending == "" {
			pending = part
		} else {
			pending = pending + "#" + part
		}
		atEnd := i == len(parts)-1
		if !strings.Contains(pending, "$") {
			continue
		}
		if atEnd || strings.Contains(parts[i+1], "$") {
			entries = append(entries, pending)
			pending = ""
		}
	}
	if pending != "" {
		entries = append(entries, pending)
	}
	return entries
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
