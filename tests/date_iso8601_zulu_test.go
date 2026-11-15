package tests

import (
	"testing"
)

// The iso8601 date-part (RFC 5260 §4.2) writes a zero offset as "Z": "a time
// zone offset of zero MUST be represented by "Z" and not "+00:00"". It was
// "+00:00", so a key ending in "Z" never matched.
func TestDateISO8601ZeroOffset(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "date";

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 +0000
Subject: Zero offset

Test.
.
;

test "Original zone +0000 is Z" {
	if not date :originalzone "date" "iso8601" "1996-07-17T02:23:25Z" {
		test_fail "iso8601 of +0000 is not 1996-07-17T02:23:25Z";
	}
	if date :originalzone :matches "date" "iso8601" "*+00:00" {
		test_fail "iso8601 of +0000 ends in +00:00";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 -0000
Subject: Unknown local offset

Test.
.
;

test "Original zone -0000 is Z" {
	if not date :originalzone "date" "iso8601" "1996-07-17T02:23:25Z" {
		test_fail "iso8601 of -0000 is not 1996-07-17T02:23:25Z";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 1996 04:23:25 +0200
Subject: Shifted to zero

Test.
.
;

test "Zone +0000 is Z" {
	if not date :zone "+0000" "date" "iso8601" "1996-07-17T02:23:25Z" {
		test_fail "iso8601 in zone +0000 is not 1996-07-17T02:23:25Z";
	}
	if not date :zone "-0000" "date" "iso8601" "1996-07-17T02:23:25Z" {
		test_fail "iso8601 in zone -0000 is not 1996-07-17T02:23:25Z";
	}
	if not date :originalzone "date" "iso8601" "1996-07-17T04:23:25+02:00" {
		test_fail "iso8601 of +0200 is not 1996-07-17T04:23:25+02:00";
	}
	if not date :zone "-0530" "date" "iso8601" "1996-07-16T20:53:25-05:30" {
		test_fail "iso8601 in zone -0530 is not 1996-07-16T20:53:25-05:30";
	}
}

test "Currentdate zone +0000 is Z" {
	if not currentdate :zone "+0000" :matches "iso8601" "????-??-??T??:??:??Z" {
		test_fail "currentdate iso8601 in zone +0000 does not end in Z";
	}
}
`)
}
