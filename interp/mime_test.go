package interp

import (
	"context"
	"fmt"
	"net/textproto"
	"strings"
	"testing"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

func TestHTMLToText(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<p>Hello&nbsp;<b>world</b></p>", "Hello world"},
		{"<html><head><style>.x{color:red}</style></head><body>text</body></html>", "text"},
		{"<SCRIPT type=\"text/javascript\">\nvar secret = 1;\n</SCRIPT >after", "after"},
		{"a <style>unterminated", "a"},
		{"one\n\n  two\t\tthree", "one two three"},
	} {
		if got := htmlToText(tc.in); got != tc.want {
			t.Errorf("htmlToText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseMediaTypeLenient(t *testing.T) {
	for _, tc := range []struct {
		in, mediaType string
		params        map[string]string
	}{
		{"text/calendar; method=request; charset=UTF-8;", "text/calendar", map[string]string{"method": "request", "charset": "UTF-8"}},
		{"Text/HTML", "text/html", nil},
		{"text", "text", nil},
		{"inline; filename=\"a.pdf\"", "inline", map[string]string{"filename": "a.pdf"}},
		// Duplicated parameter: strict parse fails, the type survives.
		{"text/plain; charset=a; charset=b", "text/plain", nil},
		{"", "", nil},
	} {
		mediaType, params := parseMediaTypeLenient(tc.in)
		if mediaType != tc.mediaType {
			t.Errorf("%q: media type %q, want %q", tc.in, mediaType, tc.mediaType)
		}
		for k, v := range tc.params {
			if params[k] != v {
				t.Errorf("%q: param %s = %q, want %q", tc.in, k, params[k], v)
			}
		}
	}
}

// loadWith compiles script with the extensions it needs.
func loadWith(t *testing.T, script string) *Script {
	t.Helper()
	toks, err := lexer.Lex(strings.NewReader(script), &lexer.Options{MaxTokens: 5000})
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := parser.Parse(lexer.NewStream(toks), &parser.Options{MaxBlockNesting: 15, MaxTestNesting: 15})
	if err != nil {
		t.Fatal(err)
	}
	s, err := LoadScript(cmds, &Options{MaxVariableCount: 128, MaxVariableNameLen: 32, MaxVariableLen: 4000},
		[]string{"mime", "foreverypart", "extracttext", "variables", "body", "fileinto", "relational"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func runOn(t *testing.T, script string, hdr textproto.MIMEHeader, body string) *RuntimeData {
	t.Helper()
	s := loadWith(t, script)
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{Header: hdr, Body: []byte(body), HasBody: true, Size: len(body)})
	if err := s.Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func multipart(boundary string, parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString("--" + boundary + "\r\n" + p + "\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

// TestMimeAttachmentRules runs the shapes real scripts use (Roundcube's
// attachment filters and the RFC's examples) end to end.
func TestMimeAttachmentRules(t *testing.T) {
	hdr := textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}, "Subject": {"s"}}
	body := multipart("b",
		"Content-Type: text/plain\r\n\r\nhello",
		"Content-Type: application/pdf; name=\"=?utf-8?q?Rechnung_M=C3=A4rz.pdf?=\"\r\n"+
			"Content-Disposition: attachment; filename*=utf-8''Rechnung%20M%C3%A4rz.pdf\r\n"+
			"Content-Transfer-Encoding: base64\r\n\r\nJVBERi0=")

	for _, tc := range []struct {
		script string
		want   string // mailbox filed into, "" for none
	}{
		{`if header :mime :anychild :contenttype "Content-Type" "application/pdf" { fileinto "pdf"; }`, "pdf"},
		{`if header :mime :anychild :type "Content-Type" "application" { fileinto "app"; }`, "app"},
		{`if header :mime :anychild :subtype "Content-Type" "pdf" { fileinto "pdf"; }`, "pdf"},
		{`if header :mime :contenttype "Content-Type" "application/pdf" { fileinto "top"; }`, ""}, // top level is multipart/mixed
		{`if header :mime :anychild :param "filename" :matches "Content-Disposition" "*.pdf" { fileinto "att"; }`, "att"},
		{`if header :mime :anychild :param "filename" :contains "Content-Disposition" "März" { fileinto "rfc2231"; }`, "rfc2231"},
		{`if header :mime :anychild :param "name" :contains "Content-Type" "März" { fileinto "rfc2047"; }`, "rfc2047"},
		{`if exists :mime :anychild "Content-Disposition" { fileinto "disp"; }`, "disp"},
		{`if exists :mime "Content-Disposition" { fileinto "disp"; }`, ""},
		{`if header :mime :anychild :count "ge" "Content-Type" "3" { fileinto "three"; }`, "three"},
		{`foreverypart { if header :mime :type "Content-Type" "application" { fileinto "loop"; break; } }`, "loop"},
	} {
		d := runOn(t, `require ["mime", "foreverypart", "fileinto", "relational"]; `+tc.script, hdr, body)
		got := strings.Join(d.Mailboxes, ",")
		if got != tc.want {
			t.Errorf("%s\n  filed into %q, want %q", tc.script, got, tc.want)
		}
	}
}

func TestExtractTextShapes(t *testing.T) {
	hdr := textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}
	body := multipart("b",
		"Content-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<style>p{}</style><p>caf=C3=A9</p>",
		"Content-Type: application/octet-stream\r\nContent-Transfer-Encoding: base64\r\n\r\nAAEC")
	d := runOn(t, `require ["foreverypart", "variables", "extracttext"];
		set "n" "";
		foreverypart { extracttext "t"; set "n" "${n}[${t}]"; }`, hdr, body)
	// Root (a container) yields "", HTML is reduced to its text, a binary
	// leaf is stored decoded.
	if got, want := d.Variables["n"], "[][café][\x00\x01\x02]"; got != want {
		t.Fatalf("extracted %q, want %q", got, want)
	}
}

// TestMimeTreeCaps keeps a hostile message from growing the tree without
// bound: parts past maxMIMEParts and nesting past maxMIMEDepth are kept as
// opaque leaves.
func TestMimeTreeCaps(t *testing.T) {
	parts := make([]string, maxMIMEParts+50)
	for i := range parts {
		parts[i] = fmt.Sprintf("X-N: %d\r\n\r\nx", i)
	}
	d := runOn(t, `require ["foreverypart", "variables"]; set "n" "0"; foreverypart { set "n" "${n}." ; }`,
		textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}, multipart("b", parts...))
	if got := len(d.Variables["n"]) - 1; got != maxMIMEParts {
		t.Fatalf("visited %d parts, want the cap %d", got, maxMIMEParts)
	}

	// Each level nests one multipart in the previous one.
	body := "x"
	for i := maxMIMEDepth + 10; i > 0; i-- {
		boundary := fmt.Sprintf("b%d", i)
		body = multipart(boundary, fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"b%d\"\r\n\r\n%s", i+1, body))
	}
	d = runOn(t, `require ["foreverypart", "variables"]; set "n" ""; foreverypart { set "n" "${n}." ; }`,
		textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b1"`}}, body)
	if got := len(d.Variables["n"]); got != maxMIMEDepth {
		t.Fatalf("visited %d parts, want the depth cap %d", got, maxMIMEDepth)
	}
}

// TestBodyTextSkipsStyleContent: a CSS rule is not text a reader sees, so a
// body rule must not match on it (and a bulk mailer's stylesheet must not
// make every message match a common word).
func TestBodyTextSkipsStyleContent(t *testing.T) {
	hdr := textproto.MIMEHeader{"Content-Type": {"text/html"}}
	body := "<html><head><style>.red{color:red}</style></head><body>Invoice</body></html>"
	d := runOn(t, `require ["body", "fileinto"];
		if body :contains "color:red" { fileinto "css"; }
		if body :contains "Invoice" { fileinto "text"; }`, hdr, body)
	if got := strings.Join(d.Mailboxes, ","); got != "text" {
		t.Fatalf("filed into %q, want only \"text\"", got)
	}
}

// TestBodyContainsReadsWholePart: `:contains` is not bounded by the regex
// matcher's input limit; a newsletter's footer past 256 KB still matches.
func TestBodyContainsReadsWholePart(t *testing.T) {
	limit := int(DefaultRegexLimits.MaxInputLength)
	body := "<html><body>" + strings.Repeat("<p>x</p>", limit/8+10) + "<a>Unsubscribe</a></body></html>"
	d := runOn(t, `require ["body", "fileinto"]; if body :text :contains "Unsubscribe" { fileinto "lists"; }`,
		textproto.MIMEHeader{"Content-Type": {"text/html"}}, body)
	if len(d.Mailboxes) != 1 {
		t.Fatalf("needle past %d octets of a text part did not match", limit)
	}
}

func TestSplitMultipartDelimiters(t *testing.T) {
	t.Run("boundary that prefixes a nested boundary with a hyphen", func(t *testing.T) {
		inner := multipart("x-y", "Content-Type: text/plain\r\n\r\nplain", "Content-Type: text/html\r\n\r\n<b>html</b>")
		body := multipart("x", "Content-Type: multipart/alternative; boundary=\"x-y\"\r\n\r\n"+inner)
		_, parts, _ := splitMultipart([]byte(body), "x")
		if len(parts) != 1 {
			t.Fatalf("outer boundary split the nested part's lines: %d parts", len(parts))
		}
		d := runOn(t, `require ["mime", "fileinto"]; if header :mime :anychild :contenttype "Content-Type" "text/html" { fileinto "html"; }`,
			textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="x"`}}, body)
		if len(d.Mailboxes) != 1 {
			t.Fatal("the HTML alternative was not found")
		}
	})
	t.Run("part without header fields", func(t *testing.T) {
		body := "--b\r\n\r\nplain text here\r\n--b\r\nContent-Type: text/plain\r\n\r\nsecond\r\n--b--\r\n"
		d := runOn(t, `require ["foreverypart", "variables", "extracttext"]; set "n" ""; foreverypart { extracttext "t"; set "n" "${n}[${t}]"; }`,
			textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}, body)
		if got, want := d.Variables["n"], "[][plain text here][second]"; got != want {
			t.Fatalf("extracted %q, want %q", got, want)
		}
	})
	t.Run("mixed line endings", func(t *testing.T) {
		body := "--b\nContent-Type: text/plain\n\nfirst\n--b\r\nContent-Type: text/plain\r\n\r\nsecond\r\n--b--\r\n"
		_, parts, _ := splitMultipart([]byte(body), "b")
		if len(parts) != 2 {
			t.Fatalf("got %d parts, want 2: %q", len(parts), parts)
		}
		if string(parts[0]) != "Content-Type: text/plain\n\nfirst" {
			t.Fatalf("first part %q", parts[0])
		}
	})
}

// TestMimeTreeFollowsMessage: a RuntimeData whose Message is swapped parses
// the new body rather than serving the previous message's tree.
func TestMimeTreeFollowsMessage(t *testing.T) {
	s := loadWith(t, `require ["mime", "fileinto"]; if header :mime :anychild :contenttype "Content-Type" "application/pdf" { fileinto "pdf"; }`)
	hdr := textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}
	withPDF := multipart("b", "Content-Type: application/pdf\r\n\r\n%PDF")
	withoutPDF := multipart("b", "Content-Type: text/plain\r\n\r\nhi")
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{Header: hdr, Body: []byte(withPDF), HasBody: true})
	if err := s.Execute(context.Background(), d); err != nil || len(d.Mailboxes) != 1 {
		t.Fatalf("first message: err=%v mailboxes=%v", err, d.Mailboxes)
	}
	d.Mailboxes = nil
	d.Msg = MessageStatic{Header: hdr, Body: []byte(withoutPDF), HasBody: true}
	if err := s.Execute(context.Background(), d); err != nil || len(d.Mailboxes) != 0 {
		t.Fatalf("second message served the first message's tree: err=%v mailboxes=%v", err, d.Mailboxes)
	}
}

func TestExtractTextUnknownCharsetIsEmpty(t *testing.T) {
	body := multipart("b", "Content-Type: text/plain; charset=x-nonexistent\r\n\r\nhello")
	d := runOn(t, `require ["foreverypart", "variables", "extracttext"]; set "n" ""; foreverypart { extracttext "t"; set "n" "${n}[${t}]"; }`,
		textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}, body)
	if got, want := d.Variables["n"], "[][]"; got != want {
		t.Fatalf("extracted %q, want %q (RFC 5703 §7: unknown charset yields an empty string)", got, want)
	}
}
