package tests

import (
	"testing"
)

// A bare angle-addr ("<user@example.com>", no display name) is a valid
// mailbox: RFC 5322 §3.4 has name-addr = [display-name] angle-addr. Bulk
// mailers send To: in this form, so an address test that does not see
// through the brackets silently skips the rule. A non-ASCII local part is
// an address too, as in every other form here and as in a Dovecot build
// with mail UTF-8 support (Pigeonhole's test-address.svtest.in disables its
// "invalid UTF-8 address" expectations with #UTF8# on such a build).
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

test_set "message" text:
From: <josé@example.org>
To: <josé@example.com>
Subject: Bare angle-addr, non-ASCII local part

Test.
.
;

test "Bare angle-addr with a non-ASCII local part" {
	if not address :is "to" "josé@example.com" {
		test_fail ":all did not see through the brackets of a UTF-8 address";
	}
	if not address :localpart :is "from" "josé" {
		test_fail ":localpart did not match a UTF-8 local part";
	}
	if not address :domain :is "from" "example.org" {
		test_fail ":domain did not match";
	}
	if address :is "to" "<josé@example.com>" {
		test_fail ":all matched the brackets as part of a UTF-8 address";
	}
}
`)
}
