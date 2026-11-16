package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// Repeated content fields must select the same first value for traversal,
// charset/transfer decoding, and relational counts, including an empty value.
func TestBodyFirstFieldsDecoding(t *testing.T) {
	mixed := textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}
	for _, tc := range []struct {
		name, body, script, want string
		hdr                      textproto.MIMEHeader
	}{
		{"top charset", "caf\xe9", `if body :text :is "café" {fileinto "hit";}`, "hit",
			textproto.MIMEHeader{"Content-Type": {"text/plain; charset=iso-8859-1", "text/plain; charset=utf-8"}}},
		{"part charset", multipart("B", "Content-Type: text/plain; charset=iso-8859-1\r\nContent-Type: text/plain; charset=utf-8\r\n\r\ncaf\xe9"),
			`if body :text :is "café" {fileinto "hit";}`, "hit", mixed},
		{"part quoted printable", multipart("B", "Content-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\nContent-Transfer-Encoding: base64\r\n\r\ncaf=E9"),
			`if body :text :is "café" {fileinto "hit";}`, "hit", mixed},
		{"first empty top type defaults", "needle", `if body :text :is "needle" {fileinto "hit";}`, "hit",
			textproto.MIMEHeader{"Content-Type": {"", "application/octet-stream"}}},
		{"first empty part type defaults", multipart("B", "Content-Type:\r\nContent-Type: application/octet-stream\r\n\r\nneedle"),
			`if body :text :is "needle" {fileinto "hit";}`, "hit", mixed},
		{"first empty transfer encoding is identity", "YWJj", `if body :text :is "YWJj" {fileinto "hit";} if body :text :is "abc" {fileinto "wrong";}`, "hit",
			textproto.MIMEHeader{"Content-Type": {"text/plain"}, "Content-Transfer-Encoding": {"", "base64"}}},
		{"first unknown transfer encoding stays undecodable", "needle", `if body :text :contains "needle" {fileinto "wrong";} if body :text :count "eq" "0" {fileinto "zero";}`, "zero",
			textproto.MIMEHeader{"Content-Type": {"text/plain"}, "Content-Transfer-Encoding": {"x-unknown", "7bit"}}},
		{"nested message charset and encoding", multipart("B", "Content-Type: message/rfc822\r\nContent-Type: application/octet-stream\r\n\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Type: application/octet-stream\r\nContent-Transfer-Encoding: quoted-printable\r\nContent-Transfer-Encoding: 7bit\r\n\r\ncaf=E9"),
			`if body :text :is "café" {fileinto "hit";}`, "hit", mixed},
		{"first leaf type prevents later multipart traversal", multipart("B", "Content-Type: text/plain\r\nContent-Type: multipart/alternative; boundary=C\r\n\r\n"+multipart("C", "Content-Type: text/html\r\n\r\n<b>needle</b>")),
			`if body :content "text/html" :contains "needle" {fileinto "wrong";}`, "", mixed},
		{"first multipart boundary", multipart("B", "Content-Type: multipart/alternative; boundary=C\r\nContent-Type: multipart/alternative; boundary=D\r\n\r\n"+multipart("C", "Content-Type: text/plain\r\n\r\nneedle")),
			`if body :text :is "needle" {fileinto "hit";} if body :text :count "eq" "1" {fileinto "one";}`, "hit,one", mixed},
		{"raw remains undecoded", "YWJj", `if body :raw :is "YWJj" {fileinto "raw";} if body :text :is "abc" {fileinto "text";}`, "raw,text",
			textproto.MIMEHeader{"Content-Type": {"text/plain"}, "Content-Transfer-Encoding": {"base64", "7bit"}}},
		{"non-content headers do not affect decoding", multipart("B", strings.Repeat("X-Ignore: value\r\n", 10000)+"Content-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=E9"),
			`if body :text :is "café" {fileinto "hit";}`, "hit", mixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, `require ["body", "fileinto", "relational"]; `+tc.script, tc.hdr, tc.body)
			if got := strings.Join(d.Mailboxes, ","); got != tc.want {
				t.Fatalf("filed into %q, want %q", got, tc.want)
			}
		})
	}
}
