package tests

import "testing"

// RFC 5228 §2.7: header names are always compared with "i;ascii-casemap",
// and RFC 5229 §3 replaces a variable reference before the string is used, so
// an address test on a header named by a variable finds the header whatever
// the case of the variable's value. The name was lowered before the variable
// was expanded, so a value that was not lower case was not found among the
// address headers and the header was skipped.
func TestAddressHeaderNamedByVariable(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "variables";

test_set "message" text:
From: a@example.com
Subject: hello

body
.
;

test "a header name from a variable is matched case-insensitively" {
	set "lower" "from";
	set "title" "From";
	set "upper" "FROM";
	if not address :is "${lower}" "a@example.com" {
		test_fail "${lower} did not match";
	}
	if not address :is "${title}" "a@example.com" {
		test_fail "${title} did not match";
	}
	if not address :is "${upper}" "a@example.com" {
		test_fail "${upper} did not match";
	}
	if not address :is ["Sender", "${upper}"] "a@example.com" {
		test_fail "a list naming ${upper} did not match";
	}
	if address :is "${upper}" "nobody@example.com" {
		test_fail "${upper} matched another address";
	}
}

test "a variable naming a header that is not an address header is skipped" {
	set "subject" "Subject";
	if address :is "${subject}" "" {
		test_fail "Subject matched the empty string";
	}
	if address :matches "${subject}" "*" {
		test_fail "Subject matched";
	}
}
`)
}
