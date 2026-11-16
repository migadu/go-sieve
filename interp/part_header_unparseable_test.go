package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// TestUnparseablePartHeader: a body part whose header does not parse (a line
// with no colon, or folding white space before the first field) is still a
// part of the multipart. The body test searches "All MIME parts with matching
// types" (RFC 5173 §5.2) and foreverypart "walks though every MIME part of a
// message" (RFC 5703 §3). The MIME tree keeps such a part as a leaf with no
// header fields holding the whole part, which extracttext reads as text/plain
// (RFC 2045 §5.2); the body test skipped it, so its text matched only through
// extracttext.
func TestUnparseablePartHeader(t *testing.T) {
	const (
		script = `require ["body", "mime", "foreverypart", "extracttext", "variables", "fileinto"];
			if body :text :contains "needle" { fileinto "body"; }
			if body :content "text" :contains "needle" { fileinto "content"; }
			foreverypart { extracttext "x"; if string :contains "${x}" "needle" { fileinto "extract"; } }`
		want = "body,content,extract"
	)
	hdr := textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}

	for _, tc := range []struct{ name, part string }{
		{"header that parses", "Content-Type: text/plain\r\n\r\nneedle words"},
		{"line without a colon", "Garbage line\r\nContent-Type: text/plain\r\n\r\nneedle words"},
		{"line without a colon after a field", "Content-Type: text/plain\r\nGarbage line\r\n\r\nneedle words"},
		{"folding white space before the first field", " folded\r\nContent-Type: text/plain\r\n\r\nneedle words"},
		{"line without a colon, bare LF", "Garbage line\nContent-Type: text/plain\n\nneedle words"},
		{"line without a colon in a nested multipart", "Content-Type: multipart/alternative; boundary=C\r\n\r\n" +
			multipart("C", "Garbage line\r\n\r\nneedle words")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, script, hdr, multipart("B", tc.part))
			if got := strings.Join(d.Mailboxes, ","); got != want {
				t.Errorf("filed into %q, want %q", got, want)
			}
		})
	}
}

// TestUnparseablePartHeaderCount: the body test counts such a part as the
// text/plain leaf foreverypart visits.
func TestUnparseablePartHeaderCount(t *testing.T) {
	const script = `require ["body", "relational", "foreverypart", "variables", "fileinto"];
		set "n" "0";
		foreverypart { set "n" "${n}."; }
		if body :content "text" :count "eq" "2" { fileinto "two"; }
		if string :is "${n}" "0..." { fileinto "three"; }`
	hdr := textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}
	body := multipart("B", "Content-Type: text/plain\r\n\r\nfirst", "Garbage line\r\n\r\nsecond")
	d := runOn(t, script, hdr, body)
	if got, want := strings.Join(d.Mailboxes, ","), "two,three"; got != want {
		t.Errorf("filed into %q, want %q", got, want)
	}
}
