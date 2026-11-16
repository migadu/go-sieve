package tests

import (
	"context"
	"net/textproto"
	"strings"
	"testing"

	sieve "github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// An unknown charset may fall back to raw text, but must not hide an unknown
// transfer encoding: that part is skipped by non-raw body tests and counts.
func TestLeafUnknownCharsetPreservesTransferDecodeFailure(t *testing.T) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"body", "fileinto", "relational"}
	loaded, err := sieve.Load(strings.NewReader(`require ["body", "fileinto", "relational"];
if body :text :contains "needle" {fileinto "wrong";}
if body :text :count "eq" "0" {fileinto "zero";}
if body :content "text" :contains "needle" {fileinto "wrong-content";}
if body :content "text" :count "eq" "0" {fileinto "zero-content";}
if body :raw :contains "needle" {fileinto "raw";}`), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, contentType := range []string{"text/plain; charset=x-unknown", "text/plain; charset=x-unknown;;", "text/plain; charset=x-unknown; ;"} {
		for _, location := range []string{"singlepart", "multipart", "nested-message"} {
			t.Run(location+"/"+contentType, func(t *testing.T) {
				header := textproto.MIMEHeader{"Content-Type": {contentType}, "Content-Transfer-Encoding": {"x-unknown"}}
				body := "needle"
				if location != "singlepart" {
					part := "Content-Type: " + contentType + "\r\nContent-Transfer-Encoding: x-unknown\r\n\r\n" + body
					if location == "nested-message" {
						part = "Content-Type: message/rfc822\r\n\r\n" + part
					}
					header = textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=B"}}
					body = "--B\r\n" + part + "\r\n--B--\r\n"
				}
				d := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{
					Header: header, Body: []byte(body), HasBody: true,
				})
				if err := loaded.Execute(context.Background(), d); err != nil {
					t.Fatal(err)
				}
				if got, want := strings.Join(d.Mailboxes, ","), "zero,zero-content,raw"; got != want {
					t.Fatalf("mailboxes = %q, want %q", got, want)
				}
			})
		}
	}
}
