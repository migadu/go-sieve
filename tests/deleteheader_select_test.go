package tests

import (
	"strings"
	"testing"
)

// RFC 5293 §5: "The value-patterns, if specified, restrict which occurrences
// of the header field are deleted to those whose values match any of the
// specified value-patterns". A deletion of a field whose value matched was
// recorded with that value, and an empty value read as "no value-patterns",
// so matching an empty field deleted every field of the name.
func TestDeleteHeaderEmptyValueMatch(t *testing.T) {
	const head = `
require "vnd.dovecot.testsuite";
require "editheader";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
X-Foo:
X-Foo: keep
X-Foo:
Subject: three X-Foo fields

Test.
.
;
`
	cases := []struct {
		name, test string
	}{
		{"empty value-pattern deletes only the empty fields", `
deleteheader :is "x-foo" "";
if not header :is "x-foo" "keep" {
	test_fail "deleteheader deleted the X-Foo field that does not match";
}
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "1" {
	test_fail "deleteheader kept an empty X-Foo field";
}`},
		{"empty value among the value-patterns", `
deleteheader :is "x-foo" ["nothing", ""];
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "1" {
	test_fail "deleteheader did not delete just the two empty X-Foo fields";
}
if not header :is "x-foo" "keep" {
	test_fail "deleteheader deleted the X-Foo field that does not match";
}`},
		{"later edits see the fields left", `
deleteheader :is "x-foo" "";
addheader :last "X-Foo" "added";
deleteheader :index 1 "x-foo";
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "1" {
	test_fail "the X-Foo fields left are not the ones the edits selected";
}
if not header :is "x-foo" "added" {
	test_fail "deleteheader :index 1 did not delete the X-Foo field left over";
}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RunDovecotTestInline(t, "", head+"test \"deleteheader\" {"+tc.test+"\n}\n")
		})
	}
}

// RFC 5293 §5: ":index <fieldno> [...] beginning at 1, the first named header
// field." There is no field 0, and :index 0 read as no :index at all, so the
// script deleted every field of the name. The index extension refuses
// ":index 0" for the same reason (RFC 5260 §6).
func TestDeleteHeaderIndexZero(t *testing.T) {
	for _, script := range []string{
		`require "editheader"; deleteheader :index 0 "X-Foo";`,
		`require "editheader"; deleteheader :index 0 :last "X-Foo";`,
		`require "editheader"; deleteheader :index 0 :is "X-Foo" "keep";`,
	} {
		err := loadWithArgumentRulesExtensions(script)
		if err == nil {
			t.Errorf("%s: loaded, want an error", script)
			continue
		}
		if !strings.Contains(err.Error(), ":index must be 1 or more") {
			t.Errorf("%s: error %q, want it to say that :index must be 1 or more", script, err)
		}
	}
	if err := loadWithArgumentRulesExtensions(`require "editheader"; deleteheader :index 1 :last "X-Foo";`); err != nil {
		t.Errorf(":index 1: %v", err)
	}
}
