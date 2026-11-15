package tests

import (
	"testing"
)

// :regex under the default comparator, i;ascii-casemap, matches whatever the
// case of an ASCII letter (RFC 5228 §2.7.3: it "treats uppercase and lowercase
// characters in the US-ASCII subset of UTF-8 as the same"), and the match
// variables hold the header's own text, as in Pigeonhole, which compiles the
// pattern with REG_ICASE and matches it against the value as it is
// (mcht-regex.c). The value was lowercased instead and the pattern left as
// written, so an upper-case pattern never matched and ${1} was lowercased.
func TestRegexCaseMap(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "regex";
require "variables";
require "comparator-i;unicode-casemap";

test_set "message" text:
From: stephan@example.org
To: nico@example.com
Subject: URGENT Stuff

Test.
.
;

test "Upper-case pattern" {
	if not header :regex "subject" "^URGENT" {
		test_fail "^URGENT did not match";
	}
	if not header :regex "subject" "^Urgent stuff$" {
		test_fail "^Urgent stuff$ did not match";
	}
	if not header :regex "subject" "^urgent" {
		test_fail "^urgent did not match";
	}
	if header :comparator "i;octet" :regex "subject" "^urgent" {
		test_fail "i;octet matched ^urgent";
	}
}

test "Match variables hold the header's text" {
	if not header :regex "subject" "^(u.*) (s.*)$" {
		test_fail "^(u.*) (s.*)$ did not match";
	}
	if not string :comparator "i;octet" :is "${0}" "URGENT Stuff" {
		test_fail "match 0 is ${0}";
	}
	if not string :comparator "i;octet" :is "${1}" "URGENT" {
		test_fail "match 1 is ${1}";
	}
	if not string :comparator "i;octet" :is "${2}" "Stuff" {
		test_fail "match 2 is ${2}";
	}
}

test "Negated bracket expression" {
	if header :regex "subject" "^[^A-Z]" {
		test_fail "[^A-Z] matched u";
	}
	if not header :comparator "i;octet" :regex "subject" "^[^[:lower:]]+ " {
		test_fail "i;octet [^[:lower:]] did not match URGENT";
	}
	if header :regex "subject" "^[^[:lower:]]+ " {
		test_fail "i;ascii-casemap [^[:lower:]] matched URGENT";
	}
}

test "Only ASCII letters fold" {
	if string :regex "É" "é" {
		test_fail "é matched É";
	}
	if not string :comparator "i;unicode-casemap" :regex "Über Stuff" "^ÜBER (s.*)$" {
		test_fail "i;unicode-casemap ^ÜBER (s.*)$ did not match";
	}
	if not string :comparator "i;octet" :is "${1}" "Stuff" {
		test_fail "i;unicode-casemap match 1 is ${1}";
	}
}
`)
}
