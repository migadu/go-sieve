package tests

import "testing"

// TestEditheaderFieldNameASCIICasemap covers which header tests see a field
// that addheader added: those naming the field under i;ascii-casemap. RFC 5228
// §2.7: "when a string represents the name of a header, the comparator is
// never user-specified. Header comparisons are always done with the
// "i;ascii-casemap" operator". RFC 4790 §9.2.1: i;ascii-casemap "treats
// US-ASCII letters case-insensitively" and "letters outside ASCII are not
// treated case-insensitively".
//
// Unicode case folding would also equate U+212A KELVIN SIGN with "k" and
// U+017F LATIN SMALL LETTER LONG S with "s", so "Key" would name an added
// Key field and "X-Teſt" an added X-Test field.
func TestEditheaderFieldNameASCIICasemap(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "editheader";
require "variables";
require "encoded-character";

test_set "message" text:
From: a@example.com
Subject: hello

body
.
;

test "an added field is found under any US-ASCII case" {
	addheader "Key" "x";
	addheader :last "X-Test" "y";
	if not header :is "KEY" "x" {
		test_fail "KEY did not find the added Key";
	}
	if not header :is "${unicode:4B}ey" "x" {
		test_fail "an encoded K did not find the added Key";
	}
	if not exists "x-TEST" {
		test_fail "x-TEST did not find the added X-Test";
	}
}

test "an added field is not found through Unicode case folding" {
	addheader "Key" "x";
	addheader :last "X-Test" "y";
	if header :is "${unicode:212A}ey" "x" {
		test_fail "a name with U+212A found the added Key";
	}
	if exists "${unicode:212A}ey" {
		test_fail "exists with U+212A found the added Key";
	}
	if header :is "X-Te${unicode:17F}t" "y" {
		test_fail "a name with U+017F found the added X-Test";
	}
	set "kelvin" "${unicode:212A}EY";
	if header :is "${kelvin}" "x" {
		test_fail "a variable naming a header with U+212A found the added Key";
	}
}
`)
}
