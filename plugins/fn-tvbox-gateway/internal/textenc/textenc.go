package textenc

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// CharsetReader implements encoding/xml Decoder.CharsetReader.
func CharsetReader(charset string, input io.Reader) (io.Reader, error) {
	enc := lookupCharset(charset)
	if enc == nil {
		return input, nil
	}
	return transform.NewReader(input, enc.NewDecoder()), nil
}

// DecodeBytes converts non-UTF-8 bodies (common CMS GBK/GB18030) to UTF-8.
// UTF-8 input is returned unchanged. On failure, returns the original bytes.
// When an XML encoding= declaration is honored, it is rewritten to UTF-8 so a
// later xml.Decoder CharsetReader will not double-decode.
func DecodeBytes(data []byte) []byte {
	if len(data) == 0 || utf8.Valid(data) {
		return data
	}
	if encName := XMLEncodingAttr(data); encName != "" {
		if enc := lookupCharset(encName); enc != nil {
			if out, err := enc.NewDecoder().Bytes(data); err == nil && utf8.Valid(out) {
				return RewriteXMLEncodingUTF8(out)
			}
		}
	}
	for _, enc := range []encoding.Encoding{
		simplifiedchinese.GB18030,
		simplifiedchinese.GBK,
		traditionalchinese.Big5,
	} {
		out, err := enc.NewDecoder().Bytes(data)
		if err == nil && utf8.Valid(out) {
			return RewriteXMLEncodingUTF8(out)
		}
	}
	return data
}

// RewriteXMLEncodingUTF8 rewrites encoding="..." inside the XML declaration to UTF-8.
func RewriteXMLEncodingUTF8(data []byte) []byte {
	headLen := 256
	if len(data) < headLen {
		headLen = len(data)
	}
	head := data[:headLen]
	lower := bytes.ToLower(head)
	idx := bytes.Index(lower, []byte("encoding"))
	if idx < 0 {
		return data
	}
	rest := head[idx+len("encoding"):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return data
	}
	start := idx + len("encoding") + eq + 1
	for start < headLen && (data[start] == ' ' || data[start] == '\t') {
		start++
	}
	if start >= headLen {
		return data
	}
	quote := data[start]
	if quote != '"' && quote != '\'' {
		return data
	}
	endRel := bytes.IndexByte(data[start+1:headLen], quote)
	if endRel < 0 {
		return data
	}
	end := start + 1 + endRel
	var b bytes.Buffer
	b.Grow(len(data) - (end - start - 1) + 5)
	b.Write(data[:start+1])
	b.WriteString("UTF-8")
	b.Write(data[end:])
	return b.Bytes()
}

func lookupCharset(charset string) encoding.Encoding {
	name := strings.TrimSpace(strings.ToLower(charset))
	name = strings.Trim(name, `"'`)
	if name == "" || name == "utf-8" || name == "utf8" || name == "us-ascii" {
		return nil
	}
	switch name {
	case "gbk", "cp936", "windows-936", "x-gbk",
		"gb2312", "gb_2312", "gb_2312-80", "euc-cn", "euc_cn":
		// Legacy CMS often labels GB2312 but ships GBK bytes; GBK is a safe superset.
		return simplifiedchinese.GBK
	case "gb18030":
		return simplifiedchinese.GB18030
	case "big5", "big-5", "cn-big5", "csbig5", "x-x-big5":
		return traditionalchinese.Big5
	}
	if enc, err := htmlindex.Get(name); err == nil {
		return enc
	}
	return nil
}

// XMLEncodingAttr returns the encoding= value from a leading XML declaration.
func XMLEncodingAttr(data []byte) string {
	head := data
	if len(head) > 256 {
		head = head[:256]
	}
	lower := bytes.ToLower(head)
	idx := bytes.Index(lower, []byte("encoding"))
	if idx < 0 {
		return ""
	}
	rest := head[idx+len("encoding"):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return ""
	}
	rest = bytes.TrimSpace(rest[eq+1:])
	if len(rest) == 0 {
		return ""
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return ""
	}
	rest = rest[1:]
	end := bytes.IndexByte(rest, quote)
	if end < 0 {
		return ""
	}
	return string(rest[:end])
}
