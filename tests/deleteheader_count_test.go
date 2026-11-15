package tests

import (
	"testing"
)

// RFC 5293 §5: the value-patterns of deleteheader match "according to the
// match-type and comparator and performed as if by the "header" test", and
// RFC 5231 §4.2: "The Header Test counts the total number of instances of the
// specified fields." deleteheader handed :count to the string matcher, which
// panics on it, so the script failed at every message.
func TestDeleteHeaderCount(t *testing.T) {
	const head = `
require "vnd.dovecot.testsuite";
require "editheader";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
X-Foo: first
X-Foo: second
Subject: two X-Foo fields

Test.
.
;
`
	cases := []struct {
		name, test string
	}{
		{"count that does not hold deletes nothing", `
deleteheader :count "eq" :comparator "i;ascii-numeric" "x-foo" "1";
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "2" {
	test_fail "deleteheader deleted an X-Foo field although their count is 2";
}`},
		{"count that holds deletes every field", `
deleteheader :count "eq" :comparator "i;ascii-numeric" "x-foo" ["5", "2"];
if exists "x-foo" {
	test_fail "deleteheader kept an X-Foo field although their count is 2";
}`},
		{"count with index is 1", `
deleteheader :index 2 :count "eq" :comparator "i;ascii-numeric" "x-foo" "1";
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "1" {
	test_fail "deleteheader :index 2 did not delete one X-Foo field";
}
if not header :is "x-foo" "first" {
	test_fail "deleteheader :index 2 deleted another X-Foo field";
}`},
		{"count with index out of range deletes nothing", `
deleteheader :index 3 :count "eq" :comparator "i;ascii-numeric" "x-foo" "1";
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "2" {
	test_fail "deleteheader :index 3 deleted an X-Foo field";
}`},
		{"count is of the fields after earlier edits", `
addheader "X-Foo" "added";
deleteheader :count "eq" :comparator "i;ascii-numeric" "x-foo" "2";
if not header :count "eq" :comparator "i;ascii-numeric" "x-foo" "3" {
	test_fail "deleteheader did not count the added X-Foo field";
}
deleteheader :count "eq" :comparator "i;ascii-numeric" "x-foo" "3";
if exists "x-foo" {
	test_fail "deleteheader kept an X-Foo field although their count is 3";
}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RunDovecotTestInline(t, "", head+"test \"deleteheader\" {"+tc.test+"\n}\n")
		})
	}
}
