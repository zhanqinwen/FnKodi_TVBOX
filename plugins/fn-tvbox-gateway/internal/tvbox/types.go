package tvbox

import "time"

// RawConfig is a decoded TVBox subscription document.
type RawConfig struct {
	Sites   []Site        `json:"sites"`
	Lives   []any         `json:"lives"`
	Parses  []any         `json:"parses"`
	Spider  string        `json:"spider"`
	Urls    []WarehouseItem `json:"urls"`
	StoreHouse []WarehouseItem `json:"storeHouse"`
	Warehouses []WarehouseItem `json:"warehouses"`
	Subscriptions []WarehouseItem `json:"subscriptions"`
}

// WarehouseItem is one entry in a multi-warehouse list.
type WarehouseItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	SourceURL string `json:"sourceUrl"`
	API  string `json:"api"`
	SourceName string `json:"sourceName"`
	Title string `json:"title"`
}

// Site is a TVBox site entry.
type Site struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Type        int               `json:"type"`
	API         string            `json:"api"`
	Searchable  *int              `json:"searchable"`
	QuickSearch *int              `json:"quickSearch"`
	Filterable  *int              `json:"filterable"`
	Changeable  *int              `json:"changeable"`
	PlayerType  any               `json:"playerType"`
	Ext         any               `json:"ext"`
	Jar         string            `json:"jar"`
	Headers     map[string]string `json:"header"`
	HeadersAlt  map[string]string `json:"headers"`
	Hide        any               `json:"hide"`
	Indexs      any               `json:"indexs"`
}

// IsSearchable mirrors TVBox JS: missing searchable => true.
func (s Site) IsSearchable() bool {
	if s.Searchable == nil {
		return true
	}
	return *s.Searchable != 0
}

// IsQuickSearch mirrors TVBox JS quickSearch flag.
func (s Site) IsQuickSearch() bool {
	if s.QuickSearch == nil {
		return false
	}
	return *s.QuickSearch != 0
}

// HeaderMap returns site request headers.
func (s Site) HeaderMap() map[string]string {
	if len(s.Headers) > 0 {
		return s.Headers
	}
	return s.HeadersAlt
}

// IsHidden reports hide flag.
func (s Site) IsHidden() bool {
	switch v := s.Hide.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return v == "1" || v == "true"
	default:
		return false
	}
}

// SupportedSite is a filtered HTTP-capable site exposed by the gateway.
type SupportedSite struct {
	Site
	SourceType string // cms | t4
}

// Episode is a playable episode after $$$/#/$ split.
type Episode struct {
	ID       string `json:"id"`
	MediaID  string `json:"mediaId"`
	Title    string `json:"title"`
	PlayFrom string `json:"playFrom"`
	PlayURL  string `json:"playUrl"`
}

// LastError is a subscription failure snapshot.
type LastError struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// Summary is the subscription API summary.
type Summary struct {
	URL                string     `json:"url"`
	Kind               string     `json:"kind"`
	LoadedAt           *time.Time `json:"loadedAt"`
	SiteCount          int        `json:"siteCount"`
	SkippedUnsupported int        `json:"skippedUnsupported"`
	LiveCount          int        `json:"liveCount"`
	ParseCount         int        `json:"parseCount"`
	LastError          *LastError `json:"lastError"`
}
