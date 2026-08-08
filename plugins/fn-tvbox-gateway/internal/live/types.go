package live

// Group is a live channel group summary.
type Group struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ChannelCount int    `json:"channelCount"`
}

// Channel is a live channel.
type Channel struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Group   string            `json:"group"`
	URL     string            `json:"url"`
	LogoURL string            `json:"logoUrl,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Parse   int               `json:"parse"`
	Lines   []any             `json:"lines"`
}
