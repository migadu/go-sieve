package tests

import "testing"

// RFC 5228 §2.7: header names are always compared with "i;ascii-casemap",
// and RFC 5229 §3 replaces a variable reference before the string is used, so
// an address test on a header named by a variable finds the header whatever
// the case of the variable's value. The name was lowered before the variable
// was expanded, so a value that was not lower case was not found among the
// address headers and the header was skipped.
//
// i;ascii-casemap folds US-ASCII letters only: "letters outside ASCII are not
// treated case-insensitively" (RFC 4790 §9.2.1). strings.ToLower would map
// U+0130 LATIN CAPITAL LETTER I WITH DOT ABOVE to "i" and so name
// Mail-Reply-To with "MAİL-REPLY-TO".
func TestAddressHeaderNamedByVariable(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "variables";

test_set "message" text:
From: a@example.com
Mail-Reply-To: b@example.com
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

test "a header name is folded for US-ASCII letters only" {
	if not address :is "MAIL-REPLY-TO" "b@example.com" {
		test_fail "MAIL-REPLY-TO did not match";
	}
	if address :is "MAİL-REPLY-TO" "b@example.com" {
		test_fail "a name with U+0130 matched Mail-Reply-To";
	}
	set "dotted" "MAİL-REPLY-TO";
	if address :is "${dotted}" "b@example.com" {
		test_fail "a variable naming a header with U+0130 matched Mail-Reply-To";
	}
}
`)
}
