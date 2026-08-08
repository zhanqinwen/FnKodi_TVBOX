package textenc

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDecodeBytesGBKXML(t *testing.T) {
	// "测试" in GBK
	name, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("测试"))
	if err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(`<?xml version="1.0" encoding="gbk"?><rss><video><name>`), name...)
	raw = append(raw, []byte(`</name></video></rss>`)...)
	if utf8.Valid(raw) {
		t.Fatal("fixture should not be valid utf-8")
	}
	out := DecodeBytes(raw)
	if !utf8.Valid(out) {
		t.Fatalf("expected utf-8, got %q", out)
	}
	if !bytes.Contains(out, []byte("测试")) {
		t.Fatalf("missing decoded text: %s", out)
	}
	if got := XMLEncodingAttr(out); got != "" && got != "UTF-8" && got != "utf-8" {
		t.Fatalf("encoding attr=%q", got)
	}
}

func TestCharsetReaderNilForUTF8(t *testing.T) {
	r, err := CharsetReader("utf-8", bytes.NewReader([]byte("a")))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	n, _ := r.Read(buf)
	if n != 1 || buf[0] != 'a' {
		t.Fatalf("%q", buf[:n])
	}
}
