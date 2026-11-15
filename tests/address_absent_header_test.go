package tests

import "testing"

// RFC 5228 §5.1: the address test, "Like envelope and header, ... returns
// true if any combination of the header-list and key-list arguments match and
// returns false otherwise". A header the message does not have has no address
// to match, and §5.7 says of the header test: "if the named header is not
// present, it does not match any key, including the empty key". An absent
// header used to be tested as one empty address, so `address :matches "Cc"
// "*"` was true for a message without a Cc: field and a rule meant to catch
// such messages (`if not address :matches "cc" "*"`) never fired. A Cc: field
// that is present but empty still matches the empty key, as in Pigeonhole's
// test-address.svtest (":all failed to match empty address").
func TestAddressAbsentHeader(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "relational";
require "comparator-i;ascii-numeric";
require "mime";

test_set "message" text:
From: a@example.com
To: b@example.com
Resent-To:
Subject: no Cc

body
.
;

test "an absent header matches no key" {
	if address :matches "Cc" "*" {
		test_fail "the absent Cc matched *";
	}
	if address :is "Cc" "" {
		test_fail "the absent Cc matched the empty key";
	}
	if address :all :is "Cc" "" {
		test_fail "the absent Cc matched the empty key with :all";
	}
	if address :localpart :is "Cc" "" {
		test_fail "the absent Cc matched the empty key with :localpart";
	}
	if address :domain :matches "Cc" "*" {
		test_fail "the absent Cc matched * with :domain";
	}
	if address :matches ["Cc", "Bcc"] "*" {
		test_fail "a list of absent headers matched";
	}
}

test "an absent header in a list leaves the others" {
	if not address :is ["Cc", "From"] "a@example.com" {
		test_fail "From was not matched after the absent Cc";
	}
}

test "an absent header counts no address" {
	if not address :count "eq" :comparator "i;ascii-numeric" "Cc" "0" {
		test_fail "the absent Cc did not count 0";
	}
}

test "an absent header matches no key with :mime" {
	if address :mime :anychild :matches "Cc" "*" {
		test_fail "the absent Cc matched * with :mime :anychild";
	}
}

test "a present empty header still matches the empty key" {
	if not address :is :all "Resent-To" "" {
		test_fail "the empty Resent-To did not match the empty key";
	}
}
`)
}
