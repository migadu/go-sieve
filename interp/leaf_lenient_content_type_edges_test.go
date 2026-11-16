package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

func TestLeafLenientContentTypeDecoderCombinations(t *testing.T) {
	const script = `require ["body", "mime", "foreverypart", "extracttext", "variables", "fileinto"];
if body :text :is "café" {fileinto "body";}
foreverypart {extracttext "x"; if string :is "${x}" "café" {fileinto "extract";}}`
	for _, tc := range []struct{ name, contentType, cte, body string }{
		{"HTML and quoted-printable", `text/html; charset=iso-8859-1; ;`, "quoted-printable", "<p>caf=E9</p>"},
		{"base64", `text/plain; charset=iso-8859-1;;`, "base64", "Y2Fm6Q=="},
		{"RFC2231 charset", `text/plain; charset*=us-ascii''iso-8859-1;;`, "8bit", "caf\xe9"},
		{"continued charset", `text/plain; charset*0=iso-; charset*1=8859-1;;`, "8bit", "caf\xe9"},
		{"encoded-word charset", `text/plain; charset="=?us-ascii?q?iso-8859-1?=";;`, "8bit", "caf\xe9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			part := "Content-Type: " + tc.contentType + "\r\nContent-Type: application/octet-stream\r\nContent-Transfer-Encoding: " + tc.cte + "\r\nContent-Transfer-Encoding: x-unknown\r\n\r\n" + tc.body
			d := runOn(t, script, textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}, multipart("B", part))
			if got, want := strings.Join(d.Mailboxes, ","), "body,extract"; got != want {
				t.Fatalf("mailboxes = %q, want %q", got, want)
			}
		})
	}
}

func TestLeafLenientContentTypeRetainsDecodeFailurePolicies(t *testing.T) {
	for _, tc := range []struct{ name, contentType, cte, body, script, want string }{
		{"unknown charset", `text/plain; charset=x-unknown;;`, "7bit", "needle",
			`if body :text :is "needle" {fileinto "raw";} foreverypart {extracttext "x"; if string :is "${x}" "" {fileinto "empty";}}`, "raw,empty"},
		{"unknown charset preserves base64 decoding", `text/plain; charset=x-unknown;;`, "BASE64", "bmVlZGxl",
			`if body :text :is "needle" {fileinto "decoded";} if body :text :count "eq" "1" {fileinto "one";} foreverypart {extracttext "x"; if string :is "${x}" "" {fileinto "empty";}}`, "decoded,one,empty"},
		{"unrecoverable parameters", `text/plain; charset=iso-8859-1; charset=utf-8;;`, "7bit", "caf\xe9",
			`if body :text :is "café" {fileinto "wrong";} if body :text :contains "caf" {fileinto "raw";} foreverypart {extracttext "x"; if string :contains "${x}" "caf" {fileinto "extract";}}`, "raw,extract"},
		{"unknown transfer encoding", `text/plain; charset=iso-8859-1;;`, "x-unknown", "needle",
			`if body :text :contains "needle" {fileinto "wrong";} if body :text :count "eq" "0" {fileinto "zero";} foreverypart {extracttext "x"; if string :is "${x}" "" {fileinto "empty";}}`, "zero,empty"},
		{"unknown charset cannot hide transfer decoding failure", `text/plain; charset=x-unknown;;`, "x-unknown", "needle",
			`if body :text :contains "needle" {fileinto "wrong";} if body :text :count "eq" "0" {fileinto "zero";} foreverypart {extracttext "x"; if string :is "${x}" "" {fileinto "empty";}}`, "zero,empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, `require ["body", "foreverypart", "extracttext", "variables", "fileinto", "relational"];`+tc.script,
				textproto.MIMEHeader{"Content-Type": {tc.contentType}, "Content-Transfer-Encoding": {tc.cte}}, tc.body)
			if got := strings.Join(d.Mailboxes, ","); got != tc.want {
				t.Fatalf("mailboxes = %q, want %q", got, tc.want)
			}
		})
	}
}
