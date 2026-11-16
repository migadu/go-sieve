package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// TestBodyReadsContentTypeAsMimeDoes: the body test (RFC 5173) reads a part's
// Content-Type with the parser the mime extension's tests, foreverypart and
// extracttext (RFC 5703) use, so a multipart whose value ends in stray
// semicolons has the same boundary for both. mime.ParseMediaType accepts one
// trailing ";" but rejects "boundary=B;;" and "boundary=B; ;"; the body test
// used to keep no parameters then, found no boundary and never descended into
// the parts that extracttext and "header :mime :anychild" see.
func TestBodyReadsContentTypeAsMimeDoes(t *testing.T) {
	const (
		agree = `if body :text :contains "needle words" { fileinto "body"; }
			if header :mime :anychild :contenttype "Content-Type" "text/plain" { fileinto "mime"; }
			foreverypart { extracttext "x"; if string :contains "${x}" "needle words" { fileinto "extract"; } }`
		text = "Content-Type: text/plain\r\n\r\nneedle words"
	)
	top := func(ct string) textproto.MIMEHeader { return textproto.MIMEHeader{"Content-Type": {ct}} }

	for _, tc := range []struct {
		name   string
		hdr    textproto.MIMEHeader
		body   string
		script string
		want   string // mailboxes filed into, comma separated
	}{
		{"message: one trailing semicolon", top("multipart/mixed; boundary=B;"),
			multipart("B", text), agree, "body,mime,extract"},
		{"message: two trailing semicolons", top("multipart/mixed; boundary=B;;"),
			multipart("B", text), agree, "body,mime,extract"},
		{"message: semicolon, space, semicolon", top("multipart/mixed; boundary=B; ;"),
			multipart("B", text), agree, "body,mime,extract"},
		{"part: two trailing semicolons", top("multipart/mixed; boundary=B"),
			multipart("B", "Content-Type: multipart/alternative; boundary=C;;\r\n\r\n"+multipart("C", text)),
			agree, "body,mime,extract"},
		{"nested message: two trailing semicolons", top("multipart/mixed; boundary=B"),
			multipart("B", "Content-Type: message/rfc822\r\n\r\nSubject: inner\r\n"+
				"Content-Type: multipart/mixed; boundary=C;;\r\n\r\n"+multipart("C", text)),
			agree, "body,mime,extract"},
		// RFC 5173 §5.2: a matching multipart has only its prologue and
		// epilogue searched; read without its boundary, the whole body was
		// searched, parts included.
		{":content multipart: parts are not its content", top("multipart/mixed; boundary=B;;"),
			multipart("B", text),
			`if body :content "multipart" :contains "needle words" { fileinto "body"; }`, ""},
		{":content multipart: prologue is", top("multipart/mixed; boundary=B;;"),
			"needle words\r\n" + multipart("B", "Content-Type: text/plain\r\n\r\nother"),
			`if body :content "multipart" :contains "needle words" { fileinto "body"; }`, "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, `require ["body", "mime", "foreverypart", "extracttext", "variables", "fileinto"]; `+tc.script, tc.hdr, tc.body)
			if got := strings.Join(d.Mailboxes, ","); got != tc.want {
				t.Errorf("%s\n  filed into %q, want %q", tc.script, got, tc.want)
			}
		})
	}
}
