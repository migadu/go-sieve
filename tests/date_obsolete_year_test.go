package tests

import "testing"

// The date test (RFC 5260 §4) reads a two- or three-digit year of a Date:
// header by RFC 5322 §4.3: "If a two digit year is encountered whose value is
// between 00 and 49, the year is interpreted by adding 2000 ... If a two digit
// year is encountered with a value between 50 and 99, or any three digit year
// is encountered, the year is interpreted by adding 1900." Read by
// net/mail.ParseDate, a year from 50 to 68 was 20xx, and a three-digit year
// was not a date at all.
func TestDateObsoleteYear(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Date: Sat, 17 Jul 49 02:23:25 +0000
Subject: 49

Test.
.
;

test "49 is 2049" {
	if not date :originalzone "date" "date" "2049-07-17" {
		test_fail "49 is not 2049";
	}
}

test_set "message" text:
Date: Mon, 17 Jul 50 02:23:25 +0000
Subject: 50

Test.
.
;

test "50 is 1950" {
	if not date :originalzone "date" "year" "1950" {
		test_fail "50 is not 1950";
	}
	if date :originalzone "date" "year" "2050" {
		test_fail "50 is 2050";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 68 02:23:25 -0700
Subject: 68

Test.
.
;

test "68 is 1968" {
	if not date :originalzone "date" "date" "1968-07-17" {
		test_fail "68 is not 1968";
	}
}

test_set "message" text:
Date: Thu, 17 Jul 69 02:23:25 +0000
Subject: 69

Test.
.
;

test "69 is 1969" {
	if not date :originalzone "date" "year" "1969" {
		test_fail "69 is not 1969";
	}
}

test_set "message" text:
Date: Mon, 3 Jan 100 10:00:00 +0000
Subject: 100

Test.
.
;

test "100 is 2000" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "a three-digit year is not a date";
	}
	if not date :originalzone "date" "date" "2000-01-03" {
		test_fail "100 is not 2000";
	}
}

test_set "message" text:
Date: Tue, 17 Nov 126 09:00:00 EST
Subject: 126

Test.
.
;

test "126 is 2026" {
	if not date :originalzone "date" "iso8601" "2026-11-17T09:00:00-05:00" {
		test_fail "126 EST is not 2026-11-17T09:00:00-05:00";
	}
}

test_set "message" text:
Date: Sun, 17 Jul 2050 02:23:25 +0000
Subject: 2050

Test.
.
;

test "2050 is 2050" {
	if not date :originalzone "date" "year" "2050" {
		test_fail "2050 is not 2050";
	}
}
`)
}
