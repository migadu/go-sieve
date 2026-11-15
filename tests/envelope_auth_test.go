package tests

import "testing"

// RFC 5228 §5.4: the envelope test, "Like address and header, ... returns
// true if any combination of the envelope-part list and key-list arguments
// match and returns false otherwise". "auth" is not one of the parts §5.4
// defines; it is the authenticated user, and when there is none it has no
// value to match. Pigeonhole (_auth_part_get_values in ext-envelope.c) and
// Cyrus (getenvelope in imap/lmtp_sieve.c) yield no value then. An absent
// auth user used to be tested as the empty string, so `envelope :matches
// "auth" "*"` was true for every message delivered without one. The null
// reverse-path is different: §5.4 says it "is matched against as the empty
// string".
func TestEnvelopeAuthAbsent(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "envelope";
require "relational";
require "comparator-i;ascii-numeric";

test "an absent auth user matches no key" {
	if envelope :matches "auth" "*" {
		test_fail "the absent auth user matched *";
	}
	if envelope :is "auth" "" {
		test_fail "the absent auth user matched the empty key";
	}
	if envelope :all :is "auth" "" {
		test_fail "the absent auth user matched the empty key with :all";
	}
	if envelope :localpart :matches "auth" "*" {
		test_fail "the absent auth user matched * with :localpart";
	}
}

test "an absent auth user in a list leaves the others" {
	if not envelope :is ["auth", "from"] "sender@example.com" {
		test_fail "from was not matched after the absent auth user";
	}
}

test "an absent auth user counts no value" {
	if not envelope :count "eq" :comparator "i;ascii-numeric" "auth" "0" {
		test_fail "the absent auth user did not count 0";
	}
}

test "an auth user matches" {
	test_set "envelope.auth" "user@example.com";
	if not envelope :matches "auth" "*" {
		test_fail "the auth user did not match *";
	}
	if not envelope :is "auth" "user@example.com" {
		test_fail "the auth user did not match itself";
	}
	if not envelope :count "eq" :comparator "i;ascii-numeric" "auth" "1" {
		test_fail "the auth user did not count 1";
	}
}

test "the null reverse-path still matches the empty key" {
	test_set "envelope.from" "<>";
	if not envelope :is "from" "" {
		test_fail "the null reverse-path did not match the empty key";
	}
	if not envelope :localpart :is "from" "" {
		test_fail "the null reverse-path did not match the empty key with :localpart";
	}
}
`)
}
