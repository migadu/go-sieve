package interp

import (
	"context"
	"io"
	"math/rand"
	"net/textproto"
	"strings"
	"testing"
	"testing/iotest"
)

// htmlToText reduces s to its text through htmlTextReader; tests only.
func htmlToText(s string) string {
	b, _ := io.ReadAll(newHTMLTextReader(strings.NewReader(s)))
	return string(b)
}

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
	// "<" is text unless a letter, "/", "!" or "?" follows (HTML tokenizer).
	{"Price: 5 < 10 dollars", "Price: 5 < 10 dollars"},
	{"5 <3 you", "5 <3 you"},
	{"a <", "a <"},
	{"a<b", "a"}, // an unterminated tag runs to the end
	// A script element ends only at a real closing tag.
	{"a<script>var x=1;</scripts>still script</script>b", "a b"},
	{"a<script_x>not script</script_x>b", "a not script b"},
	{"a<script-x>is script</script-x>b", "a"}, // "</script-x>" is not a closing tag, so the element never ends
	{"<script>x</script/>y", "y"},
	{"x <!-- a > b visible", "x"}, // an unterminated comment runs to the end
	// Whitespace: \s and Zs runs collapse in the middle; any white space is
	// trimmed from the ends; other white space stays in the middle.
	{"\vhello\v", "hello"},
	{" hello\u0085", "hello"},
	{"a \v b", "a \v b"},
	{"a\v\vb", "a\v\vb"},
	// Octets that are not UTF-8 pass through, as for text/plain parts.
	{"caf\xe9\xe8 x", "caf\xe9\xe8 x"},
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

// bodyMatch runs a body test over a text/plain part.
func bodyMatch(t *testing.T, script string, body string) bool {
	t.Helper()
	s := loadWith(t, `require ["body", "fileinto", "comparator-i;octet", "comparator-i;ascii-casemap", "comparator-i;unicode-casemap"]; `+script)
	hdr := textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}}
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{Header: hdr, Body: []byte(body), HasBody: true})
	if err := s.Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return len(d.Mailboxes) == 1
}

func TestStreamContainsAcrossChunks(t *testing.T) {
	pad := strings.Repeat("x", bodyReadChunk-3)
	cases := []struct {
		name, script, body string
		want               bool
	}{
		{"needle straddles the chunk edge", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + "NEEDLE" + pad, true},
		{"needle in the last chunk", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + pad + pad + "NEEDLE", true},
		{"absent", `if body :contains "NEEDLE" { fileinto "m"; }`, pad + pad + "NEEDLX", false},
		{"one-byte key past the first chunk", `if body :contains "@" { fileinto "m"; }`, pad + pad + "@", true},
		{"one-byte key absent over several chunks", `if body :contains "@" { fileinto "m"; }`, pad + pad + pad, false},
		{"longest key is one byte", `if body :contains ["@", "#"] { fileinto "m"; }`, pad + pad + "#", true},
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
			if got := bodyMatch(t, tc.script, tc.body); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStreamMatchesTestString holds the streaming :contains and :is to the
// same answers as testString, the whole-string form used by every other
// test, for each comparator, with the content arriving one byte at a time.
func TestStreamMatchesTestString(t *testing.T) {
	alphabet := []rune("aAbBzZ@ éÉßİı x")
	rng := rand.New(rand.NewSource(1))
	random := func(n int) string {
		r := make([]rune, n)
		for i := range r {
			r[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(r)
	}
	s := loadWith(t, `require ["body"]; keep;`)
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	comparators := []Comparator{ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap}
	for i := 0; i < 2000; i++ {
		value := random(rng.Intn(12))
		key := random(1 + rng.Intn(4))
		if rng.Intn(4) == 0 {
			key = value // exercise :is hits
		}
		for _, cmp := range comparators {
			for _, match := range []Match{MatchContains, MatchIs} {
				test := &TestBody{matcherTest: newMatcherTest()}
				test.comparator, test.match, test.key = cmp, match, []string{key}
				want, _, _ := testString(context.Background(), cmp, match, "", value, key)
				got, err := test.matchStream(context.Background(), d, iotest.OneByteReader(strings.NewReader(value)))
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s %v value=%q key=%q: stream=%v testString=%v", cmp, match, value, key, got, want)
				}
			}
		}
	}
}

// TestStreamMatchesStaysBounded: :matches keeps the matcher's input bound,
// so a needle past it does not match and the rest is never read.
func TestStreamMatchesStaysBounded(t *testing.T) {
	limit := int(DefaultRegexLimits.MaxInputLength)
	if bodyMatch(t, `if body :matches "*NEEDLE*" { fileinto "m"; }`, strings.Repeat("x", limit+10)+"NEEDLE") {
		t.Fatal(":matches matched past the input bound")
	}
	if !bodyMatch(t, `if body :matches "*NEEDLE*" { fileinto "m"; }`, strings.Repeat("x", limit-10)+"NEEDLE") {
		t.Fatal(":matches did not match within the input bound")
	}
}

// countingReader counts the bytes handed out and fails the test past a limit,
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

// TestStreamReadsOnlyWhatItNeeds: :contains stops at the first match and
// never asks for the rest of the part, and :is reads at most 4*len(key)+1
// bytes however long the part is.
func TestStreamReadsOnlyWhatItNeeds(t *testing.T) {
	s := loadWith(t, `require ["body"]; keep;`)
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	body := "NEEDLE" + strings.Repeat("abcdefghij", 1<<20)

	test := &TestBody{matcherTest: newMatcherTest()}
	test.match, test.key = MatchContains, []string{"NEEDLE"}
	r := &countingReader{src: strings.NewReader(body), limit: bodyReadChunk, t: t}
	if ok, err := test.matchStream(context.Background(), d, r); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	test.match = MatchIs
	r = &countingReader{src: strings.NewReader(body), limit: 4*len("NEEDLE") + 1, t: t}
	if ok, _ := test.matchStream(context.Background(), d, r); ok {
		t.Fatal(":is matched a longer value")
	}
}

// failingReader serves src and then fails, like a part whose encoding is
// corrupt part-way.
type failingReader struct {
	src io.Reader
}

func (f failingReader) Read(p []byte) (int, error) {
	n, err := f.src.Read(p)
	if err == io.EOF {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

// TestStreamDecodeErrorMatchesWhatDecoded: content decoded before a failure
// is still matched, and bytes handed over together with the error are not
// dropped.
func TestStreamDecodeErrorMatchesWhatDecoded(t *testing.T) {
	s := loadWith(t, `require ["body"]; keep;`)
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	test := &TestBody{matcherTest: newMatcherTest()}
	test.match, test.key = MatchContains, []string{"NEEDLE"}
	for _, body := range []string{"before NEEDLE " + strings.Repeat("x", 2*bodyReadChunk), strings.Repeat("x", 2*bodyReadChunk) + "NEEDLE"} {
		ok, err := test.matchStream(context.Background(), d, failingReader{strings.NewReader(body)})
		if err != nil || !ok {
			t.Fatalf("needle decoded before the failure was not matched (ok=%v err=%v)", ok, err)
		}
	}
}
