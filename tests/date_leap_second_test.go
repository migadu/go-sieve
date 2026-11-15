package tests

import (
	"testing"
	"time"
	_ "time/tzdata" // the zones the test runs in, on hosts without zoneinfo
)

// The date test (RFC 5260 §4) reads a Date: header whose time-of-day is a
// leap second: RFC 5322 §3.3 allows "00:00:00 through 23:59:60 (the number of
// seconds allowing for a leap second ...)". It is read as second 59 of the
// same minute. net/mail.ParseDate did not parse it, so every date test on such
// a header was false, as for a field that "does not contain a syntactically
// valid date-time specification".
func TestDateLeapSecond(t *testing.T) {
	const script = `
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Date: Sat, 31 Dec 2016 23:59:60 +0000
Subject: Leap second

Test.
.
;

test "Leap second" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "a leap second is not a date";
	}
	if not date :originalzone "date" "date" "2016-12-31" {
		test_fail "leap second not on 2016-12-31";
	}
	if not date :originalzone "date" "time" "23:59:59" {
		test_fail "leap second not read as 23:59:59";
	}
	if not date :zone "+0100" "date" "date" "2017-01-01" {
		test_fail "leap second not on 2017-01-01 in +0100";
	}
	if not date :zone "+0100" "date" "time" "00:59:59" {
		test_fail "leap second not read as 00:59:59 in +0100";
	}
}

test_set "message" text:
Date: Sun, 1 Jan 2017 00:59:60 +0100
Subject: Leap second in +0100

Test.
.
;

test "Leap second in +0100" {
	if not date :originalzone "date" "zone" "+0100" {
		test_fail "original zone is not +0100";
	}
	if not date :zone "+0000" "date" "iso8601" "2016-12-31T23:59:59+00:00" {
		test_fail "leap second in +0100 not 2016-12-31T23:59:59 UTC";
	}
}
`
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, name := range []string{"UTC", "Europe/Berlin"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = loc // no test in this package runs in parallel
		t.Run(name, func(t *testing.T) {
			RunDovecotTestInline(t, "", script)
		})
	}
}
