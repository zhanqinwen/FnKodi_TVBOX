package cms

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Normalized is CMS JSON-shaped data from XML or JSON.
type Normalized struct {
	Class     []ClassItem `json:"class"`
	List      []Vod       `json:"list"`
	Page      int         `json:"page"`
	PageCount int         `json:"pagecount"`
	Total     int         `json:"total"`
	Filters   any         `json:"filters"`
}

type ClassItem struct {
	TypeID   string `json:"type_id"`
	TypeName string `json:"type_name"`
}

type Vod struct {
	VodID       string `json:"vod_id"`
	VodName     string `json:"vod_name"`
	VodPic      string `json:"vod_pic"`
	VodRemarks  string `json:"vod_remarks"`
	VodYear     string `json:"vod_year"`
	VodContent  string `json:"vod_content"`
	VodActor    string `json:"vod_actor"`
	VodDirector string `json:"vod_director"`
	VodArea     string `json:"vod_area"`
	TypeName    string `json:"type_name"`
	VodClass    string `json:"vod_class"`
	VodTag      string `json:"vod_tag"`
	VodPlayFrom string `json:"vod_play_from"`
	VodPlayURL  string `json:"vod_play_url"`
}

type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Class   *xmlClass  `xml:"class"`
	List    *xmlList   `xml:"list"`
	Videos  []xmlVideo `xml:"video"`
}

type xmlClass struct {
	Ty []xmlTy `xml:"ty"`
}

type xmlTy struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

type xmlList struct {
	Page        string     `xml:"page,attr"`
	PageCount   string     `xml:"pagecount,attr"`
	RecordCount string     `xml:"recordcount,attr"`
	Videos      []xmlVideo `xml:"video"`
	Class       *xmlClass  `xml:"class"`
}

type xmlVideo struct {
	ID       string `xml:"id"`
	VodID    string `xml:"vod_id"`
	Name     string `xml:"name"`
	VodName  string `xml:"vod_name"`
	Pic      string `xml:"pic"`
	VodPic   string `xml:"vod_pic"`
	Note     string `xml:"note"`
	Remarks  string `xml:"remarks"`
	Year     string `xml:"year"`
	Des      string `xml:"des"`
	Content  string `xml:"content"`
	Actor    string `xml:"actor"`
	Area     string `xml:"area"`
	Director string `xml:"director"`
	Type     string `xml:"type"`
	Class    string `xml:"class"`
	Tag      string `xml:"tag"`
	DL       *xmlDL `xml:"dl"`
	DD       []xmlDD `xml:"dd"`
}

type xmlDL struct {
	DD []xmlDD `xml:"dd"`
}

type xmlDD struct {
	Flag string `xml:"flag,attr"`
	URL  string `xml:",chardata"`
}

// ParseLegacyXML converts apple CMS XML into Normalized.
func ParseLegacyXML(data []byte) (*Normalized, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.Strict = false
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil // assume utf-8 / already decoded
	}

	var root rssRoot
	if err := dec.Decode(&root); err != nil {
		// try wrapping
		wrapped := []byte("<rss>" + string(data) + "</rss>")
		dec2 := xml.NewDecoder(strings.NewReader(string(wrapped)))
		dec2.Strict = false
		if err2 := dec2.Decode(&root); err2 != nil {
			return nil, fmt.Errorf("parse cms xml: %w", err)
		}
	}

	out := &Normalized{}
	classSrc := root.Class
	if classSrc == nil && root.List != nil {
		classSrc = root.List.Class
	}
	if classSrc != nil {
		for _, ty := range classSrc.Ty {
			id := strings.TrimSpace(ty.ID)
			name := strings.TrimSpace(ty.Name)
			if id != "" && name != "" {
				out.Class = append(out.Class, ClassItem{TypeID: id, TypeName: name})
			}
		}
	}

	videos := root.Videos
	if root.List != nil {
		out.Page = atoi(root.List.Page)
		out.PageCount = atoi(root.List.PageCount)
		out.Total = atoi(root.List.RecordCount)
		if len(root.List.Videos) > 0 {
			videos = root.List.Videos
		}
	}
	for _, v := range videos {
		vod := xmlVideoToVod(v)
		if vod.VodID == "" || vod.VodName == "" {
			continue
		}
		out.List = append(out.List, vod)
	}
	return out, nil
}

func xmlVideoToVod(v xmlVideo) Vod {
	id := first(v.ID, v.VodID)
	name := first(v.Name, v.VodName)
	from, url := playGroupsFromXML(v)
	return Vod{
		VodID:       id,
		VodName:     name,
		VodPic:      first(v.Pic, v.VodPic),
		VodRemarks:  first(v.Note, v.Remarks),
		VodYear:     strings.TrimSpace(v.Year),
		VodContent:  first(v.Des, v.Content),
		VodActor:    strings.TrimSpace(v.Actor),
		VodArea:     strings.TrimSpace(v.Area),
		VodDirector: strings.TrimSpace(v.Director),
		TypeName:    strings.TrimSpace(v.Type),
		VodClass:    strings.TrimSpace(v.Class),
		VodTag:      strings.TrimSpace(v.Tag),
		VodPlayFrom: from,
		VodPlayURL:  url,
	}
}

func playGroupsFromXML(v xmlVideo) (string, string) {
	dds := v.DD
	if v.DL != nil && len(v.DL.DD) > 0 {
		dds = v.DL.DD
	}
	if len(dds) == 0 {
		return "", ""
	}
	var fromParts, urlParts []string
	for i, dd := range dds {
		u := strings.TrimSpace(dd.URL)
		if u == "" {
			continue
		}
		flag := strings.TrimSpace(dd.Flag)
		if flag == "" {
			flag = fmt.Sprintf("线路%d", i+1)
		}
		fromParts = append(fromParts, flag)
		urlParts = append(urlParts, u)
	}
	return strings.Join(fromParts, "$$$"), strings.Join(urlParts, "$$$")
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
