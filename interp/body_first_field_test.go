package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// TestBodyReadsFirstContentFields: a message or part that repeats its
// Content-Type or Content-Transfer-Encoding field is read by the body test
// (RFC 5173) with the first one, as the mime extension's tests, foreverypart
// and extracttext (RFC 5703) read it, so one evaluation does not give the same
// part two types. The body test used to read the last one: a part that is
// text/plain to extracttext was application/octet-stream to "body :text".
func TestBodyReadsFirstContentFields(t *testing.T) {
	mixed := textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}
	const (
		agree = `if body :text :contains "needle words" { fileinto "body"; }
			if header :mime :anychild :contenttype "Content-Type" "text/plain" { fileinto "mime"; }
			foreverypart { extracttext "x"; if string :contains "${x}" "needle words" { fileinto "extract"; } }`
		bodyText = `if body :text :contains "needle words" { fileinto "body"; }`
	)

	for _, tc := range []struct {
		name   string
		hdr    textproto.MIMEHeader
		body   string
		script string
		want   string // mailboxes filed into, comma separated
	}{
		{"part: text first, then not text", mixed,
			multipart("B", "Content-Type: text/plain\r\nContent-Type: application/octet-stream\r\n\r\nneedle words"),
			agree, "body,mime,extract"},
		{"part: not text first, then text", mixed,
			multipart("B", "Content-Type: application/octet-stream\r\nContent-Type: text/plain\r\n\r\nneedle words"),
			bodyText, ""},
		{"part: HTML first, then plain text", mixed,
			multipart("B", "Content-Type: text/html\r\nContent-Type: text/plain\r\n\r\n<b>needle</b> words"),
			`if body :text :contains "needle words" { fileinto "body"; }
			foreverypart { extracttext "x"; if string :is "${x}" "needle words" { fileinto "extract"; } }`,
			"body,extract"},
		{"part: multipart first, then plain text", mixed,
			multipart("B", "Content-Type: multipart/alternative; boundary=C\r\nContent-Type: text/plain\r\n\r\n"+
				multipart("C", "Content-Type: text/html\r\n\r\n<p>needle words</p>")),
			`if body :content "text/html" :contains "needle words" { fileinto "body"; }
			if header :mime :anychild :contenttype "Content-Type" "text/html" { fileinto "mime"; }`,
			"body,mime"},
		{"part: base64 first, then 7bit", mixed,
			multipart("B", "Content-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n"+
				"Content-Transfer-Encoding: 7bit\r\n\r\nbmVlZGxlIHdvcmRz"),
			`if body :text :contains "needle words" { fileinto "body"; }
			foreverypart { extracttext "x"; if string :is "${x}" "needle words" { fileinto "extract"; } }`,
			"body,extract"},
		{"nested message: text first, then not text", mixed,
			multipart("B", "Content-Type: message/rfc822\r\n\r\nSubject: inner\r\n"+
				"Content-Type: text/plain\r\nContent-Type: application/octet-stream\r\n\r\nneedle words"),
			bodyText, "body"},
		{"message: text first, then not text",
			textproto.MIMEHeader{"Content-Type": {"text/plain", "application/octet-stream"}},
			"needle words", bodyText, "body"},
		{"message: not text first, then text",
			textproto.MIMEHeader{"Content-Type": {"application/octet-stream", "text/plain"}},
			"needle words", bodyText, ""},
		{"message: base64 first, then 7bit",
			textproto.MIMEHeader{"Content-Type": {"text/plain"}, "Content-Transfer-Encoding": {"base64", "7bit"}},
			"bmVlZGxlIHdvcmRz", bodyText, "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, `require ["body", "mime", "foreverypart", "extracttext", "variables", "fileinto"]; `+tc.script, tc.hdr, tc.body)
			if got := strings.Join(d.Mailboxes, ","); got != tc.want {
				t.Errorf("%s\n  filed into %q, want %q", tc.script, got, tc.want)
			}
		})
	}
}
