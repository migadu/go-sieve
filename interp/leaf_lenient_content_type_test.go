package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// TestLeafDecodedByLenientContentType: a text part is decoded by the
// parameters the mime extension reads. "header :mime :param" (RFC 5703 §4)
// parses Content-Type with parseMediaTypeLenient, which retries without
// trailing semicolons and white space; the body test (RFC 5173) and
// extracttext (RFC 5703 §7) used to hand go-message the raw value, which it
// parses with mime.ParseMediaType alone. That accepts one trailing ";" but
// rejects "charset=iso-8859-1;;" and "charset=iso-8859-1; ;", so the part was
// matched and extracted still in ISO-8859-1 while :param read its charset.
func TestLeafDecodedByLenientContentType(t *testing.T) {
	const (
		script = `require ["body", "mime", "foreverypart", "extracttext", "variables", "fileinto"];
			if body :text :contains "café" { fileinto "body"; }
			if header :mime :anychild :param "charset" "Content-Type" "iso-8859-1" { fileinto "param"; }
			foreverypart { extracttext "x"; if string :contains "${x}" "café" { fileinto "extract"; } }`
		latin1 = "caf\xe9 words" // "café words" in ISO-8859-1
		want   = "body,param,extract"
	)
	top := func(ct string) textproto.MIMEHeader { return textproto.MIMEHeader{"Content-Type": {ct}} }
	part := func(ct string) string { return "Content-Type: " + ct + "\r\n\r\n" + latin1 }

	for _, tc := range []struct {
		name string
		hdr  textproto.MIMEHeader
		body string
	}{
		{"message: one trailing semicolon", top("text/plain; charset=iso-8859-1;"), latin1},
		{"message: two trailing semicolons", top("text/plain; charset=iso-8859-1;;"), latin1},
		{"message: semicolon, space, semicolon", top("text/plain; charset=iso-8859-1; ;"), latin1},
		{"part: two trailing semicolons", top("multipart/mixed; boundary=B"),
			multipart("B", part("text/plain; charset=iso-8859-1;;"))},
		{"nested message: two trailing semicolons", top("multipart/mixed; boundary=B"),
			multipart("B", "Content-Type: message/rfc822\r\n\r\nSubject: inner\r\n"+part("text/plain; charset=iso-8859-1;;"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, script, tc.hdr, tc.body)
			if got := strings.Join(d.Mailboxes, ","); got != want {
				t.Errorf("filed into %q, want %q", got, want)
			}
		})
	}
}
