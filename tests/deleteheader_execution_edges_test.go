package tests

import (
	"context"
	"net/textproto"
	"reflect"
	"strings"
	"testing"

	sieve "github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

func TestDeleteHeaderRegexErrorRollsBackOnlyCurrentAction(t *testing.T) {
	loaded, err := loadWithRegexKeyExtensions(`require ["editheader", "regex", "variables", "fileinto"];
addheader "X-Prior" "saved";
deleteheader :regex "X-Foo" ["^ok$", "${bad}"];
fileinto "after";`)
	if err != nil {
		t.Fatal(err)
	}
	d := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{
		Header: textproto.MIMEHeader{"X-Foo": {"ok", "other", "ok"}},
	})
	d.Variables["bad"] = "("
	if err := loaded.Execute(context.Background(), d); err == nil || !strings.Contains(err.Error(), "regex compile error") {
		t.Fatalf("Execute error = %v, want invalid regex", err)
	}
	want := []interp.HeaderEdit{{Action: "add", FieldName: "X-Prior", Value: "saved"}}
	if !reflect.DeepEqual(d.HeaderEdits, want) || len(d.Mailboxes) != 0 {
		t.Fatalf("edits = %+v, mailboxes = %q; want only the earlier addheader", d.HeaderEdits, d.Mailboxes)
	}
}

func TestDeleteHeaderRegexUsesStatementEntryCaptures(t *testing.T) {
	RunDovecotTestInline(t, "", `
require ["vnd.dovecot.testsuite", "editheader", "variables", "regex", "relational"];
test_set "message" text:
Subject: start:target
X-Tag: prefix-first
X-Tag: keep
X-Tag: target

Body.
.
;
test "regex keys keep entry captures while successful deletes update output captures" {
 if not header :regex "Subject" "start:(target)" { test_fail "setup failed"; }
 deleteheader :regex "X-Tag" ["^prefix-(.*)$", "^(${1})$"];
 if not header :count "eq" "X-Tag" "1" { test_fail "wrong field count"; }
 if not header :is "X-Tag" "keep" { test_fail "wrong field survived"; }
 if not string :is "${1}" "target" { test_fail "last successful match capture was lost"; }
}
`)
}
