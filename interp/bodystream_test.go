package interp

import (
	"context"
	"io"
	"net/textproto"
	"strings"
	"testing"
	"testing/iotest"
)

// htmlCases are the html-to-text expectations; each is checked whole and one
// byte at a time, so every chunk boundary is exercised.
var htmlCases = []struct{ in, want string }{
	{"<p>Hello&nbsp;<b>world</b></p>", "Hello world"},
	{"<html><head><style>.x{color:red}</style></head><body>text</body></html>", "text"},
	{"<SCRIPT type=\"text/javascript\">\nvar secret = 1;\n</SCRIPT >after", "after"},
	{"a <style>unterminated", "a"},
	{"one\n\n  two\t\tthree", "one two three"},
	{"<!-- a > b --> visible", "visible"},
	{"x &amp; y &lt;z&gt; &#169; &#xe9; &unknown; &amp", "x & y <z> © é &unknown; &"},
	{"<styles>not style</styles>", "not style"},
	{"café   tab", "café tab"},
	{"<a href=\"http://x/?a=1&b=2\">link</a>", "link"},
	{"", ""},
	{"   ", ""},
	{"<br>", ""},
}

func TestHTMLTextReader(t *testing.T) {
	for _, tc := range htmlCases {
		if got := htmlToText(tc.in); got != tc.want {
			t.Errorf("whole: htmlToText(%q) = %q, want %q", tc.in, got, tc.want)
		}
		b, err := io.ReadAll(newHTMLTextReader(iotest.OneByteReader(strings.NewReader(tc.in))))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tc.want {
			t.Errorf("byte at a time: %q -> %q, want %q", tc.in, b, tc.want)
		}
	}
}

// bodyMatch runs a body test over a text/plain part served through r.
func bodyMatch(t *testing.T, script string, body string, oneByte bool) bool {
	t.Helper()
	s := loadWith(t, `require ["body", "fileinto", "comparator-i;octet", "comparator-i;ascii-casemap", "comparator-i;unicode-casemap"]; `+script)
	hdr := textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}}
	var msg Message = MessageStatic{Header: hdr, Body: []byte(body), HasBody: true}
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, msg)
	if oneByte {
		d.Msg = oneByteMessage{msg}
	}
	if err := s.Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return len(d.Mailboxes) == 1
}

// oneByteMessage is not used for the body (BodyRaw returns bytes), so chunk
// boundaries are instead forced by bodyReadChunk-sized content below.
type oneByteMessage struct{ Message }

func TestStreamContainsAcrossChunks(t *testing.T) {
	pad := strings.Repeat("x", bodyReadChunk-3)
	cases := []struct {
		name, script, body string
		want               bool
	}{
		{"needle straddles the chunk edge", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + "NEEDLE" + pad, true},
		{"needle in the last chunk", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + pad + pad + "NEEDLE", true},
		{"absent", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + pad + "NEEDLX", false},
		{"ascii-casemap across the edge", `if body :comparator "i;ascii-casemap" :contains "needle" { fileinto "m"; }`, pad + "NeEdLe" + pad, true},
		{"octet is case-sensitive", `if body :comparator "i;octet" :contains "needle" { fileinto "m"; }`, pad + "NEEDLE" + pad, false},
		{"unicode-casemap multibyte across the edge", `if body :comparator "i;unicode-casemap" :contains "straße" { fileinto "m"; }`, pad + "STRAßE" + pad, true},
		{"multibyte rune cut by the edge", `if body :comparator "i;unicode-casemap" :contains "éé" { fileinto "m"; }`, pad + "ÉÉ" + pad, true},
		{"second key matches", `if body :contains ["absent", "NEEDLE"] { fileinto "m"; }`, pad + "NEEDLE", true},
		{"empty key matches anything", `if body :contains "" { fileinto "m"; }`, "", true},
		{"is, exact", `if body :is "hello" { fileinto "m"; }`, "hello", true},
		{"is, longer value", `if body :is "hello" { fileinto "m"; }`, "hello world", false},
		{"is, unicode fold", `if body :comparator "i;unicode-casemap" :is "STRASSE" { fileinto "m"; }`, "strasse", true},
		{"is, huge value stops early", `if body :is "hello" { fileinto "m"; }`, pad + pad + pad, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bodyMatch(t, tc.script, tc.body, false); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStreamMatchesStaysBounded: :matches keeps the matcher's input bound,
// so a needle past it does not match and the rest is never read.
func TestStreamMatchesStaysBounded(t *testing.T) {
	limit := int(DefaultRegexLimits.MaxInputLength)
	if bodyMatch(t, `if body :matches "*NEEDLE*" { fileinto "m"; }`, strings.Repeat("x", limit+10)+"NEEDLE", false) {
		t.Fatal(":matches matched past the input bound")
	}
	if !bodyMatch(t, `if body :matches "*NEEDLE*" { fileinto "m"; }`, strings.Repeat("x", limit-10)+"NEEDLE", false) {
		t.Fatal(":matches did not match within the input bound")
	}
}

// countingReader counts the bytes handed out and refuses to go past a limit,
// so a test can prove how much of a part the matcher asked for.
type countingReader struct {
	src   io.Reader
	n     int
	limit int
	t     *testing.T
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	c.n += n
	if c.n > c.limit {
		c.t.Fatalf("matcher read %d bytes, more than the %d it needed", c.n, c.limit)
	}
	return n, err
}

// TestStreamContainsReadsOnlyWhatItNeeds: the matcher stops at the first
// match and never asks for the rest of the part, so a 10 MB part with its
// needle at the front costs one chunk.
func TestStreamContainsReadsOnlyWhatItNeeds(t *testing.T) {
	s := loadWith(t, `require ["body"]; if body :contains "NEEDLE" { stop; }`)
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	body := "NEEDLE" + strings.Repeat("abcdefghij", 1<<20)

	test := &TestBody{matcherTest: newMatcherTest()}
	test.match = MatchContains
	test.key = []string{"NEEDLE"}
	r := &countingReader{src: strings.NewReader(body), limit: 2 * bodyReadChunk, t: t}
	ok, err := test.matchStream(context.Background(), d, r)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	// :is reads at most 4*len(key)+1 bytes however long the part is.
	test.match = MatchIs
	r = &countingReader{src: strings.NewReader(body), limit: 4*len("NEEDLE") + 1 + bodyReadChunk, t: t}
	if ok, _ := test.matchStream(context.Background(), d, r); ok {
		t.Fatal(":is matched a longer value")
	}
}
