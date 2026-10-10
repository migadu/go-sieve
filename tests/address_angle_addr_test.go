package tests

import (
	"testing"
)

// A bare angle-addr ("<user@example.com>", no display name) is a valid
// mailbox: RFC 5322 §3.4 has name-addr = [display-name] angle-addr. Bulk
// mailers send To: in this form, so an address test that does not see
// through the brackets silently skips the rule. The invalid UTF-8 form
// Pigeonhole's test-address.svtest uses still matches only literally.
func TestAddressBareAngleAddr(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";

test_set "message" text:
From: <stephan@example.org>
To: <Nico@Example.com>
Cc: <harry@example.net>
Subject: Bare angle-addr

Test.
.
;

test "Bare angle-addr :all" {
	if not address :is "to" "nico@example.com" {
		test_fail ":all did not see through the brackets";
	}
	if not address :is ["to", "cc"] "harry@example.net" {
		test_fail ":all did not match a bare angle-addr in Cc";
	}
	if address :is "to" "<nico@example.com>" {
		test_fail ":all matched the brackets as part of the address";
	}
}

test "Bare angle-addr :localpart and :domain" {
	if not address :localpart :is "from" "stephan" {
		test_fail ":localpart did not match";
	}
	if not address :domain :is "from" "example.org" {
		test_fail ":domain did not match";
	}
}

test "Bare angle-addr :count" {
	require "relational";
	require "comparator-i;ascii-numeric";
	if not address :count "eq" :comparator "i;ascii-numeric" ["to", "cc"] "2" {
		test_fail ":count did not count bare angle-addrs";
	}
}
`)
}
