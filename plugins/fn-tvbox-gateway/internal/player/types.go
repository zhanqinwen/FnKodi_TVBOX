package player

// ResolveRequest is POST /api/player/resolve body.
type ResolveRequest struct {
	SourceID string            `json:"sourceId"`
	MediaID  string            `json:"mediaId"`
	EpisodeID string           `json:"episodeId"`
	PlayURL  string            `json:"playUrl"`
	PlayFrom string            `json:"playFrom"`
	Headers  map[string]string `json:"headers"`
}

// ResolvedPlay is the playback result.
type ResolvedPlay struct {
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers,omitempty"`
	Format          string            `json:"format,omitempty"`
	Parse           int               `json:"parse"`
	PositionSeconds float64           `json:"positionSeconds,omitempty"`
	Subtitles       []any             `json:"subtitles,omitempty"`
	Danmaku         []any             `json:"danmaku,omitempty"`
	ParserName      string            `json:"parserName,omitempty"`
	ParserURL       string            `json:"parserUrl,omitempty"`
}

// ParseEntry is a TVBox parses[] item.
type ParseEntry struct {
	Name    string            `json:"name"`
	Type    int               `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"header"`
	Ext     any               `json:"ext"`
}
