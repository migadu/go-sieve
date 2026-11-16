package interp

import (
	"net/textproto"
	"strings"
	"testing"
)

// A whitespace-only continuation contributes one unfolded separator too. Losing
// it changes the quoted boundary and hides the nested fields and decoded body.
func TestPartHeaderFoldedBoundaryPreservesDescendants(t *testing.T) {
	body := multipart("outer", "Content-Type: multipart/mixed; boundary=\"inner\r\n \r\n boundary\"\r\n\r\n"+
		multipart("inner  boundary", "Content-Type: text/plain\r\n\r\nneedle inside"))
	d := runOn(t, `require ["mime","body","fileinto"];
  if header :mime :anychild :param "boundary" :is "Content-Type" "inner  boundary" {fileinto "boundary";}
  if body :text :contains "needle inside" {fileinto "body";}
  if header :mime :anychild :contenttype "Content-Type" "text/plain" {fileinto "child";}`,
		textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="outer"`}}, body)
	if got, want := strings.Join(d.Mailboxes, ","), "boundary,body,child"; got != want {
		t.Fatalf("mailboxes=%q want=%q", got, want)
	}
}
